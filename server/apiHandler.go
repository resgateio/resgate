package server

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
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
	return nil
}

func (s *Service) apiHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.RawPath
	if path == "" {
		path = r.URL.Path
	}

	apiPath := s.cfg.APIPath

	switch r.Method {
	case "GET":
		// Redirect paths with trailing slash (unless it is only the APIPath)
		if len(path) > len(apiPath) && path[len(path)-1] == '/' {
			notFoundHandler(w, r, s.enc)
			return
		}

		rid := PathToRID(path, r.URL.RawQuery, apiPath)
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

	case "POST":
		// Redirect paths with trailing slash (unless it is only the APIPath)
		if len(path) > len(apiPath) && path[len(path)-1] == '/' {
			notFoundHandler(w, r, s.enc)
			return
		}

		rid, action := PathToRIDAction(path, r.URL.RawQuery, apiPath)
		if !codec.IsValidRID(rid, true) || !codec.IsValidRID(action, false) {
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
					w.WriteHeader(http.StatusCreated)
					cb(nil, nil)
				} else {
					cb(s.enc.EncodePOST(r))
				}
			})
		})

	default:
		httpError(w, reserr.ErrMethodNotAllowed, s.enc)
	}
}

func notFoundHandler(w http.ResponseWriter, r *http.Request, enc APIEncoder) {
	w.Header().Set("Content-Type", enc.ContentType())
	w.WriteHeader(http.StatusNotFound)
	w.Write(enc.NotFoundError())
}

func (s *Service) temporaryConn(w http.ResponseWriter, r *http.Request, cb func(*wsConn, func([]byte, error))) {
	c := s.newWSConn(nil, r)
	if c == nil {
		httpError(w, reserr.ErrServiceUnavailable, s.enc)
		return
	}

	done := make(chan struct{})
	rs := func(out []byte, err error) {
		defer c.dispose()
		defer close(done)

		if err != nil {
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
	out := enc.EncodeError(rerr)

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
	default:
		code = http.StatusBadRequest
	}

	w.Header().Set("Content-Type", enc.ContentType())
	w.WriteHeader(code)
	w.Write(out)
}
