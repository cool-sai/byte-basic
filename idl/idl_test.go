package idl

import "testing"

func TestParseUserIDL(t *testing.T) {
	spec, err := ParseFile("user.idl")
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
	st, err := spec.Struct("GetUserResp")
	if err != nil {
		t.Fatal(err)
	}
	f, ok := st.FieldByID(2)
	if !ok || f.Name != "name" || f.Type != "string" {
		t.Fatalf("field=%+v", f)
	}
}
