package server

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
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

	// NotFound on oaths with trailing slash (unless it is only the APIPath)
	if len(path) > len(apiPath) && path[len(path)-1] == '/' {
		notFoundHandler(w, r, s.enc)
		return
	}

	var rid, action string
	switch r.Method {
	case "HEAD":
		fallthrough
	case "GET":
		rid = PathToRID(path, r.URL.RawQuery, apiPath)
		if !codec.IsValidRID(rid, true) {
			notFoundHandler(w, r, s.enc)
			return
		}

		s.temporaryConn(w, r, func(c *wsConn, cb func([]byte, error)) {
			c.GetSubscription(rid, func(sub *Subscription, err error) {
				if err != nil {
					cb(nil, err)
					return
				}
				cb(s.enc.EncodeGET(sub))
			})
		})
		return

	case "POST":
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
		rid = PathToRID(path, r.URL.RawQuery, apiPath)
		action = *m
	}

	s.handleCall(w, r, rid, action)
}

func notFoundHandler(w http.ResponseWriter, r *http.Request, enc APIEncoder) {
	w.Header().Set("Content-Type", enc.ContentType())
	w.WriteHeader(http.StatusNotFound)
	w.Write(enc.NotFoundError())
}

func (s *Service) handleCall(w http.ResponseWriter, r *http.Request, rid string, action string) {
	if !codec.IsValidRID(rid, true) || !codec.IsValidRIDPart(action) {
		notFoundHandler(w, r, s.enc)
		return
	}

	// Try to parse the body
	b, err := ioutil.ReadAll(r.Body)
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

	s.temporaryConn(w, r, func(c *wsConn, cb func([]byte, error)) {
		c.CallHTTPResource(rid, s.cfg.APIPath, action, params, func(r json.RawMessage, href string, err error) {
			if err != nil {
				cb(nil, err)
			} else if href != "" {
				w.Header().Set("Location", href)
				w.WriteHeader(http.StatusOK)
				cb(nil, nil)
			} else {
				cb(s.enc.EncodePOST(r))
			}
		})
	})
}

func (s *Service) temporaryConn(w http.ResponseWriter, r *http.Request, cb func(*wsConn, func([]byte, error))) {
	c := s.newWSConn(nil, r, versionLatest)
	if c == nil {
		httpError(w, reserr.ErrServiceUnavailable, s.enc)
		return
	}

	done := make(chan struct{})
	rs := func(out []byte, err error) {
		defer c.dispose()
		defer close(done)

		if err != nil {
			// Convert system.methodNotFound to system.methodNotAllowed for PUT/DELETE/PATCH
			if rerr, ok := err.(*reserr.Error); ok {
				if rerr.Code == reserr.CodeMethodNotFound && (r.Method == "PUT" || r.Method == "DELETE" || r.Method == "PATCH") {
					httpError(w, reserr.ErrMethodNotAllowed, s.enc)
					return
				}
			}
			httpError(w, err, s.enc)
			return
		}

		if len(out) > 0 {
			w.Header().Set("Content-Type", s.enc.ContentType())
			w.Write(out)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
	c.Enqueue(func() {
		if s.cfg.HeaderAuth != nil {
			c.AuthResource(s.cfg.headerAuthRID, s.cfg.headerAuthAction, nil, func(_ interface{}, err error) {
				cb(c, rs)
			})
		} else {
			cb(c, rs)
		}
	})
	<-done
}

func httpError(w http.ResponseWriter, err error, enc APIEncoder) {
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

	w.Header().Set("Content-Type", enc.ContentType())
	w.WriteHeader(code)
	w.Write(enc.EncodeError(rerr))
}
