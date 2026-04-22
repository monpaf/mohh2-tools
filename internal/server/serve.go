package server

import (
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/lmittmann/tint"
)

type server struct {
	mu      sync.RWMutex
	gpsUser string
	sslPort string
	tcpPort string
	udpPort string
	db      *memoryDB
}

func newServer(sslPort, tcpPort, udpPort string) *server {
	return &server{
		sslPort: sslPort,
		tcpPort: tcpPort,
		udpPort: udpPort,
		db:      newDB(),
	}
}

func Serve(sslPort, tcpPort, udpPort, logLevel string) {
	s := newServer(sslPort, tcpPort, udpPort)

	lvl := new(slog.LevelVar)
	logger := slog.New(
		tint.NewHandler(color.Output, &tint.Options{
			Level:      lvl,
			TimeFormat: time.Kitchen,
		}),
	)
	slog.SetDefault(logger)
	switch strings.ToUpper(logLevel) {
	case "DEBUG":
		lvl.Set(slog.LevelDebug)
	case "INFO":
		lvl.Set(slog.LevelInfo)
	case "WARN":
		lvl.Set(slog.LevelWarn)
	case "ERROR":
		lvl.Set(slog.LevelError)
	default:
		slog.Error(fmt.Sprintf("Invalid log level \"%s\", using default value \"INFO\"", logLevel))
		lvl.Set(slog.LevelInfo)
	}

	var wg sync.WaitGroup

	// wg.Add(3)
	wg.Add(2)

	go func() {
		defer wg.Done()
		s.StartSSLServer(sslPort)
	}()
	go func() {
		defer wg.Done()
		s.StartTCPServer(tcpPort)
	}()
	// go func() {
	// 	defer wg.Done()
	// 	s.StartUDPServer(udpPort)
	// }()
	wg.Wait()
}

func (s *server) handleConnection(conn net.Conn) {
	buffer := make([]byte, 1024)
	parser := NewAriesParser(conn, s)

	for {
		n, err := conn.Read(buffer)
		if err != nil {
			if err.Error() == "EOF" {
				slog.Info("TCP connection closed by client", "localAddr", conn.LocalAddr(), "remoteAddr", conn.RemoteAddr())
				return
			}
			slog.Error("Could not read from TCP connection", "localAddr", conn.LocalAddr(), "remoteAddr", conn.RemoteAddr(), "err", err)
			return
		}
		parser.parse(buffer[:n])
	}
}
