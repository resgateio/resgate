package test

import (
	"testing"

	"github.com/resgateio/resgate/server/rescache"
)

func TestParseResourcePattern_WithValidPatterns_IsValidReturnsTrue(t *testing.T) {
	tbl := []struct {
		Pattern string
	}{
		{"test"},
		{"test.model"},
		{"test.model.foo"},
		{"test$.model"},
		{"$test.model"},

		{">"},
		{"test.>"},
		{"test.model.>"},

		{"*"},
		{"test.*"},
		{"*.model"},
		{"test.*.foo"},
		{"test.model.*"},
		{"*.model.foo"},
		{"test.*.*"},

		{"test.*.>"},
	}

	for _, r := range tbl {
		if !rescache.ParseResourcePattern(r.Pattern).IsValid() {
			t.Errorf("ResourcePattern(%#v).IsValid() did not return true", r.Pattern)
		}
	}
}

func TestParseResourcePattern_WithInvalidPatterns_IsValidReturnsFalse(t *testing.T) {
	tbl := []struct {
		Pattern string
	}{
		{""},
		{"."},
		{".test"},
		{"test."},
		{"test..foo"},

		{"*test"},
		{"test*"},
		{"test.*foo"},
		{"test.foo*"},
		{"test.**.foo"},

		{">test"},
		{"test>"},
		{"test.>foo"},
		{"test.foo>"},
		{"test.>.foo"},

		{"test.$foo>"},
		{"test.$foo*"},

		{"test.foo?"},
		{"test. .foo"},
		{"test.\x127.foo"},
		{"test.räv"},
	}

	for _, r := range tbl {
		if rescache.ParseResourcePattern(r.Pattern).IsValid() {
			t.Errorf("ResourcePattern(%#v).IsValid() did not return false", r.Pattern)
		}
	}
}

func TestResourcePatternMatche_MatchingPattern_ReturnsTrue(t *testing.T) {
	tbl := []struct {
		Pattern      string
		ResourceName string
	}{
		{"test", "test"},
		{"test.model", "test.model"},
		{"test.model.foo", "test.model.foo"},
		{"test$.model", "test$.model"},
		{"$foo.model", "$foo.model"},

		{">", "test.model.foo"},
		{"test.>", "test.model.foo"},
		{"test.model.>", "test.model.foo"},

		{"*", "test"},
		{"test.*", "test.model"},
		{"*.model", "test.model"},
		{"test.*.foo", "test.model.foo"},
		{"test.model.*", "test.model.foo"},
		{"*.model.foo", "test.model.foo"},
		{"test.*.*", "test.model.foo"},

		{"test.*.>", "test.model.foo"},
	}

	for _, r := range tbl {
		if !rescache.ParseResourcePattern(r.Pattern).Match(r.ResourceName) {
			t.Errorf("ParseResourcePattern(%#v).Matches(%#v) did not return true", r.Pattern, r.ResourceName)
		}
	}
}

func TestResourcePatternMatche_NonMatchingPattern_ReturnsFalse(t *testing.T) {
	tbl := []struct {
		Pattern      string
		ResourceName string
	}{
		{"", ""},
		{"", "test"},
		{"test", "test.model"},
		{"test.model", "test.mode"},
		{"test.model.foo", "test.model"},
		{"test.model.foo", "test.mode.foo"},
		{"test", "testing"},
		{"testing", "test"},
		{"foo", "bar"},

		{">", ""},
		{"test.>", "test"},
		{"test.model.>", "test.model"},

		{"*", "test.model"},
		{"test.*", "test.model.foo"},
		{"*.model", "test"},
		{"test.*.foo", "test.model"},
		{"test.model.*", "test.model"},
		{"*.model.foo", "test.model"},
		{"test.*.*", "test.model"},
		{"foo.*", "bar.model"},
		{"*.bar", "foo.baz"},

		{"test.*.>", "test.model"},
	}

	for _, r := range tbl {
		if rescache.ParseResourcePattern(r.Pattern).Match(r.ResourceName) {
			t.Errorf("ParseResourcePattern(%#v).Match(%#v) did not return false", r.Pattern, r.ResourceName)
		}
	}
}
