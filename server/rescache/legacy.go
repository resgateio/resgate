package rescache

import (
	"bytes"
	"encoding/json"

	"github.com/resgateio/resgate/server/codec"
)

// Legacy120Model marshals a model compatible with version 1.2.0
// (versionSoftResourceReference) and below.
type Legacy120Model Model

// Legacy120Collection marshals a collection compatible with version 1.2.0
// (versionSoftResourceReference) and below.
type Legacy120Collection Collection

// Legacy120Value marshals a value compatible with version 1.2.0
// (versionSoftResourceReference) and below.
type Legacy120Value codec.Value

// Legacy120ValueMap marshals a map of values compatible with version 1.2.0
// (versionSoftResourceReference) and below.
type Legacy120ValueMap map[string]codec.Value

var legacyDataPlaceholderBytes = []byte(`"[Data]"`)

// MarshalJSON creates a JSON encoded representation of the model
func (m *Legacy120Model) MarshalJSON() ([]byte, error) {
	for _, v := range m.Values {
		if v.Type == codec.ValueTypeSoftReference || v.Type == codec.ValueTypeData {
			return Legacy120ValueMap(m.Values).MarshalJSON()
		}
	}
	return (*Model)(m).MarshalJSON()
}

// MarshalJSON creates a JSON encoded representation of the model
func (c *Legacy120Collection) MarshalJSON() ([]byte, error) {
	for _, v := range c.Values {
		if v.Type == codec.ValueTypeSoftReference || v.Type == codec.ValueTypeData {
			goto LegacyMarshal
		}
	}
	return (*Collection)(c).MarshalJSON()

LegacyMarshal:

	vs := c.Values
	lvs := make([]Legacy120Value, len(vs))
	for i, v := range vs {
		lvs[i] = Legacy120Value(v)
	}

	return json.Marshal(lvs)
}

// MarshalJSON creates a JSON encoded representation of the value
func (v Legacy120Value) MarshalJSON() ([]byte, error) {
	if v.Type == codec.ValueTypeSoftReference {
		return json.Marshal(v.RID)
	}
	if v.Type == codec.ValueTypeData {
		return legacyDataPlaceholderBytes, nil
	}
	return v.RawMessage, nil
}

// MarshalJSON creates a JSON encoded representation of the map
func (m Legacy120ValueMap) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	notfirst := false
	b.WriteByte('{')
	for k, v := range m {
		if notfirst {
			b.WriteByte(',')
		}
		notfirst = true
		dta, err := json.Marshal(k)
		if err != nil {
			return nil, err
		}
		b.Write(dta)
		b.WriteByte(':')
		dta, err = Legacy120Value(v).MarshalJSON()
		if err != nil {
			return nil, err
		}
		b.Write(dta)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}
