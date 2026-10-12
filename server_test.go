package rpc

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     Request
		wantErr bool
	}{
		{
			name: "valid method with no arguments",
			req: Request{
				Method: "NoArgs",
				Args:   nil,
			},
			wantErr: false,
		},
		{
			name: "valid method with arguments",
			req: Request{
				Method: "Add",
				Args: []json.RawMessage{
					json.RawMessage(`2`),
					json.RawMessage(`3`),
				},
			},
			wantErr: false,
		},
		{
			name: "empty method",
			req: Request{
				Method: "",
			},
			wantErr: true,
		},
		{
			name: "whitespace method",
			req: Request{
				Method: "Add Method",
			},
			wantErr: true,
		},
		{
			name: "control character in method",
			req: Request{
				Method: "Add\n",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRequest(tt.req)

			if tt.wantErr && err == nil {
				t.Fatal("expected an error, got nil")
			}

			if !tt.wantErr && err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
		})
	}
}

func TestValidateRequestErrorMessage(t *testing.T) {
	req := Request{Method: "Add Method"}

	err := validateRequest(req)
	if err == nil {
		t.Fatal("expected validation error")
	}

	if !strings.Contains(strings.ToLower(err.Error()), "method") {
		t.Fatalf("expected an error mentioning the method, got: %v", err)
	}
}
