package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
)

type udpProxySession struct {
	client   *net.UDPAddr
	upstream *net.UDPConn
}

const mohzUHSUDPPortOffset = 30

func runUHSProxy(ctx context.Context, listen, target string) error {
	listenAddr, err := uhsProxyListenAddress(listen)
	if err != nil {
		return fmt.Errorf("listen address: %w", err)
	}
	mohzAddr, targetAddr, err := uhsProxyTargetAddress(target)
	if err != nil {
		return fmt.Errorf("target address: %w", err)
	}

	conn, err := net.ListenUDP("udp4", listenAddr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer conn.Close()

	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	slog.Info("UHS proxy listening", "addr", conn.LocalAddr(), "mohzAddr", mohzAddr, "target", targetAddr)

	buffer := make([]byte, 65535)
	sessions := make(map[string]*udpProxySession)

	for {
		n, addr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("read: %w", err)
		}

		session, ok := sessions[addr.String()]
		if !ok {
			session, err = newUDPProxySession(ctx, conn, cloneUDPAddr(addr), targetAddr)
			if err != nil {
				return err
			}
			sessions[addr.String()] = session
		}

		if _, err := session.upstream.Write(buffer[:n]); err != nil {
			return fmt.Errorf("write to target: %w", err)
		}
	}
}

func newUDPProxySession(ctx context.Context, downstream *net.UDPConn, client, target *net.UDPAddr) (*udpProxySession, error) {
	upstream, err := net.DialUDP("udp4", nil, target)
	if err != nil {
		return nil, fmt.Errorf("connect to target: %w", err)
	}

	session := &udpProxySession{
		client:   client,
		upstream: upstream,
	}

	go func() {
		<-ctx.Done()
		_ = upstream.Close()
	}()

	go func() {
		defer upstream.Close()

		buffer := make([]byte, 65535)
		for {
			n, err := upstream.Read(buffer)
			if err != nil {
				return
			}
			if _, err := downstream.WriteToUDP(buffer[:n], client); err != nil {
				return
			}
		}
	}()

	return session, nil
}

func uhsProxyListenAddress(value string) (*net.UDPAddr, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, errors.New("address is empty")
	}
	if !strings.Contains(value, ":") {
		value = ":" + value
	}
	return net.ResolveUDPAddr("udp4", value)
}

func uhsProxyTargetAddress(value string) (*net.UDPAddr, *net.UDPAddr, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil, errors.New("address is empty")
	}
	if !strings.Contains(value, ":") {
		value = "127.0.0.1:" + value
	}
	mohzAddr, err := net.ResolveUDPAddr("udp4", value)
	if err != nil {
		return nil, nil, err
	}
	if mohzAddr.Port+mohzUHSUDPPortOffset > 65535 {
		return nil, nil, fmt.Errorf("mohz port %d is too high for UHS UDP offset %d", mohzAddr.Port, mohzUHSUDPPortOffset)
	}

	targetAddr := cloneUDPAddr(mohzAddr)
	targetAddr.Port += mohzUHSUDPPortOffset

	return mohzAddr, targetAddr, nil
}

func cloneUDPAddr(addr *net.UDPAddr) *net.UDPAddr {
	if addr == nil {
		return nil
	}
	return &net.UDPAddr{
		IP:   append(net.IP(nil), addr.IP...),
		Port: addr.Port,
		Zone: addr.Zone,
	}
}
