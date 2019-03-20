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
		{`{ "foo": "bar", "faz": 42 }`, true}, // Multiple values
		{`{ "values": {"action": "delete"}, "faz": 42 }`, true},   // Multiple values with delete action
		{`{ "values": {"rid": "example.bar"}, "faz": 42 }`, true}, // Multiple values with resource reference
		{`{ "values": {"foo": "bar"}, "faz": 42 }`, true},         // Invalid multiple values
		{`{ "foo": "bar" }`, true},                                // Single value
		{`{ "values": "bar" }`, true},                             // Single value named values
		{` { "values": "bar" }`, true},                            // Single value named values with leading whitespace
		{"\t\r\n { \"values\": \"bar\" }", true},                  // Single value named values with all types of whitespace
		{`{ "values": { "foo": "bar" }}`, false},                  // Non-legacy with single value
		{`{ "values": { "foo": "bar", "faz": 42 }}`, false},       // Non-legacy with multiple values
		{` { "values": { "foo": "bar" }}`, false},                 // Non-legacy with single value with leading whitespace

		// The ones below should be the only false negatives
		{`{ "values": { "action": "delete" }}`, false},   // Legacy with delete action
		{`{ "values": { "rid": "example.foo" }}`, false}, // Legacy with resource reference
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
