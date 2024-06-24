package test

import (
	"encoding/json"

	"github.com/resgateio/resgate/server/reserr"
)

type mockData struct {
	UnsubscribeReasonAccessDenied json.RawMessage
	UnsubscribeReasonDeleted      json.RawMessage
}

var mock = mockData{
	UnsubscribeReasonAccessDenied: json.RawMessage(`{"reason":{"code":"system.accessDenied","message":"Access denied"}}`),
	UnsubscribeReasonDeleted:      json.RawMessage(`{"reason":{"code":"system.deleted","message":"Deleted"}}`),
}

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
	rsrc, ok := resources[rid]
	if !ok {
		panic("no resource named " + rid)
	}
	if rsrc.typ == typeError {
		b, _ := json.Marshal(rsrc.err)
		return string(b)
	}
	return rsrc.data
}

func generateString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'a'
	}
	return string(b)
}

var resources = map[string]resource{
	// Model resources
	"test.model":              {typeModel, `{"string":"foo","int":42,"bool":true,"null":null}`, nil},
	"test.model.parent":       {typeModel, `{"name":"parent","child":{"rid":"test.model"}}`, nil},
	"test.model.secondparent": {typeModel, `{"name":"secondparent","child":{"rid":"test.model"}}`, nil},
	"test.model.grandparent":  {typeModel, `{"name":"grandparent","child":{"rid":"test.model.parent"}}`, nil},
	"test.model.brokenchild":  {typeModel, `{"name":"brokenchild","child":{"rid":"test.err.notFound"}}`, nil},
	"test.model.soft":         {typeModel, `{"name":"soft","child":{"rid":"test.model","soft":true}}`, nil},
	"test.model.soft.parent":  {typeModel, `{"name":"softparent","child":{"rid":"test.model.soft","soft":false}}`, nil},
	"test.model.data":         {typeModel, `{"name":"data","primitive":{"data":12},"object":{"data":{"foo":["bar"]}},"array":{"data":[{"foo":"bar"}]}}`, nil},
	"test.model.data.parent":  {typeModel, `{"name":"dataparent","child":{"rid":"test.model.data"}}`, nil},
	"test.model.query.parent": {typeModel, `{"name":"queryparent","child":{"rid":"test.model?foo=bar"}}`, nil},

	// Cyclic model resources
	"test.m.a": {typeModel, `{"a":{"rid":"test.m.a"}}`, nil},

	"test.m.b": {typeModel, `{"c":{"rid":"test.m.c"}}`, nil},
	"test.m.c": {typeModel, `{"b":{"rid":"test.m.b"}}`, nil},

	"test.m.d": {typeModel, `{"e":{"rid":"test.m.e"},"f":{"rid":"test.m.f"}}`, nil},
	"test.m.e": {typeModel, `{"d":{"rid":"test.m.d"}}`, nil},
	"test.m.f": {typeModel, `{"d":{"rid":"test.m.d"}}`, nil},

	"test.m.g": {typeModel, `{"e":{"rid":"test.m.e"},"f":{"rid":"test.m.f"}}`, nil},
	"test.m.h": {typeModel, `{"e":{"rid":"test.m.e"}}`, nil},

	// Collection resources
	"test.collection":              {typeCollection, `["foo",42,true,null]`, nil},
	"test.collection.parent":       {typeCollection, `["parent",{"rid":"test.collection"}]`, nil},
	"test.collection.secondparent": {typeCollection, `["secondparent",{"rid":"test.collection"}]`, nil},
	"test.collection.grandparent":  {typeCollection, `["grandparent",{"rid":"test.collection.parent"}]`, nil},
	"test.collection.brokenchild":  {typeCollection, `["brokenchild",{"rid":"test.err.notFound"}]`, nil},
	"test.collection.soft":         {typeCollection, `["soft",{"rid":"test.collection","soft":true}]`, nil},
	"test.collection.soft.parent":  {typeCollection, `["softparent",{"rid":"test.collection.soft","soft":false}]`, nil},
	"test.collection.data":         {typeCollection, `["data",{"data":12},{"data":{"foo":["bar"]}},{"data":[{"foo":"bar"}]}]`, nil},
	"test.collection.data.parent":  {typeCollection, `["dataparent",{"rid":"test.collection.data"}]`, nil},
	"test.collection.empty":        {typeCollection, `[]`, nil},

	// Cyclic collection resources
	"test.c.a": {typeCollection, `[{"rid":"test.c.a"}]`, nil},

	"test.c.b": {typeCollection, `[{"rid":"test.c.c"}]`, nil},
	"test.c.c": {typeCollection, `[{"rid":"test.c.b"}]`, nil},

	"test.c.d": {typeCollection, `[{"rid":"test.c.e"},{"rid":"test.c.f"}]`, nil},
	"test.c.e": {typeCollection, `[{"rid":"test.c.d"}]`, nil},
	"test.c.f": {typeCollection, `[{"rid":"test.c.d"}]`, nil},

	"test.c.g": {typeCollection, `[{"rid":"test.c.e"},{"rid":"test.c.f"}]`, nil},
	"test.c.h": {typeCollection, `[{"rid":"test.c.e"}]`, nil},

	// Errors
	"test.err.notFound":      {typeError, "", reserr.ErrNotFound},
	"test.err.internalError": {typeError, "", reserr.ErrInternalError},
	"test.err.timeout":       {typeError, "", reserr.ErrTimeout},
}

// Call responses
const (
	requestTimeout uint64 = iota
	noRequest
	noToken
)

type sequenceEvent struct {
	Event string
	RID   string
}

type sequenceSet struct {
	Version string
	Table   [][]sequenceEvent
}

var sequenceSets = []sequenceSet{
	{
		versionLatest,
		[][]sequenceEvent{
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
			{
				{"subscribe", "test.model.soft"},
				{"access", "test.model.soft"},
				{"get", "test.model.soft"},
				{"response", "test.model.soft"},
				{"event", "test.model.soft"},
				{"nosubscription", "test.model"},
			},
			{
				{"subscribe", "test.model.soft.parent"},
				{"access", "test.model.soft.parent"},
				{"get", "test.model.soft.parent"},
				{"get", "test.model.soft"},
				{"response", "test.model.soft.parent"},
				{"event", "test.model.soft.parent"},
				{"event", "test.model.soft"},
				{"nosubscription", "test.model"},
			},
			{
				{"subscribe", "test.model.data"},
				{"access", "test.model.data"},
				{"get", "test.model.data"},
				{"response", "test.model.data"},
				{"event", "test.model.data"},
			},
			{
				{"subscribe", "test.model.data.parent"},
				{"access", "test.model.data.parent"},
				{"get", "test.model.data.parent"},
				{"get", "test.model.data"},
				{"response", "test.model.data.parent"},
				{"event", "test.model.data.parent"},
				{"event", "test.model.data"},
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
			{
				{"subscribe", "test.collection.soft"},
				{"access", "test.collection.soft"},
				{"get", "test.collection.soft"},
				{"response", "test.collection.soft"},
				{"event", "test.collection.soft"},
				{"nosubscription", "test.collection"},
			},
			{
				{"subscribe", "test.collection.soft.parent"},
				{"access", "test.collection.soft.parent"},
				{"get", "test.collection.soft.parent"},
				{"get", "test.collection.soft"},
				{"response", "test.collection.soft.parent"},
				{"event", "test.collection.soft.parent"},
				{"event", "test.collection.soft"},
				{"nosubscription", "test.collection"},
			},
			{
				{"subscribe", "test.collection.data"},
				{"access", "test.collection.data"},
				{"get", "test.collection.data"},
				{"response", "test.collection.data"},
				{"event", "test.collection.data"},
				{"nosubscription", "test.collection"},
			},
			{
				{"subscribe", "test.collection.data.parent"},
				{"access", "test.collection.data.parent"},
				{"get", "test.collection.data.parent"},
				{"get", "test.collection.data"},
				{"response", "test.collection.data.parent"},
				{"event", "test.collection.data.parent"},
				{"event", "test.collection.data"},
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
		},
	},
	{
		"1.2.0",
		[][]sequenceEvent{
			// Model tests
			{
				{"subscribe", "test.model.soft"},
				{"access", "test.model.soft"},
				{"get", "test.model.soft"},
				{"response", "test.model.soft"},
				{"event", "test.model.soft"},
				{"nosubscription", "test.model"},
			},
			{
				{"subscribe", "test.model.soft.parent"},
				{"access", "test.model.soft.parent"},
				{"get", "test.model.soft.parent"},
				{"get", "test.model.soft"},
				{"response", "test.model.soft.parent"},
				{"event", "test.model.soft.parent"},
				{"event", "test.model.soft"},
			},
			{
				{"subscribe", "test.model.data"},
				{"access", "test.model.data"},
				{"get", "test.model.data"},
				{"response", "test.model.data"},
				{"event", "test.model.data"},
				{"nosubscription", "test.model"},
			},
			{
				{"subscribe", "test.model.data.parent"},
				{"access", "test.model.data.parent"},
				{"get", "test.model.data.parent"},
				{"get", "test.model.data"},
				{"response", "test.model.data.parent"},
				{"event", "test.model.data.parent"},
				{"event", "test.model.data"},
			},
			// Collection tests
			{
				{"subscribe", "test.collection.soft"},
				{"access", "test.collection.soft"},
				{"get", "test.collection.soft"},
				{"response", "test.collection.soft"},
				{"event", "test.collection.soft"},
				{"nosubscription", "test.collection"},
			},
			{
				{"subscribe", "test.collection.soft.parent"},
				{"access", "test.collection.soft.parent"},
				{"get", "test.collection.soft.parent"},
				{"get", "test.collection.soft"},
				{"response", "test.collection.soft.parent"},
				{"event", "test.collection.soft.parent"},
				{"event", "test.collection.soft"},
			},
			{
				{"subscribe", "test.collection.data"},
				{"access", "test.collection.data"},
				{"get", "test.collection.data"},
				{"response", "test.collection.data"},
				{"event", "test.collection.data"},
				{"nosubscription", "test.collection"},
			},
			{
				{"subscribe", "test.collection.data.parent"},
				{"access", "test.collection.data.parent"},
				{"get", "test.collection.data.parent"},
				{"get", "test.collection.data"},
				{"response", "test.collection.data.parent"},
				{"event", "test.collection.data.parent"},
				{"event", "test.collection.data"},
			},
		},
	},
}
