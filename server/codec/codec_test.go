package codec

import (
	"encoding/json"
	"testing"
)

// Test IsLegacyChangeEvent properly detects legacy v1.0 change events
// Remove after 2020-03-31
func TestIsLegacyChangeEvent(t *testing.T) {
	tbl := []struct {
		Payload  string
		Expected bool
	}{
		{` [}`, false},                        // Broken JSON
		{`"foo"`, false},                      // Broken event
		{`{ "foo": "bar", "faz": 42 }`, true}, // Multiple props
		{`{ "props": {"action": "delete"}, "faz": 42 }`, true},   // Multiple props with delete action
		{`{ "props": {"rid": "example.bar"}, "faz": 42 }`, true}, // Multiple props with resource reference
		{`{ "props": {"foo": "bar"}, "faz": 42 }`, true},         // Invalid multiple props
		{`{ "foo": "bar" }`, true},                               // Single prop
		{`{ "props": "bar" }`, true},                             // Single prop named props
		{` { "props": "bar" }`, true},                            // Single prop named props with leading whitespace
		{"\t\r\n { \"props\": \"bar\" }", true},                  // Single prop named props with all types of whitespace
		{`{ "props": { "foo": "bar" }}`, false},                  // Non-legacy with single prop
		{`{ "props": { "foo": "bar", "faz": 42 }}`, false},       // Non-legacy with multiple props
		{` { "props": { "foo": "bar" }}`, false},                 // Non-legacy with single prop with leading whitespace

		// The ones below should be the only false negatives
		{`{ "props": { "action": "delete" }}`, false},   // Legacy with delete action
		{`{ "props": { "rid": "example.foo" }}`, false}, // Legacy with resource reference
	}

	for _, r := range tbl {
		isLegacy := IsLegacyChangeEvent(json.RawMessage(r.Payload))

		if isLegacy != r.Expected {
			if r.Expected {
				t.Fatalf("expected %+v to be detected as legacy, but it wasn't", string(r.Payload))
			} else {
				t.Fatalf("expected %+v not to be detected as legacy, but it was", string(r.Payload))
			}
		}
	}
}
