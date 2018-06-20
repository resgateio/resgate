package service

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/jirenius/resgate/httpApi"
	"github.com/jirenius/resgate/reserr"
)

func (s *Service) initHTTPListener() {
	s.conns = make(map[string]*wsConn)
	s.mux.HandleFunc(s.cfg.APIPath, s.httpHandler)
}

func (s *Service) httpHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.RawPath
	if path == "" {
		path = r.URL.Path
	}

	switch r.Method {
	case "GET":

		rid, err := httpApi.PathToRID(path, r.URL.RawQuery, s.cfg.APIPath)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		s.temporaryConn(w, r, func(c *wsConn, cb func(interface{}, error)) {
			c.GetHTTPResource(rid, s.cfg.APIPath, cb)
		})

	case "POST":
		rid, action, err := httpApi.PathToRIDAction(path, r.URL.RawQuery, s.cfg.APIPath)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		// Try to parse the body
		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			// [TODO] Send a more proper response
			http.Error(w, err.Error(), 500)
			return
		}

		var params json.RawMessage
		if strings.TrimSpace(string(b)) != "" {
			err = json.Unmarshal(b, &params)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
		}

		s.temporaryConn(w, r, func(c *wsConn, cb func(interface{}, error)) {
			switch action {
			case "new":
				c.NewHTTPResource(rid, s.cfg.APIPath, params, func(href string, err error) {
					if err == nil {
						w.Header().Set("Location", href)
						w.WriteHeader(http.StatusCreated)
					}
					cb(nil, err)
				})
			default:
				c.CallResource(rid, action, params, cb)
			}
		})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Service) temporaryConn(w http.ResponseWriter, r *http.Request, cb func(*wsConn, func(interface{}, error))) {
	c := s.newWSConn(nil, r)
	if c == nil {
		// [TODO] Send a more proper response
		http.NotFound(w, r)
		return
	}

	done := make(chan struct{})
	rs := func(data interface{}, err error) {
		defer c.dispose()
		defer close(done)

		var out []byte
		if err != nil {
			httpError(w, err)
			return
		}

		if data != nil {
			out, err = json.Marshal(data)
			if err != nil {
				httpError(w, err)
				return
			}

			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Write(out)
		}
	}
	c.Enqueue(func() {
		if s.cfg.HeaderAuth != nil {
			c.AuthResource(s.cfg.headerAuthRID, s.cfg.headerAuthAction, nil, func(result interface{}, err error) {
				cb(c, rs)
			})
		} else {
			cb(c, rs)
		}
	})
	<-done
}

func httpError(w http.ResponseWriter, err error) {
	rerr := reserr.RESError(err)
	out, err := json.Marshal(rerr)
	if err != nil {
		httpError(w, err)
		return
	}

	var code int
	switch rerr.Code {
	case "system.notFound":
		fallthrough
	case "system.timeout":
		code = http.StatusNotFound
	case "system.accessDenied":
		code = http.StatusUnauthorized
	case "system.internalError":
		fallthrough
	default:
		code = http.StatusInternalServerError
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	w.Write(out)
}
