package server

import (
	"encoding/binary"
	"encoding/hex"
	"log/slog"
	"net"
	"os"

	"github.com/fatih/color"
)

func (s *server) StartUDPServer(port string) {
	ln, err := net.ListenPacket("udp", ":"+port)
	if err != nil {
		slog.Error("Could not start UDP server", "port", port, "err", err)
		os.Exit(1)
	}

	slog.Info("UDP server listening", "addr", ln.LocalAddr())

	// randomBytes := make([]byte, 19)

	defer ln.Close()
	buffer := make([]byte, 1024)
	for {
		n, addr, err := ln.ReadFrom(buffer)
		if err != nil {
			slog.Error("Could not read from UDP connection", "addr", addr, "err", err)
			continue
		}
		bytes := buffer[:n]
		slog.Debug(color.MagentaString("UDP <--\n") + color.CyanString(hex.Dump(bytes)))

		if binary.BigEndian.Uint32(bytes) == 5 {
			bytes = confirmConnection(bytes)
		} else if len(bytes) == 8 && binary.BigEndian.Uint32(bytes) == 0x100 && binary.BigEndian.Uint32(bytes[4:8]) == 0xff {
			binary.BigEndian.PutUint32(bytes[4:], 0x100)
		} else if len(bytes) > 8 && binary.BigEndian.Uint32(bytes) >= 0x80 && binary.BigEndian.Uint32(bytes) <= 0x10000 && bytes[len(bytes)-1] == 0x40 {
			ident := binary.BigEndian.Uint32(bytes)
			ack := binary.BigEndian.Uint32(bytes[4:8])
			syncRepl := binary.BigEndian.Uint32(bytes[len(bytes)-7 : len(bytes)-3])
			response := make([]byte, 19)

			binary.BigEndian.PutUint32(response, ident)
			binary.BigEndian.PutUint32(response[4:], ack)
			binary.BigEndian.PutUint32(response[8:], syncRepl)
			binary.BigEndian.PutUint32(response[12:], syncRepl)
			binary.BigEndian.PutUint16(response[16:], 0)
			response[18] = 0x40
			bytes = response
		} else {
			// if bytes[len(bytes)-1] == 7 || bytes[len(bytes)-1] == 0x47 {
			// 	binary.BigEndian.PutUint64(randomBytes, binary.BigEndian.Uint64(bytes[16:]))
			// 	if bytes[24] == 0xaa {
			// 		for i := 8; i < 16; i += 4 {
			// 			binary.BigEndian.PutUint32(randomBytes[i:], uint32(rand.Int31()))
			// 		}
			// 	}
			// 	binary.BigEndian.PutUint16(randomBytes[16:], binary.BigEndian.Uint16(bytes[32:]))
			// 	randomBytes[18] = bytes[34]
			// 	binary.LittleEndian.PutUint32(bytes[12:], crc32.ChecksumIEEE(randomBytes))
			// 	copy(bytes[16:], randomBytes)
			// }
			// if bytes[len(bytes)-1] == 7 || bytes[len(bytes)-1] == 0x47 {
			// 	disconnect := []byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x15, 0xf8, 0x86, 0x83, 0x28, 0xde, 0x33, 0x6a, 0x01, 0x00, 0x00}
			// 	binary.BigEndian.PutUint64(disconnect, binary.BigEndian.Uint64(bytes[16:]))
			// 	binary.LittleEndian.PutUint32(bytes[12:], crc32.ChecksumIEEE(disconnect))
			// 	copy(bytes[16:], disconnect)
			// }
			// if bytes[len(bytes)-1] == 7 || bytes[len(bytes)-1] == 0x47 {
			// 	size := 17
			// 	size2 := 1
			// 	if bytes[len(bytes)-1] == 0x47 {
			// 		size += 10
			// 		size2 += 10
			// 	}
			// 	seven := make([]byte, len(bytes)-size)
			// 	binary.BigEndian.PutUint64(seven, binary.BigEndian.Uint64(bytes[16:]))
			// 	for i := 8; i < len(bytes)-size-4; i += 4 {
			// 		binary.BigEndian.PutUint32(seven[i:], uint32(rand.Int31()))
			// 	}

			// 	binary.BigEndian.PutUint16(seven[len(seven)-3:], binary.BigEndian.Uint16(bytes[len(bytes)-size2-3:]))
			// 	seven[len(seven)-1] = bytes[len(bytes)-size2-1]
			// 	binary.LittleEndian.PutUint32(bytes[12:], crc32.ChecksumIEEE(seven))
			// 	copy(bytes[16:], seven)
			// }
		}
		// if bytes[len(bytes)-1] != 7 {
		_, err = ln.WriteTo(bytes, addr)
		if err != nil {
			slog.Error("Could not write to UDP connection", "addr", addr, "err", err)
		}
		slog.Debug(color.MagentaString("UDP -->\n") + color.YellowString(hex.Dump(bytes)))
		// }
	}
}

func confirmConnection(packet []byte) []byte {
	connIdent := binary.BigEndian.Uint32(packet[4:8])
	bytes := make([]byte, 8)
	binary.BigEndian.PutUint32(bytes, 2)
	binary.BigEndian.PutUint32(bytes[4:], connIdent)
	return bytes
}
