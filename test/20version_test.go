package test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/resgateio/resgate/server/reserr"
)

func TestVersion_Request_ReturnsExpectedResponse(t *testing.T) {

	tbl := []struct {
		Params   json.RawMessage
		Expected interface{}
	}{
		// Valid requests
		{nil, versionResult},
		{json.RawMessage(`{}`), versionResult},
		{json.RawMessage(`{"foo":"bar"}`), versionResult},
		{json.RawMessage(`{"protocol":""}`), versionResult},
		{json.RawMessage(`{"protocol":null}`), versionResult},
		{json.RawMessage(`{"protocol":"1.0.0"}`), versionResult},
		{json.RawMessage(`{"protocol":"1.1.0"}`), versionResult},
		{json.RawMessage(`{"protocol":"1.0.1"}`), versionResult},
		{json.RawMessage(`{"protocol":"1.999.999"}`), versionResult},
		// Invalid params
		{json.RawMessage(`""`), reserr.ErrInvalidParams},
		{json.RawMessage(`"1.0.0"`), reserr.ErrInvalidParams},
		{json.RawMessage(`["1.0.0"]`), reserr.ErrInvalidParams},
		{json.RawMessage(`{"protocol":1.0}`), reserr.ErrInvalidParams},
		{json.RawMessage(`{"protocol":"1.0"}`), reserr.ErrInvalidParams},
		{json.RawMessage(`{"protocol":"1.2.3.4"}`), reserr.ErrInvalidParams},
		{json.RawMessage(`{"protocol":"v1.0.0"}`), reserr.ErrInvalidParams},
		{json.RawMessage(`{"protocol":"1.0.1000"}`), reserr.ErrInvalidParams},
		{json.RawMessage(`{"protocol":"1.1000.0"}`), reserr.ErrInvalidParams},
		{json.RawMessage(`{"protocol":"v1.0.0"}`), reserr.ErrInvalidParams},
		// Unsupported protocol
		{json.RawMessage(`{"protocol":"0.0.0"}`), reserr.ErrUnsupportedProtocol},
		{json.RawMessage(`{"protocol":"2.0.0"}`), reserr.ErrUnsupportedProtocol},
		{json.RawMessage(`{"protocol":"0.999.999"}`), reserr.ErrUnsupportedProtocol},
		{json.RawMessage(`{"protocol":"3.2.1"}`), reserr.ErrUnsupportedProtocol},
	}

	for i, l := range tbl {
		runNamedTest(t, fmt.Sprintf("#%d", i+1), func(s *Session) {
			c := s.Connect()

			// Send client call request
			creq := c.Request("version", l.Params)

			// Validate client response
			cresp := creq.GetResponse(t)
			if err, ok := l.Expected.(*reserr.Error); ok {
				cresp.AssertError(t, err)
			} else {
				cresp.AssertResult(t, l.Expected)
			}
		})
	}
}
