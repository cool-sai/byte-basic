package generic

import (
	"bytes"
	"testing"

	"minikitex/idl"
	"minikitex/wire"
)

func TestMapJSONBinaryRoundTrip(t *testing.T) {
	spec, err := idl.ParseFile("../idl/user.thrift")
	if err != nil {
		t.Fatal(err)
	}

	got, err := Encode(spec, "GetUserReq", map[string]any{"id": 1})
	if err != nil {
		t.Fatal(err)
	}
	fromJSON, err := Encode(spec, "GetUserReq", `{"id": 1}`)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, fromJSON) {
		t.Fatalf("map vs json:\n%x\n%x", got, fromJSON)
	}

	w := wire.NewWriter()
	w.FieldI64(1, 1)
	w.Stop()
	if !bytes.Equal(got, w.Bytes()) {
		t.Fatalf("generic vs hand-written wire:\n%x\n%x", got, w.Bytes())
	}

	m, err := Decode(spec, "GetUserReq", got)
	if err != nil {
		t.Fatal(err)
	}
	if m["id"].(int64) != 1 {
		t.Fatalf("decoded=%v", m)
	}

	passthrough, err := Encode(spec, "GetUserReq", got)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(passthrough, got) {
		t.Fatal("binary generic must pass bytes through")
	}
}
