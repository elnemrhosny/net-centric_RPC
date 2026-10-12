package rpc

import (
	"encoding/json"
	"io"
	"net"
	"time"
)

func (s *Server) handle(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	var req Request
	if err := json.NewDecoder(io.LimitReader(conn, maxRequestBytes)).Decode(&req); err != nil {
		reply(conn, Response{Error: "bad request: " + err.Error()})
		return
	}
	if err := validateRequest(req); err != nil {
		reply(conn, Response{Error: err.Error()})
		return
	}
	result, err := s.invoke(req)
	if err != nil {
		reply(conn, Response{Error: err.Error()})
		return
	}
	resp := Response{OK: true}
	if result != nil {
		b, err := json.Marshal(result)
		if err != nil {
			reply(conn, Response{Error: "encode result: " + err.Error()})
			return
		}
		resp.Value = b
	}
	reply(conn, resp)
}

func reply(conn net.Conn, r Response) {
	_ = json.NewEncoder(conn).Encode(&r)
}
