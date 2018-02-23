package httpApi

import (
	"encoding/json"
	"errors"
	"net/url"
	"strings"
)

var (
	errInvalidPath = errors.New("Invalid path")
)

// Resource holds a resource information to be sent to the client
type Resource struct {
	APIPath    string
	ResourceID string
	Data       interface{}
	Error      error
}

// MarshalJSON implements the json.Marshaler interface
func (r *Resource) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		HREF  string      `json:"href"`
		Data  interface{} `json:"data,omitempty"`
		Error error       `json:"error,omitempty"`
	}{
		HREF:  ResourceIDToPath(r.ResourceID, r.APIPath),
		Data:  r.Data,
		Error: r.Error,
	})
}

// PathToResourceID parses a raw URL path and returns the resourceID.
// The prefix is the beginning of the path which is not part of the
// resourceID, and it should both start and end with /. Eg. "/api/"
func PathToResourceID(path, query, prefix string) (string, error) {
	if len(path) == 0 {
		return "", errInvalidPath
	}

	path = strings.TrimPrefix(path, prefix)

	if path[0] == '/' {
		path = path[1:]
	}
	parts := strings.Split(path, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		part, err := url.PathUnescape(parts[i])
		if err != nil {
			return "", errInvalidPath
		}
		parts[i] = part
	}

	rid := strings.Join(parts, ".")
	if query != "" {
		rid += "?" + query
	}

	return rid, nil
}

// PathToResourceIDAction parses a raw URL path and returns the resourceID and action.
// The prefix is the beginning of the path which is not part of the
// resourceID, and it should both start and end with /. Eg. "/api/"
func PathToResourceIDAction(path, query, prefix string) (string, string, error) {
	if len(path) == 0 {
		return "", "", errInvalidPath
	}

	path = strings.TrimPrefix(path, prefix)

	if path[0] == '/' {
		path = path[1:]
	}
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return "", "", errInvalidPath
	}

	for i := len(parts) - 1; i >= 0; i-- {
		part, err := url.PathUnescape(parts[i])
		if err != nil {
			return "", "", errInvalidPath
		}
		parts[i] = part
	}

	rid := strings.Join(parts[:len(parts)-1], ".")
	if query != "" {
		rid += query
	}

	return rid, parts[len(parts)-1], nil
}

// ResourceIDToPath converts a resourceID to a URL path string.
// The prefix is the part of the path that should be prepended
// to the resourceID path, and it should both start and end with /. Eg. "/api/".
func ResourceIDToPath(resourceID, prefix string) string {
	return prefix + strings.Replace(url.PathEscape(resourceID), ".", "/", -1)
}
