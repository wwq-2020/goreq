package goreq

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDo(t *testing.T) {
	type req struct {
		A string `json:"a"`
	}
	type resp struct {
		B string `json:"a"`
	}
	givenReq := &req{A: "a"}
	givenResp := &resp{B: "b"}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReq := &req{}
		if err := json.NewDecoder(r.Body).Decode(gotReq); err != nil {
			t.Fatal("unexpected err")
		}
		if gotReq.A != givenReq.A {
			t.Fatal("unexpected gotreq")
		}
		if err := json.NewEncoder(w).Encode(givenResp); err != nil {
			t.Fatal("unexpected err")
		}
		time.Sleep(time.Second * 3)
	}))
	defer s.Close()
	gotResp := &resp{}
	err := New().URL(s.URL).Method(http.MethodPost).Req(givenReq).Resp(gotResp).
		WrapTransport(LoggingTransport("demo"), TraceTransport("demo"), TimeoutTransport(time.Second)).
		Do(context.TODO())
	if err != nil {
		t.Fatalf("unexpected err:%#v", err)
	}
	if gotResp.B != givenResp.B {
		t.Fatal("unexpected gotResp")
	}
}
