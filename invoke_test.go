package rpc

import (
	"encoding/json"
	"testing"
)

func TestInvoke(t *testing.T) {
	s := &Server{}
	args := func(v ...string) []json.RawMessage {
		r := make([]json.RawMessage, len(v))
		for i, x := range v {
			r[i] = json.RawMessage(x)
		}
		return r
	}
	tests := []struct {
		name    string
		req     Request
		want    any
		wantErr bool
	}{
		{"no args", Request{"NoArgs", args()}, "ok", false},
		{"add", Request{"Add", args("2", "3")}, 5, false},
		{"echo", Request{"Echo", args(`"hi"`)}, "hi", false},
		{"error only ok", Request{"OnlyError", args("false")}, nil, false},
		{"error only fails", Request{"OnlyError", args("true")}, nil, true},
		{"ping ok", Request{"Ping", args("7")}, "pong 7", false},
		{"ping error", Request{"Ping", args("-1")}, nil, true},
		{"unknown method", Request{"Missing", args()}, nil, true},
		{"wrong arg count", Request{"Add", args("1")}, nil, true},
		{"wrong json type", Request{"Add", args(`"x"`, "2")}, nil, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := s.invoke(tc.req)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
