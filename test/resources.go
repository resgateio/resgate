package test

import (
	"encoding/json"

	"github.com/resgateio/resgate/server/reserr"
)

// The following cyclic groups exist
// a -> a

// b -> c
// c -> f

// d -> e, f
// e -> d
// f -> d

// Other entry points
// g -> e, f
// h -> e
type resourceType byte

const (
	typeModel resourceType = iota
	typeCollection
	typeError
)

type resource struct {
	typ  resourceType
	data string
	err  *reserr.Error
}

func resourceData(rid string) string {
	rsrc := resources[rid]
	if rsrc.typ == typeError {
		b, _ := json.Marshal(rsrc.err)
		return string(b)
	}
	return rsrc.data
}

var resources = map[string]resource{
	// Model resources
	"test.model":              resource{typeModel, `{"string":"foo","int":42,"bool":true,"null":null}`, nil},
	"test.model.parent":       resource{typeModel, `{"name":"parent","child":{"rid":"test.model"}}`, nil},
	"test.model.secondparent": resource{typeModel, `{"name":"secondparent","child":{"rid":"test.model"}}`, nil},
	"test.model.grandparent":  resource{typeModel, `{"name":"grandparent","child":{"rid":"test.model.parent"}}`, nil},
	"test.model.brokenchild":  resource{typeModel, `{"name":"brokenchild","child":{"rid":"test.err.notFound"}}`, nil},

	// Cyclic model resources
	"test.m.a": resource{typeModel, `{"a":{"rid":"test.m.a"}}`, nil},

	"test.m.b": resource{typeModel, `{"c":{"rid":"test.m.c"}}`, nil},
	"test.m.c": resource{typeModel, `{"b":{"rid":"test.m.b"}}`, nil},

	"test.m.d": resource{typeModel, `{"e":{"rid":"test.m.e"},"f":{"rid":"test.m.f"}}`, nil},
	"test.m.e": resource{typeModel, `{"d":{"rid":"test.m.d"}}`, nil},
	"test.m.f": resource{typeModel, `{"d":{"rid":"test.m.d"}}`, nil},

	"test.m.g": resource{typeModel, `{"e":{"rid":"test.m.e"},"f":{"rid":"test.m.f"}}`, nil},
	"test.m.h": resource{typeModel, `{"e":{"rid":"test.m.e"}}`, nil},

	// Collection resources
	"test.collection":              resource{typeCollection, `["foo",42,true,null]`, nil},
	"test.collection.parent":       resource{typeCollection, `["parent",{"rid":"test.collection"}]`, nil},
	"test.collection.secondparent": resource{typeCollection, `["secondparent",{"rid":"test.collection"}]`, nil},
	"test.collection.grandparent":  resource{typeCollection, `["grandparent",{"rid":"test.collection.parent"}]`, nil},
	"test.collection.brokenchild":  resource{typeCollection, `["brokenchild",{"rid":"test.err.notFound"}]`, nil},

	// Cyclic collection resources
	"test.c.a": resource{typeCollection, `[{"rid":"test.c.a"}]`, nil},

	"test.c.b": resource{typeCollection, `[{"rid":"test.c.c"}]`, nil},
	"test.c.c": resource{typeCollection, `[{"rid":"test.c.b"}]`, nil},

	"test.c.d": resource{typeCollection, `[{"rid":"test.c.e"},{"rid":"test.c.f"}]`, nil},
	"test.c.e": resource{typeCollection, `[{"rid":"test.c.d"}]`, nil},
	"test.c.f": resource{typeCollection, `[{"rid":"test.c.d"}]`, nil},

	"test.c.g": resource{typeCollection, `[{"rid":"test.c.e"},{"rid":"test.c.f"}]`, nil},
	"test.c.h": resource{typeCollection, `[{"rid":"test.c.e"}]`, nil},

	// Errors
	"test.err.notFound":      resource{typeError, "", reserr.ErrNotFound},
	"test.err.internalError": resource{typeError, "", reserr.ErrInternalError},
	"test.err.timeout":       resource{typeError, "", reserr.ErrTimeout},
}

// Call responses
const (
	requestTimeout uint64 = iota
	noRequest
)

type sequenceEvent struct {
	Event string
	RID   string
}

var sequenceTable = [][]sequenceEvent{
	// Model tests
	{
		{"subscribe", "test.model"},
		{"access", "test.model"},
		{"get", "test.model"},
		{"response", "test.model"},
		{"event", "test.model"},
	},
	{
		{"subscribe", "test.model.parent"},
		{"access", "test.model.parent"},
		{"get", "test.model.parent"},
		{"get", "test.model"},
		{"response", "test.model.parent"},
		{"event", "test.model.parent"},
		{"event", "test.model"},
	},
	{
		{"subscribe", "test.model.grandparent"},
		{"access", "test.model.grandparent"},
		{"get", "test.model.grandparent"},
		{"get", "test.model.parent"},
		{"get", "test.model"},
		{"response", "test.model.grandparent"},
		{"event", "test.model.grandparent"},
		{"event", "test.model.parent"},
		{"event", "test.model"},
	},
	{
		{"subscribe", "test.model.parent"},
		{"access", "test.model.parent"},
		{"get", "test.model.parent"},
		{"get", "test.model"},
		{"response", "test.model.parent"},
		{"subscribe", "test.model.secondparent"},
		{"access", "test.model.secondparent"},
		{"get", "test.model.secondparent"},
		{"response", "test.model.secondparent"},
	},
	{
		{"subscribe", "test.model.brokenchild"},
		{"access", "test.model.brokenchild"},
		{"get", "test.model.brokenchild"},
		{"get", "test.err.notFound"},
		{"response", "test.model.brokenchild"},
		{"event", "test.model.brokenchild"},
		{"noevent", "test.err.notFound"},
	},
	// Cyclic model tests
	{
		{"subscribe", "test.m.a"},
		{"access", "test.m.a"},
		{"get", "test.m.a"},
		{"response", "test.m.a"},
	},
	{
		{"subscribe", "test.m.b"},
		{"access", "test.m.b"},
		{"get", "test.m.b"},
		{"get", "test.m.c"},
		{"response", "test.m.b"},
	},
	{
		{"subscribe", "test.m.d"},
		{"access", "test.m.d"},
		{"get", "test.m.d"},
		{"get", "test.m.e"},
		{"get", "test.m.f"},
		{"response", "test.m.d"},
	},
	{
		{"subscribe", "test.m.g"},
		{"access", "test.m.g"},
		{"get", "test.m.g"},
		{"get", "test.m.e"},
		{"get", "test.m.f"},
		{"get", "test.m.d"},
		{"response", "test.m.g"},
	},
	{
		{"subscribe", "test.m.d"},
		{"access", "test.m.d"},
		{"get", "test.m.d"},
		{"subscribe", "test.m.h"},
		{"access", "test.m.h"},
		{"get", "test.m.e"},
		{"get", "test.m.h"},
		{"get", "test.m.f"},
		{"response", "test.m.d"},
		{"response", "test.m.h"},
	},

	// Collection tests
	{
		{"subscribe", "test.collection"},
		{"access", "test.collection"},
		{"get", "test.collection"},
		{"response", "test.collection"},
		{"event", "test.collection"},
	},
	{
		{"subscribe", "test.collection.parent"},
		{"access", "test.collection.parent"},
		{"get", "test.collection.parent"},
		{"get", "test.collection"},
		{"response", "test.collection.parent"},
		{"event", "test.collection.parent"},
		{"event", "test.collection"},
	},
	{
		{"subscribe", "test.collection.grandparent"},
		{"access", "test.collection.grandparent"},
		{"get", "test.collection.grandparent"},
		{"get", "test.collection.parent"},
		{"get", "test.collection"},
		{"response", "test.collection.grandparent"},
		{"event", "test.collection.grandparent"},
		{"event", "test.collection.parent"},
		{"event", "test.collection"},
	},
	{
		{"subscribe", "test.collection.parent"},
		{"access", "test.collection.parent"},
		{"get", "test.collection.parent"},
		{"get", "test.collection"},
		{"response", "test.collection.parent"},
		{"subscribe", "test.collection.secondparent"},
		{"access", "test.collection.secondparent"},
		{"get", "test.collection.secondparent"},
		{"response", "test.collection.secondparent"},
	},
	{
		{"subscribe", "test.collection.brokenchild"},
		{"access", "test.collection.brokenchild"},
		{"get", "test.collection.brokenchild"},
		{"get", "test.err.notFound"},
		{"response", "test.collection.brokenchild"},
		{"event", "test.collection.brokenchild"},
		{"noevent", "test.err.notFound"},
	},
	// Cyclic collection tests
	{
		{"subscribe", "test.c.a"},
		{"access", "test.c.a"},
		{"get", "test.c.a"},
		{"response", "test.c.a"},
	},
	{
		{"subscribe", "test.c.b"},
		{"access", "test.c.b"},
		{"get", "test.c.b"},
		{"get", "test.c.c"},
		{"response", "test.c.b"},
	},
	{
		{"subscribe", "test.c.d"},
		{"access", "test.c.d"},
		{"get", "test.c.d"},
		{"get", "test.c.e"},
		{"get", "test.c.f"},
		{"response", "test.c.d"},
	},
	{
		{"subscribe", "test.c.g"},
		{"access", "test.c.g"},
		{"get", "test.c.g"},
		{"get", "test.c.e"},
		{"get", "test.c.f"},
		{"get", "test.c.d"},
		{"response", "test.c.g"},
	},
	{
		{"subscribe", "test.c.d"},
		{"access", "test.c.d"},
		{"get", "test.c.d"},
		{"subscribe", "test.c.h"},
		{"access", "test.c.h"},
		{"get", "test.c.e"},
		{"get", "test.c.h"},
		{"get", "test.c.f"},
		{"response", "test.c.d"},
		{"response", "test.c.h"},
	},
	// Access test
	{
		{"subscribe", "test.model.parent"},
		{"access", "test.model.parent"},
		{"get", "test.model.parent"},
		{"get", "test.model"},
		{"response", "test.model.parent"},
		{"subscribe", "test.model"},
		{"access", "test.model"},
		{"response", "test.model"},
		{"event", "test.model.parent"},
		{"event", "test.model"},
	},
	{
		{"subscribe", "test.model.parent"},
		{"access", "test.model.parent"},
		{"get", "test.model.parent"},
		{"get", "test.model"},
		{"response", "test.model.parent"},
		{"subscribe", "test.model"},
		{"accessDenied", "test.model"},
		{"errorResponse", "test.model"},
		{"event", "test.model.parent"},
		{"event", "test.model"},
	},
}
