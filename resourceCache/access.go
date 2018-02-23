package resourceCache

import (
	"../mq/codec"
	"../reserr"
)

type Access struct {
	*codec.AccessResult
	Error error
}

func (a *Access) CanGet() error {
	if a.Error != nil {
		return a.Error
	}

	if a.Get {
		return nil
	}

	return reserr.ErrAccessDenied
}

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
