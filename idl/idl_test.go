package idl

import "testing"

func TestParseUserIDL(t *testing.T) {
	spec, err := ParseFile("user.thrift")
	if err != nil {
		t.Fatal(err)
	}
	if spec.Service != "UserService" {
		t.Fatalf("service=%s", spec.Service)
	}
	m, ok := spec.Method("GetUser")
	if !ok || m.Req != "GetUserReq" || m.Resp != "GetUserResp" {
		t.Fatalf("method=%+v ok=%v", m, ok)
	}
	if m.URI != "" {
		t.Fatalf("user methods are RPC-only, got uri=%q", m.URI)
	}
	st, err := spec.Struct("GetUserResp")
	if err != nil {
		t.Fatal(err)
	}
	f, ok := st.FieldByID(2)
	if !ok || f.Name != "name" || f.Type != "string" {
		t.Fatalf("field=%+v", f)
	}
}

func TestParseOrderAGW(t *testing.T) {
	spec, err := ParseEmbedded("order.thrift")
	if err != nil {
		t.Fatal(err)
	}
	m, ok := spec.Method("GetOrder")
	if !ok || m.HTTPMethod != "POST" || m.URI != "/order/get" {
		t.Fatalf("GetOrder agw=%+v ok=%v", m, ok)
	}
	m, ok = spec.Method("CreateOrder")
	if !ok || m.URI != "/order/create" {
		t.Fatalf("CreateOrder agw=%+v ok=%v", m, ok)
	}
}

func TestParseToyMethodStillWorks(t *testing.T) {
	spec, err := ParseString(`
service S {
    Foo(FooReq) FooResp
}
struct FooReq {
    1: i64 id
}
struct FooResp {
    1: string name
}
`)
	if err != nil {
		t.Fatal(err)
	}
	m, ok := spec.Method("Foo")
	if !ok || m.Req != "FooReq" || m.Resp != "FooResp" {
		t.Fatalf("method=%+v ok=%v", m, ok)
	}
}

func TestParseThriftFieldExtras(t *testing.T) {
	spec, err := ParseString(`
namespace go demo
service S {
    FooResp Foo(1: FooReq req) (agw.method = 'POST', agw.uri = '/foo')
}
struct FooReq {
    1: required i64 id (agw.key = "id")
    2: optional string name = "" (agw.source = "header", agw.key = "X-Name")
}
struct FooResp {
    1: bool ok
}
`)
	if err != nil {
		t.Fatal(err)
	}
	m, _ := spec.Method("Foo")
	if m.HTTPMethod != "POST" || m.URI != "/foo" {
		t.Fatalf("agw=%+v", m)
	}
	st, _ := spec.Struct("FooReq")
	if len(st.Fields) != 2 || st.Fields[0].Name != "id" || st.Fields[1].Name != "name" {
		t.Fatalf("fields=%+v", st.Fields)
	}
}
