package test

var resource = map[string]string{
	"test.model":                   `{"string":"foo","int":42,"bool":true,"null":null}`,
	"test.model.parent":            `{"name":"parent","child":{"rid":"test.model"}}`,
	"test.model.secondparent":      `{"name":"secondparent","child":{"rid":"test.model"}}`,
	"test.model.grandparent":       `{"name":"grandparent","child":{"rid":"test.model.parent"}}`,
	"test.model.a":                 `{"bref":{"rid":"test.model.b"}}`,
	"test.model.b":                 `{"aref":{"rid":"test.model.a"},"bref":{"rid":"test.model.b"}}`,
	"test.collection":              `["foo",42,true,null]`,
	"test.collection.parent":       `["parent",{"rid":"test.collection"}]`,
	"test.collection.secondparent": `["secondparent",{"rid":"test.collection"}]`,
	"test.collection.grandparent":  `["grandparent",{"rid":"test.collection.parent"},null]`,
	"test.collection.a":            `[{"rid":"test.collection.b"}]`,
	"test.collection.b":            `[{"rid":"test.collection.a"},{"rid":"test.collection.b"}]`,
}
