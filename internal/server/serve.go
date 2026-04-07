package server

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/lmittmann/tint"
)

func Serve(tcp1Port string, tcp2Port string, udpPort string, logLevel string) {
	InitDB()
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

	wg.Add(3)

	go func() {
		defer wg.Done()
		StartSSLServer(tcp1Port)
	}()
	go func() {
		defer wg.Done()
		StartTCPServer(tcp2Port)
	}()
	go func() {
		defer wg.Done()
		StartUDPServer(udpPort)
	}()
	wg.Wait()
}
