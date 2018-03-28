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

	s.Logf(path)

	switch r.Method {
	case "GET":

		rid, err := httpApi.PathToRID(path, r.URL.RawQuery, s.cfg.APIPath)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		c := s.newWSConn(nil, r)
		if c == nil {
			// [TODO] Send a more proper response
			http.NotFound(w, r)
			return
		}

		done := make(chan struct{})
		c.Enqueue(func() {
			if s.cfg.HeaderAuth != nil {
				c.AuthResource(s.cfg.headerAuthRID, s.cfg.headerAuthAction, nil, func(result interface{}, err error) {
					c.GetHTTPResource(rid, s.cfg.APIPath, responseSender(w, c, done))
				})
			} else {
				c.GetHTTPResource(rid, s.cfg.APIPath, responseSender(w, c, done))
			}
		})
		<-done

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

		c := s.newWSConn(nil, r)
		if c == nil {
			// [TODO] Send a more proper response
			http.NotFound(w, r)
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

		done := make(chan struct{})
		c.Enqueue(func() {
			c.CallResource(rid, action, params, responseSender(w, c, done))
		})
		<-done

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func responseSender(w http.ResponseWriter, c *wsConn, done chan struct{}) func(interface{}, error) {
	return func(data interface{}, err error) {
		defer c.dispose()
		defer close(done)

		var out []byte
		if err != nil {
			httpError(w, err)
			return
		}

		out, err = json.Marshal(data)
		if err != nil {
			httpError(w, err)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write(out)
	}
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
		code = 404
	case "system.accessDenied":
		code = 401
	case "system.internalError":
		fallthrough
	default:
		code = 500
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	w.Write(out)
}
