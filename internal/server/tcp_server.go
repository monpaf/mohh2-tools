package server

import (
	"log/slog"
	"net"
	"os"
)

func (s *server) StartTCPServer(port string) {
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		slog.Error("Could not start TCP server", "port", port, "err", err)
		os.Exit(1)
	}

	slog.Info("TCP server listening", "addr", ln.Addr())

	for {
		conn, err := ln.Accept()
		if err != nil {
			slog.Error("Could not accept TCP connection", "addr", ln.Addr(), "err", err)
			continue
		}

		slog.Info("TCP connection accepted", "localAddr", ln.Addr(), "remoteAddr", conn.RemoteAddr())

		go s.handleConnection(conn)
	}
}
