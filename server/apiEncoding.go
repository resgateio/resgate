package server

import (
	"bytes"
	"encoding/json"
	"net/url"
	"strings"

	"github.com/resgateio/resgate/server/codec"
	"github.com/resgateio/resgate/server/rescache"
	"github.com/resgateio/resgate/server/reserr"
)

var nullBytes = []byte("null")

// APIEncoderFactory create an APIEncoder.
type APIEncoderFactory func(cfg Config) APIEncoder

// APIEncoder encodes responses to HTTP API requests.
type APIEncoder interface {
	EncodeGET(*Subscription) ([]byte, error)
	EncodePOST(json.RawMessage) ([]byte, error)
	EncodeError(*reserr.Error) []byte
	NotFoundError() []byte
	ContentType() string
}

var apiEncoderFactories = make(map[string]APIEncoderFactory)

// RegisterAPIEncoderFactory adds an APIEncoderFactory by name.
// Panics if another factory with the same name is already registered.
func RegisterAPIEncoderFactory(name string, f APIEncoderFactory) {
	if _, ok := apiEncoderFactories[name]; ok {
		panic("multiple registration of API encoder factory " + name)
	}
	apiEncoderFactories[name] = f
}

// PathToRID parses a raw URL path and returns the resource ID.
// The prefix is the beginning of the path which is not part of the
// resource ID, and it should both start and end with /. Eg. "/api/"
func PathToRID(path, query, prefix string) string {
	if len(path) == len(prefix) || !strings.HasPrefix(path, prefix) {
		return ""
	}

	path = path[len(prefix):]

	// Dot separator not allowed in path
	if strings.ContainsRune(path, '.') {
		return ""
	}

	if path[0] == '/' {
		path = path[1:]
	}
	parts := strings.Split(path, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		part, err := url.PathUnescape(parts[i])
		if err != nil {
			return ""
		}
		parts[i] = part
	}

	rid := strings.Join(parts, ".")
	if query != "" {
		rid += "?" + query
	}

	return rid
}

// PathToRIDAction parses a raw URL path and returns the resource ID and action.
// The prefix is the beginning of the path which is not part of the
// resource ID, and it should both start and end with /. Eg. "/api/"
func PathToRIDAction(path, query, prefix string) (string, string) {
	if len(path) == len(prefix) || !strings.HasPrefix(path, prefix) {
		return "", ""
	}

	path = path[len(prefix):]

	// Dot separator not allowed in path
	if strings.ContainsRune(path, '.') {
		return "", ""
	}

	if path[0] == '/' {
		path = path[1:]
	}
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return "", ""
	}

	for i := len(parts) - 1; i >= 0; i-- {
		part, err := url.PathUnescape(parts[i])
		if err != nil {
			return "", ""
		}
		parts[i] = part
	}

	rid := strings.Join(parts[:len(parts)-1], ".")
	if query != "" {
		rid += "?" + query
	}

	return rid, parts[len(parts)-1]
}

// RIDToPath converts a resource ID to a URL path string.
// The prefix is the part of the path that should be prepended
// to the resource ID path, and it should both start and end with /. Eg. "/api/".
func RIDToPath(rid, prefix string) string {
	if rid == "" {
		return ""
	}
	return prefix + strings.Replace(url.PathEscape(rid), ".", "/", -1)
}

func init() {
	b, err := json.Marshal(reserr.ErrNotFound)
	if err != nil {
		panic(err)
	}

	RegisterAPIEncoderFactory("json", func(cfg Config) APIEncoder {
		return &encoderJSON{apiPath: cfg.APIPath, notFoundBytes: b}

	})
	RegisterAPIEncoderFactory("jsonflat", func(cfg Config) APIEncoder {
		return &encoderJSONFlat{apiPath: cfg.APIPath, notFoundBytes: b}
	})
}

type encoderJSON struct {
	b             bytes.Buffer
	path          []string
	apiPath       string
	notFoundBytes []byte
}

func (e *encoderJSON) ContentType() string {
	return "application/json; charset=utf-8"
}

func (e *encoderJSON) EncodeGET(s *Subscription) ([]byte, error) {
	// Clone encoder for concurrency safety
	ec := encoderJSON{
		apiPath:       e.apiPath,
		notFoundBytes: e.notFoundBytes,
	}

	err := ec.encodeSubscription(s, false)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(ec.b.Bytes()), nil
}

func (e *encoderJSON) EncodePOST(r json.RawMessage) ([]byte, error) {
	b := []byte(r)
	if bytes.Equal(b, nullBytes) {
		return nil, nil
	}
	return b, nil
}

func (e *encoderJSON) EncodeError(rerr *reserr.Error) []byte {
	return jsonEncodeError(rerr)
}

func (e *encoderJSON) NotFoundError() []byte {
	return e.notFoundBytes
}

func (e *encoderJSON) encodeSubscription(s *Subscription, wrap bool) error {
	rid := s.RID()

	if wrap {
		e.b.Write([]byte(`{"href":`))
		dta, err := json.Marshal(RIDToPath(rid, e.apiPath))
		if err != nil {
			return err
		}
		e.b.Write(dta)
		defer e.b.WriteByte('}')
	}

	// Check for cyclic reference
	if containsString(e.path, rid) {
		return nil
	}

	// Check for errors
	if err := s.Error(); err != nil {
		if wrap {
			e.b.Write([]byte(`,"error":`))
		}
		e.b.Write(jsonEncodeError(reserr.RESError(err)))
		return nil
	}

	// Add itself to path
	e.path = append(e.path, s.rid)

	switch s.ResourceType() {
	case rescache.TypeCollection:
		if wrap {
			e.b.Write([]byte(`,"collection":`))
		}
		e.b.WriteByte('[')
		vals := s.CollectionValues()
		for i, v := range vals {
			if i > 0 {
				e.b.WriteByte(',')
			}
			if err := e.encodeValue(s, v); err != nil {
				return err
			}
		}
		e.b.WriteByte(']')

	case rescache.TypeModel:
		if wrap {
			e.b.Write([]byte(`,"model":`))
		}
		e.b.WriteByte('{')
		vals := s.ModelValues()
		first := true
		for k, v := range vals {
			// Write comma separator
			if !first {
				e.b.WriteByte(',')
			}
			first = false

			// Write object key
			dta, err := json.Marshal(k)
			if err != nil {
				return err
			}
			e.b.Write(dta)
			e.b.WriteByte(':')

			if err := e.encodeValue(s, v); err != nil {
				return err
			}
		}
		e.b.WriteByte('}')
	}

	// Remove itself from path
	e.path = e.path[:len(e.path)-1]

	return nil
}

func (e *encoderJSON) encodeValue(s *Subscription, v codec.Value) error {
	switch v.Type {
	case codec.ValueTypeReference:
		sc := s.Ref(v.RID)
		if err := e.encodeSubscription(sc, true); err != nil {
			return err
		}
	case codec.ValueTypeSoftReference:
		e.b.Write([]byte(`{"href":`))
		dta, err := json.Marshal(RIDToPath(v.RID, e.apiPath))
		if err != nil {
			return err
		}
		e.b.Write(dta)
		e.b.WriteByte('}')
	case codec.ValueTypeData:
		e.b.Write(v.Inner)
	default:
		e.b.Write(v.RawMessage)
	}
	return nil
}

type encoderJSONFlat struct {
	b             bytes.Buffer
	path          []string
	apiPath       string
	notFoundBytes []byte
}

func (e *encoderJSONFlat) ContentType() string {
	return "application/json; charset=utf-8"
}

func (e *encoderJSONFlat) EncodeGET(s *Subscription) ([]byte, error) {
	// Clone encoder for concurrency safety
	ec := encoderJSONFlat{
		apiPath:       e.apiPath,
		notFoundBytes: e.notFoundBytes,
	}

	err := ec.encodeSubscription(s)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(ec.b.Bytes()), nil
}

func (e *encoderJSONFlat) EncodePOST(r json.RawMessage) ([]byte, error) {
	b := []byte(r)
	if bytes.Equal(b, nullBytes) {
		return nil, nil
	}
	return b, nil
}

func (e *encoderJSONFlat) EncodeError(rerr *reserr.Error) []byte {
	return jsonEncodeError(rerr)
}

func (e *encoderJSONFlat) NotFoundError() []byte {
	return e.notFoundBytes
}

func (e *encoderJSONFlat) encodeSubscription(s *Subscription) error {
	rid := s.RID()

	// Check for cyclic reference
	if containsString(e.path, rid) {
		e.b.Write([]byte(`{"href":`))
		dta, err := json.Marshal(RIDToPath(rid, e.apiPath))
		if err != nil {
			return err
		}
		e.b.Write(dta)
		e.b.WriteByte('}')
		return nil
	}

	// Check for errors
	if err := s.Error(); err != nil {
		e.b.Write(jsonEncodeError(reserr.RESError(err)))
		return nil
	}

	// Add itself to path
	e.path = append(e.path, s.rid)

	switch s.ResourceType() {
	case rescache.TypeCollection:
		e.b.WriteByte('[')
		vals := s.CollectionValues()
		for i, v := range vals {
			if i > 0 {
				e.b.WriteByte(',')
			}
			if err := e.encodeValue(s, v); err != nil {
				return err
			}
		}
		e.b.WriteByte(']')

	case rescache.TypeModel:
		e.b.WriteByte('{')
		vals := s.ModelValues()
		first := true
		for k, v := range vals {
			// Write comma separator
			if !first {
				e.b.WriteByte(',')
			}
			first = false

			// Write object key
			dta, err := json.Marshal(k)
			if err != nil {
				return err
			}
			e.b.Write(dta)
			e.b.WriteByte(':')

			if err := e.encodeValue(s, v); err != nil {
				return err
			}
		}
		e.b.WriteByte('}')
	}

	// Remove itself from path
	e.path = e.path[:len(e.path)-1]
	return nil
}

func (e *encoderJSONFlat) encodeValue(s *Subscription, v codec.Value) error {
	switch v.Type {
	case codec.ValueTypeReference:
		sc := s.Ref(v.RID)
		if err := e.encodeSubscription(sc); err != nil {
			return err
		}
	case codec.ValueTypeSoftReference:
		e.b.Write([]byte(`{"href":`))
		dta, err := json.Marshal(RIDToPath(v.RID, e.apiPath))
		if err != nil {
			return err
		}
		e.b.Write(dta)
		e.b.WriteByte('}')
	case codec.ValueTypeData:
		e.b.Write(v.Inner)
	default:
		e.b.Write(v.RawMessage)
	}
	return nil
}

func jsonEncodeError(rerr *reserr.Error) []byte {
	out, err := json.Marshal(rerr)
	if err != nil {
		return jsonEncodeError(reserr.RESError(err))
	}
	return out
}
