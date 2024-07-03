package test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/resgateio/resgate/server/codec"
	"github.com/resgateio/resgate/server/rescache"
)

func TestLegacy120Model_MarshalJSON_ReturnsSoftReferenceAsString(t *testing.T) {
	var v map[string]codec.Value
	dta := []byte(`{"name":"softparent","child":{"rid":"test.model","soft":true}}`)
	expected := []byte(`{"name":"softparent","child":"test.model"}`)
	err := json.Unmarshal(dta, &v)
	if err != nil {
		t.Fatal(err)
	}
	m := &rescache.Model{Values: v}
	lm := (*rescache.Legacy120Model)(m)

	out, err := lm.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

	AssertEqualJSON(t, "Legacy120Model.MarshalJSON", json.RawMessage(out), json.RawMessage(expected))
}

func TestLegacy120Model_MarshalJSON_ReturnsDataValuePlaceholder(t *testing.T) {
	var v map[string]codec.Value
	dta := []byte(`{"name":"data","primitive":{"data":12},"object":{"data":{"foo":["bar"]}},"array":{"data":[{"foo":"bar"}]}}`)
	expected := []byte(`{"name":"data","primitive":12,"object":"[Data]","array":"[Data]"}`)
	err := json.Unmarshal(dta, &v)
	if err != nil {
		t.Fatal(err)
	}
	m := &rescache.Model{Values: v}
	lm := (*rescache.Legacy120Model)(m)

	out, err := lm.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

	AssertEqualJSON(t, "Legacy120Model.MarshalJSON", json.RawMessage(out), json.RawMessage(expected))
}

func TestLegacy120Collection_MarshalJSON_ReturnsSoftReferenceAsString(t *testing.T) {
	var v []codec.Value
	dta := []byte(`["softparent",{"rid":"test.model","soft":true}]`)
	expected := []byte(`["softparent","test.model"]`)
	err := json.Unmarshal(dta, &v)
	if err != nil {
		t.Fatal(err)
	}
	m := &rescache.Collection{Values: v}
	lm := (*rescache.Legacy120Collection)(m)

	out, err := lm.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

	AssertEqualJSON(t, "Legacy120Collection.MarshalJSON", json.RawMessage(out), json.RawMessage(expected))
}

func TestLegacy120Collection_MarshalJSON_ReturnsDataValuePlaceholder(t *testing.T) {
	var v []codec.Value
	dta := []byte(`["data",{"data":12},{"data":{"foo":["bar"]}},{"data":[{"foo":"bar"}]}]`)
	expected := []byte(`["data",12,"[Data]","[Data]"]`)
	err := json.Unmarshal(dta, &v)
	if err != nil {
		t.Fatal(err)
	}
	m := &rescache.Collection{Values: v}
	lm := (*rescache.Legacy120Collection)(m)

	out, err := lm.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

	AssertEqualJSON(t, "Legacy120Collection.MarshalJSON", json.RawMessage(out), json.RawMessage(expected))
}

func TestLegacy120Value_MarshalJSON_ReturnsSoftReferenceAsString(t *testing.T) {
	var v codec.Value
	dta := []byte(`{"rid":"test.model","soft":true}`)
	expected := []byte(`"test.model"`)
	err := json.Unmarshal(dta, &v)
	if err != nil {
		t.Fatal(err)
	}
	lv := rescache.Legacy120Value(v)

	out, err := lv.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}

	AssertEqualJSON(t, "Legacy120Value.MarshalJSON", json.RawMessage(out), json.RawMessage(expected))
}

// AssertEqualJSON expects that a and b json marshals into equal values, and
// returns true if they do, otherwise logs a fatal error and returns false.
func AssertEqualJSON(t *testing.T, name string, result, expected interface{}, ctx ...interface{}) bool {
	aa, aj := jsonMap(t, result)
	bb, bj := jsonMap(t, expected)

	if !reflect.DeepEqual(aa, bb) {
		t.Fatalf("expected %s to be:\n\t%s\nbut got:\n\t%s%s", name, bj, aj, ctxString(ctx))
		return false
	}

	return true
}

func ctxString(ctx []interface{}) string {
	if len(ctx) == 0 {
		return ""
	}
	return "\nin " + fmt.Sprint(ctx...)
}

func jsonMap(t *testing.T, v interface{}) (interface{}, []byte) {
	var err error
	j, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("test: error marshaling value:\n\t%+v\nerror:\n\t%s", v, err)
	}

	var m interface{}
	err = json.Unmarshal(j, &m)
	if err != nil {
		t.Fatalf("test: error unmarshaling value:\n\t%s\nerror:\n\t%s", j, err)
	}

	return m, j
}
