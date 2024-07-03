package server

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/resgateio/resgate/server/codec"
	"github.com/resgateio/resgate/server/reserr"
)

func (s *Service) initAPIHandler() error {
	f := apiEncoderFactories[strings.ToLower(s.cfg.APIEncoding)]
	if f == nil {
		keys := make([]string, 0, len(apiEncoderFactories))
		for k := range apiEncoderFactories {
			keys = append(keys, k)
		}
		return fmt.Errorf("invalid apiEncoding setting (%s) - available encodings: %s", s.cfg.APIEncoding, strings.Join(keys, ", "))
	}
	s.enc = f(s.cfg)
	mimetype, _, err := mime.ParseMediaType(s.enc.ContentType())
	s.mimetype = mimetype
	return err
}

// setCommonHeaders sets common headers such as Access-Control-*.
// It returns error if the origin header does not match any allowed origin.
func (s *Service) setCommonHeaders(w http.ResponseWriter, r *http.Request) error {
	if s.cfg.HeaderAuth != nil {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}
	if s.cfg.allowOrigin[0] == "*" {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		return nil
	}

	// CORS validation
	origin := r.Header["Origin"]
	// If no Origin header is set, or the value is null, we can allow access
	// as it is not coming from a CORS enabled browser.
	if len(origin) > 0 && origin[0] != "null" {
		if matchesOrigins(s.cfg.allowOrigin, origin[0]) {
			w.Header().Set("Access-Control-Allow-Origin", origin[0])
			w.Header().Set("Vary", "Origin")
		} else {
			// No matching origin
			w.Header().Set("Access-Control-Allow-Origin", s.cfg.allowOrigin[0])
			w.Header().Set("Vary", "Origin")
			return reserr.ErrForbiddenOrigin
		}
	}
	return nil
}

func (s *Service) apiHandler(w http.ResponseWriter, r *http.Request) {
	err := s.setCommonHeaders(w, r)
	if r.Method == "OPTIONS" {
		w.Header().Set("Access-Control-Allow-Methods", s.cfg.allowMethods)
		reqHeaders := r.Header["Access-Control-Request-Headers"]
		if len(reqHeaders) > 0 {
			w.Header().Set("Access-Control-Allow-Headers", strings.Join(reqHeaders, ", "))
		}
		return
	}
	if err != nil {
		httpError(w, err, s.enc)
		return
	}

	path := r.URL.RawPath
	if path == "" {
		path = r.URL.Path
	}

	apiPath := s.cfg.APIPath

	// NotFound on paths with trailing slash (unless it is only the APIPath)
	if len(path) > len(apiPath) && path[len(path)-1] == '/' {
		notFoundHandler(w, s.enc)
		return
	}

	var rid, action string
	switch r.Method {
	case "HEAD":
		fallthrough
	case "GET":
		// Metrics
		if s.metrics != nil {
			s.metrics.HTTPRequestsGet.Add(1)
		}

		rid = PathToRID(path, r.URL.RawQuery, apiPath)
		if !codec.IsValidRID(rid, true) {
			notFoundHandler(w, s.enc)
			return
		}

		s.temporaryConn(w, r, func(c *wsConn, cb func([]byte, string, error, *codec.Meta)) {
			c.GetHTTPSubscription(rid, func(sub *Subscription, meta *codec.Meta, err error) {
				var b []byte
				if err == nil && !meta.IsDirectResponseStatus() {
					b, err = s.enc.EncodeGET(sub)
				}
				cb(b, "", err, meta)
			})
		})
		return

	case "POST":
		// Metrics
		if s.metrics != nil {
			s.metrics.HTTPRequestsPost.Add(1)
		}

		rid, action = PathToRIDAction(path, r.URL.RawQuery, apiPath)
	default:
		var m *string
		switch r.Method {
		case "PUT":
			if s.cfg.PUTMethod != nil {
				m = s.cfg.PUTMethod
			}
		case "DELETE":
			if s.cfg.DELETEMethod != nil {
				m = s.cfg.DELETEMethod
			}
		case "PATCH":
			if s.cfg.PATCHMethod != nil {
				m = s.cfg.PATCHMethod
			}
		}
		// Return error if we have no mapping for the method
		if m == nil {
			httpError(w, reserr.ErrMethodNotAllowed, s.enc)
			return
		}

		// Metrics
		if s.metrics != nil {
			s.metrics.HTTPRequests.With(r.Method).Add(1)
		}

		rid = PathToRID(path, r.URL.RawQuery, apiPath)
		action = *m
	}

	s.handleCall(w, r, rid, action)
}

func notFoundHandler(w http.ResponseWriter, enc APIEncoder) {
	w.Header().Set("Content-Type", enc.ContentType())
	w.WriteHeader(http.StatusNotFound)
	w.Write(enc.NotFoundError())
}

func (s *Service) handleCall(w http.ResponseWriter, r *http.Request, rid string, action string) {
	if !codec.IsValidRID(rid, true) || !codec.IsValidRIDPart(action) {
		notFoundHandler(w, s.enc)
		return
	}

	// Try to parse the body
	b, err := io.ReadAll(r.Body)
	if err != nil {
		httpError(w, &reserr.Error{Code: reserr.CodeBadRequest, Message: "Error reading request body: " + err.Error()}, s.enc)
		return
	}

	var params json.RawMessage
	if strings.TrimSpace(string(b)) != "" {
		err = json.Unmarshal(b, &params)
		if err != nil {
			httpError(w, &reserr.Error{Code: reserr.CodeBadRequest, Message: "Error decoding request body: " + err.Error()}, s.enc)
			return
		}
	}

	s.temporaryConn(w, r, func(c *wsConn, cb func([]byte, string, error, *codec.Meta)) {
		c.CallHTTPResource(rid, action, params, func(r json.RawMessage, refRID string, err error, meta *codec.Meta) {
			var b []byte
			if err == nil && refRID == "" && !meta.IsDirectResponseStatus() {
				b, err = s.enc.EncodePOST(r)
			}
			cb(b, RIDToPath(refRID, s.cfg.APIPath), err, meta)
		})
	})
}

// temporaryConn creates a temporary connection which is provides through a
// callback. Once the callback calls its write callback, the temporaryConn will
// return.
// The write callback takes 4 arguments, with priority in order: meta, err, href, out
// * out  - If not nil, it will be the output with a status OK response
// * href - If not empty, it will be used as Location for a Found response
// * err  - If not empty, it will be encoded into an error for an error response based on the error code
// * meta - If not empty, may change the behavior of all the others.
func (s *Service) temporaryConn(w http.ResponseWriter, r *http.Request, cb func(*wsConn, func(out []byte, href string, err error, meta *codec.Meta))) {
	c := s.newWSConn(r, versionLatest)
	if c == nil {
		httpError(w, reserr.ErrServiceUnavailable, s.enc)
		return
	}

	done := make(chan struct{})
	var authMeta *codec.Meta
	rs := func(out []byte, href string, err error, meta *codec.Meta) {
		defer c.dispose()
		defer close(done)

		// Merge auth meta into the callbacks meta
		meta = authMeta.Merge(meta)

		// Validate the status of the meta object.
		if !meta.IsValidStatus() {
			s.Errorf("Invalid meta status: %d", *meta.Status)
			meta.Status = nil
		}

		// Handle meta override
		if meta.IsDirectResponseStatus() {
			httpStatusResponse(w, s.enc, *meta.Status, meta.Header, href, err)
			return
		}

		// Merge any meta headers into the response header.
		codec.MergeHeader(w.Header(), meta.GetHeader())

		// Handle error
		if err != nil {
			// Convert system.methodNotFound to system.methodNotAllowed for PUT/DELETE/PATCH
			if rerr, ok := err.(*reserr.Error); ok {
				if rerr.Code == reserr.CodeMethodNotFound && (r.Method == "PUT" || r.Method == "DELETE" || r.Method == "PATCH") {
					err = reserr.ErrMethodNotAllowed
				}
			}
			httpError(w, err, s.enc)
			return
		}

		// Handle href
		if href != "" {
			w.Header().Set("Location", href)
			w.WriteHeader(http.StatusOK)
			return
		}

		// Output content
		if len(out) > 0 {
			w.Header().Set("Content-Type", s.enc.ContentType())
			w.Write(out)
			return
		}

		// No content
		w.WriteHeader(http.StatusNoContent)
	}
	c.Enqueue(func() {
		if s.cfg.HeaderAuth != nil {
			c.AuthResourceNoResult(s.cfg.headerAuthRID, s.cfg.headerAuthAction, nil, func(refRID string, err error, m *codec.Meta) {
				if m.IsDirectResponseStatus() {
					httpStatusResponse(w, s.enc, *m.Status, m.Header, RIDToPath(refRID, s.cfg.APIPath), err)
					c.dispose()
					close(done)
					return
				}

				authMeta = m
				cb(c, rs)
			})
		} else {
			cb(c, rs)
		}
	})
	<-done
}

func httpStatusResponse(w http.ResponseWriter, enc APIEncoder, status int, header http.Header, href string, err error) {
	// Redirect
	if status >= 300 && status < 400 {
		if href != "" {
			if _, ok := header["Location"]; !ok {
				w.Header().Set("Location", href)
			}
		}
		codec.MergeHeader(w.Header(), header)
		w.WriteHeader(status)
		return
	}

	// 4xx and 5xx errors
	var rerr *reserr.Error
	if err == nil {
		rerr = statusError(status)
	} else {
		rerr = reserr.RESError(err)
	}

	codec.MergeHeader(w.Header(), header)
	w.Header().Set("Content-Type", enc.ContentType())
	w.WriteHeader(status)
	w.Write(enc.EncodeError(rerr))
}

func errorStatus(err error) (*reserr.Error, int) {
	rerr := reserr.RESError(err)

	var code int
	switch rerr.Code {
	case reserr.CodeNotFound:
		fallthrough
	case reserr.CodeMethodNotFound:
		fallthrough
	case reserr.CodeTimeout:
		code = http.StatusNotFound
	case reserr.CodeAccessDenied:
		code = http.StatusUnauthorized
	case reserr.CodeMethodNotAllowed:
		code = http.StatusMethodNotAllowed
	case reserr.CodeInternalError:
		code = http.StatusInternalServerError
	case reserr.CodeServiceUnavailable:
		code = http.StatusServiceUnavailable
	case reserr.CodeForbidden:
		code = http.StatusForbidden
	case reserr.CodeSubjectTooLong:
		code = http.StatusRequestURITooLong
	default:
		code = http.StatusBadRequest
	}

	return rerr, code
}

func httpError(w http.ResponseWriter, err error, enc APIEncoder) {
	rerr, code := errorStatus(err)
	w.Header().Set("Content-Type", enc.ContentType())
	w.WriteHeader(code)
	w.Write(enc.EncodeError(rerr))
}

// statusError returns a res error based on a HTTP status.
func statusError(status int) *reserr.Error {
	if status >= 400 {
		if status < 500 {
			switch status {
			// Access denied
			case http.StatusUnauthorized:
				fallthrough
			case http.StatusPaymentRequired:
				fallthrough
			case http.StatusProxyAuthRequired:
				return reserr.ErrAccessDenied

			// Forbidden
			case http.StatusForbidden:
				fallthrough
			case http.StatusUnavailableForLegalReasons:
				return reserr.ErrForbidden

			// Not found
			case http.StatusGone:
				fallthrough
			case http.StatusNotFound:
				return reserr.ErrNotFound

			// Method not allowed
			case http.StatusMethodNotAllowed:
				return reserr.ErrMethodNotAllowed

			// Timeout
			case http.StatusRequestTimeout:
				return reserr.ErrTimeout

			// Bad request (default)
			default:
				return reserr.ErrBadRequest
			}
		}
		if status < 600 {
			switch status {
			// Not implemented
			case http.StatusNotImplemented:
				return reserr.ErrNotImplemented

			// Service unavailable
			case http.StatusServiceUnavailable:
				return reserr.ErrServiceUnavailable

			// Timeout
			case http.StatusGatewayTimeout:
				return reserr.ErrTimeout

			// Internal error (default)
			default:
				return reserr.ErrInternalError
			}
		}
	}

	return reserr.ErrInternalError
}
