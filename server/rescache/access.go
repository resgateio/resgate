package rescache

import (
	"github.com/jirenius/resgate/server/codec"
	"github.com/jirenius/resgate/server/reserr"
)

// Access represents a RES-service access response
type Access struct {
	*codec.AccessResult
	Error error
}

// CanGet reports wheter get access is granted.
// Returns nil if get access is granted, otherwise an error.
func (a *Access) CanGet() error {
	if a.Error != nil {
		return a.Error
	}

	if a.Get {
		return nil
	}

	return reserr.ErrAccessDenied
}

// CanCall reports wheter call access for a given action is granted.
// Returns nil if get access is granted, otherwise an error.
func (a *Access) CanCall(action string) error {
	if a.Error != nil {
		return a.Error
	}

	if a.Call == "*" {
		return nil
	}

	if a.Call == "" {
		return reserr.ErrAccessDenied
	}

	s := a.Call
	e := len(s)
	i := e
	for {
		i--
		if i == -1 || s[i] == ',' {
			if string(s[i+1:e]) == action {
				return nil
			}

			if i == -1 {
				break
			}
			e = i
		}
	}

	return reserr.ErrAccessDenied
}
