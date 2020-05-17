package test

import (
	"encoding/json"
	"testing"

	"github.com/raphaelpereira/resgate/server/codec"
)

// Test IsValidRID method
func TestIsValidRID(t *testing.T) {
	tbl := []struct {
		RID        string
		AllowQuery bool
		Valid      bool
	}{
		// Valid RID
		{"test", true, true},
		{"test.model", true, true},
		{"test.model._foo_", true, true},
		{"test.model.<foo", true, true},
		{"test.model.23", true, true},
		{"test.model.23?", true, true},
		{"test.model.23?foo=bar", true, true},
		{"test.model.23?foo=test.bar", true, true},
		{"test.model.23?foo=*&?", true, true},
		// Invalid RID
		{"", true, false},
		{".test", true, false},
		{"test.", true, false},
		{".test.model", true, false},
		{"test..model", true, false},
		{"test.model.", true, false},
		{".test.model", true, false},
		{"test\tmodel", true, false},
		{"test\nmodel", true, false},
		{"test\rmodel", true, false},
		{"test model", true, false},
		{"test\ufffdmodel", true, false},
		{"täst.model", true, false},
		{"test.*.model", true, false},
		{"test.>.model", true, false},
		{"test.model.>", true, false},
		{"?foo=test.bar", true, false},
		{".test.model?foo=test.bar", true, false},
		{"test..model?foo=test.bar", true, false},
		{"test.model.?foo=test.bar", true, false},
		{".test.model?foo=test.bar", true, false},
		{"test\tmodel?foo=test.bar", true, false},
		{"test\nmodel?foo=test.bar", true, false},
		{"test\rmodel?foo=test.bar", true, false},
		{"test model?foo=test.bar", true, false},
		{"test\ufffdmodel?foo=test.bar", true, false},
		{"täst.model?foo=test.bar", true, false},
		{"test.*.model?foo=test.bar", true, false},
		{"test.>.model?foo=test.bar", true, false},
		{"test.model.>?foo=test.bar", true, false},
		// Invalid RID with allowQuery set to false
		{"test.model.23?", false, false},
		{"test.model.23?foo=bar", false, false},
		{"test.model.23?foo=test.bar", false, false},
		{"test.model.23?foo=*&?", false, false},
		// Invalid RID independent on value of allowQuery
		{"?foo=test.bar", false, false},
		{".test.model?foo=test.bar", false, false},
		{"test..model?foo=test.bar", false, false},
		{"test.model.?foo=test.bar", false, false},
		{".test.model?foo=test.bar", false, false},
		{"test\tmodel?foo=test.bar", false, false},
		{"test\nmodel?foo=test.bar", false, false},
		{"test\rmodel?foo=test.bar", false, false},
		{"test model?foo=test.bar", false, false},
		{"test\ufffdmodel?foo=test.bar", false, false},
		{"täst.model?foo=test.bar", false, false},
		{"test.*.model?foo=test.bar", false, false},
		{"test.>.model?foo=test.bar", false, false},
		{"test.model.>?foo=test.bar", false, false},
	}

	for _, l := range tbl {
		v := codec.IsValidRID(l.RID, l.AllowQuery)
		if v != l.Valid {
			if l.Valid {
				t.Errorf("expected RID %#v to be valid, but it wasn't", l.RID)
			} else {
				t.Errorf("expected RID %#v not to be valid, but it was", l.RID)
			}
		}
	}
}

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
		isLegacy := codec.IsLegacyChangeEvent(json.RawMessage(r.Payload))

		if isLegacy != r.Expected {
			if r.Expected {
				t.Fatalf("expected %+v to be detected as legacy, but it wasn't", string(r.Payload))
			} else {
				t.Fatalf("expected %+v not to be detected as legacy, but it was", string(r.Payload))
			}
		}
	}
}
