# Reflection-Based RPC in Go

A small RPC system over TCP with JSON messages. The server uses Go's `reflect` package to find and call methods on a `Service` type by name, so adding a new remote method only requires adding a method to `Service`.

## Project Structure

| File | Purpose |
|---|---|
| `types.go` | `Request` and `Response` structs, protocol limits |
| `server.go` | `Server` lifecycle: `Start`, `Stop`, `Address`, accept loop |
| `handle.go` | Per-connection handler: decode, validate, invoke, reply |
| `invoke.go` | Reflection: method lookup, argument decoding, call, result handling |
| `validate.go` | Request validation (method name, argument count and size) |
| `service.go` | The remote methods exposed by the server |
| `client.go` | `Client` with `Call` |
| `*_test.go` | Unit and end-to-end tests |

## Protocol

One request and one response per TCP connection, each a single JSON object.

Request:

```json
{"method": "Add", "args": [2, 3]}
```

Response on success:

```json
{"ok": true, "value": 5}
```

Response on failure:

```json
{"ok": false, "error": "unknown method \"Missing\""}
```

### Limits

| Limit | Value |
|---|---|
| Max arguments | 16 |
| Max size of one argument | 1024 bytes |
| Max request size | 4096 bytes |
| Method name | non-empty, no whitespace or control characters |

Requests that break a limit get an error response.

## Exposed Methods

| Method | Arguments | Returns |
|---|---|---|
| `NoArgs` | none | `"ok"` |
| `Add` | `int, int` | sum |
| `Echo` | `string` | the same string |
| `OnlyError` | `bool` | nothing; error if `true` |
| `Ping` | `int` | `"pong <id>"`; error if `id < 0` |

## Usage

```go
package main

import (
	"fmt"
	"time"

	rpc "RPC"
)

func main() {
	s := &rpc.Server{}
	if err := s.Start("127.0.0.1:0"); err != nil { // port 0 = any free port
		panic(err)
	}
	defer s.Stop()

	c := rpc.NewClient(s.Address(), 2*time.Second)

	var sum int
	if err := c.Call("Add", &sum, 2, 3); err != nil {
		panic(err)
	}
	fmt.Println(sum) // 5
}
```

`Call(method, out, args...)` decodes the result into `out`. Pass `nil` for `out` when you don't need the result.

## Adding a Remote Method

Add an exported method to `Service` in `service.go`:

```go
func (s *Service) Multiply(a, b int) int { return a * b }
```

Supported return shapes: no value, one value, `error`, or `(value, error)`. Arguments must be JSON-decodable.

## Design Notes

- **Concurrency:** each connection is handled in its own goroutine. A `sync.WaitGroup` tracks the accept loop and all handlers, so `Stop` waits for in-flight requests before returning.
- **Safe lifecycle:** `Start` rejects a second call while running, `Stop` is idempotent, and the server can be restarted.
- **Panic safety:** `invoke` recovers from panics in service methods and returns them as error responses instead of crashing the server.
- **Timeouts:** the client sets a connection and read deadline (default 5s); the server sets a deadline on every connection.
- **Bounded input:** the server reads at most `maxRequestBytes` per connection.

## Testing

```bash
go test ./... -race -count=1
```

The tests cover:

- reflection dispatch (`invoke_test.go`)
- request validation (`server_test.go`)
- server start/stop lifecycle (`lifecycle_test.go`)
- end-to-end client and server calls, error cases, limits, concurrency, timeouts, and restart (`client_test.go`)