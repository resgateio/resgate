package test

import (
	"testing"

	"github.com/jirenius/resgate/server/codec"
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
