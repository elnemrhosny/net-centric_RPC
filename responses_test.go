package rpc

import (
	"encoding/json"
	"net"
	"testing"
	"time"
)

// sendRaw dials the server, writes a raw JSON request, and returns the decoded response.
func sendRaw(t *testing.T, addr, raw string) Response {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(time.Second))

	if _, err := conn.Write([]byte(raw)); err != nil {
		t.Fatalf("write: %v", err)
	}

	var resp Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return resp
}

func TestAllResponses(t *testing.T) {
	srv := &Server{service: &Service{}}
	if err := srv.Start("127.0.0.1:0"); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer srv.Stop()
	addr := srv.Address()

	cases := []struct {
		name     string
		raw      string
		wantOK   bool
		wantVal  string // expected JSON value (as string), if wantOK
		wantErrs string // substring expected in error, if !wantOK
	}{
		// ---- successful calls ----
		{
			name:    "NoArgs",
			raw:     `{"method":"NoArgs","args":[]}`,
			wantOK:  true,
			wantVal: `"ok"`,
		},
		{
			name:    "Add 2+3",
			raw:     `{"method":"Add","args":[2,3]}`,
			wantOK:  true,
			wantVal: `5`,
		},
		{
			name:    "Add negative",
			raw:     `{"method":"Add","args":[-10,4]}`,
			wantOK:  true,
			wantVal: `-6`,
		},
		{
			name:    "Echo hello",
			raw:     `{"method":"Echo","args":["hello"]}`,
			wantOK:  true,
			wantVal: `"hello"`,
		},
		{
			name:    "OnlyError false",
			raw:     `{"method":"OnlyError","args":[false]}`,
			wantOK:  true,
			wantVal: ``, // no value returned
		},
		{
			name:    "Ping 7",
			raw:     `{"method":"Ping","args":[7]}`,
			wantOK:  true,
			wantVal: `"pong 7"`,
		},
		{
			name:    "Slow",
			raw:     `{"method":"Slow","args":[]}`,
			wantOK:  true,
			wantVal: `"ok"`,
		},

		// ---- method returns error ----
		{
			name:     "OnlyError true",
			raw:      `{"method":"OnlyError","args":[true]}`,
			wantOK:   false,
			wantErrs: "requested failure",
		},
		{
			name:     "Ping negative",
			raw:      `{"method":"Ping","args":[-1]}`,
			wantOK:   false,
			wantErrs: "invalid id",
		},

		// ---- method panics ----
		{
			name:     "Panic",
			raw:      `{"method":"Panic","args":[]}`,
			wantOK:   false,
			wantErrs: "panic",
		},

		// ---- unknown method ----
		{
			name:     "Unknown method",
			raw:      `{"method":"Missing","args":[]}`,
			wantOK:   false,
			wantErrs: `unknown method "Missing"`,
		},

		// ---- wrong arg count ----
		{
			name:     "Add too few args",
			raw:      `{"method":"Add","args":[1]}`,
			wantOK:   false,
			wantErrs: "expects 2 args",
		},
		{
			name:     "Add too many args",
			raw:      `{"method":"Add","args":[1,2,3]}`,
			wantOK:   false,
			wantErrs: "expects 2 args",
		},
		{
			name:     "NoArgs with args",
			raw:      `{"method":"NoArgs","args":[1]}`,
			wantOK:   false,
			wantErrs: "expects 0 args",
		},

		// ---- wrong JSON type ----
		{
			name:     "Add string arg",
			raw:      `{"method":"Add","args":["two",3]}`,
			wantOK:   false,
			wantErrs: "cannot decode",
		},
		{
			name:     "Echo int arg",
			raw:      `{"method":"Echo","args":[42]}`,
			wantOK:   false,
			wantErrs: "cannot decode",
		},

		// ---- invalid method name ----
		{
			name:     "Empty method",
			raw:      `{"method":"","args":[]}`,
			wantOK:   false,
			wantErrs: "method",
		},
		{
			name:     "Whitespace in method",
			raw:      `{"method":"Ad d","args":[1,2]}`,
			wantOK:   false,
			wantErrs: "method",
		},
		{
			name:     "Control char in method",
			raw:      `{"method":"Add\u0000","args":[1,2]}`,
			wantOK:   false,
			wantErrs: "method",
		},

		// ---- malformed JSON ----
		{
			name:     "Malformed JSON",
			raw:      `{not json`,
			wantOK:   false,
			wantErrs: "malformed",
		},

		// ---- oversized input ----
		{
			name:     "Oversized single arg",
			raw:      `{"method":"Echo","args":["` + string(make([]byte, 100_000)) + `"]}`,
			wantOK:   false,
			wantErrs: "Oversized argument",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := sendRaw(t, addr, tc.raw)

			// Always print what came back for visibility.
			t.Logf("in : %s", truncate(tc.raw, 120))
			t.Logf("out: ok=%v value=%s error=%q",
				resp.OK, string(resp.Value), resp.Error)

			if resp.OK != tc.wantOK {
				t.Fatalf("ok=%v, want %v (error=%q)",
					resp.OK, tc.wantOK, resp.Error)
			}
			if tc.wantOK {
				if tc.wantVal != "" && string(resp.Value) != tc.wantVal {
					t.Fatalf("value=%s, want %s", resp.Value, tc.wantVal)
				}
				if tc.wantVal == "" && len(resp.Value) > 0 {
					t.Fatalf("value=%s, want empty", resp.Value)
				}
			} else if tc.wantErrs != "" {
				if !contains(resp.Error, tc.wantErrs) {
					t.Fatalf("error=%q, want it to contain %q", resp.Error, tc.wantErrs)
				}
			}
		})
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
