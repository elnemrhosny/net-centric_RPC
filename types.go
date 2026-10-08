package rpc

import "encoding/json"

// Request is the JSON body a client sends to the server.
type Request struct {
	Method string            `json:"method"`
	Args   []json.RawMessage `json:"args"`
}

// Response is the JSON body the server sends back to the client.
type Response struct {
	OK    bool            `json:"ok"`
	Value json.RawMessage `json:"value,omitempty"`
	Error string          `json:"error,omitempty"`
}

// Protocol limits used by validateRequest.
const (
	maxArguments     = 16
	maxArgumentBytes = 1024
	maxRequestBytes  = 4096
)
