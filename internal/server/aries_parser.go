package server

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"

	"github.com/fatih/color"
)

type AriesParser struct {
	conn          net.Conn
	messageBuffer []byte
}

func NewAriesParser(conn net.Conn) *AriesParser {
	return &AriesParser{
		conn:          conn,
		messageBuffer: make([]byte, 0),
	}
}

func (s *AriesParser) parse(data []byte) {
	s.messageBuffer = append(s.messageBuffer, data...)
	for len(s.messageBuffer) >= 12 {
		messageSize := binary.BigEndian.Uint32(s.messageBuffer[8:12])
		if uint32(len(s.messageBuffer)) >= messageSize {
			message := s.messageBuffer[:messageSize]
			s.messageBuffer = s.messageBuffer[messageSize:]
			s.parseMessage(message)
		} else {
			break
		}
	}
}

func parseKeyValueString(input string) map[string]string {
	result := make(map[string]string)
	lines := strings.SplitSeq(input, "\n")
	for line := range lines {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			result[key] = value
		}
	}
	return result
}

func (s *AriesParser) parseMessage(message []byte) {
	kind := string(message[0:4])
	// packetSize := binary.BigEndian.Uint32(message[8:12])
	// content := string(message[12:packetSize])
	slog.Debug(color.MagentaString("TCP <--\n") + color.CyanString(hex.Dump(message)))

	switch kind {
	case "~png":
	case "@tic":
	case "addr":
	case "@dir":
		s.dirResponse(kind)
	case "sele":
		s.seleResponse(kind)
	case "gpsc":
		packetSize := binary.BigEndian.Uint32(message[8:12])
		content := string(message[12:packetSize])

		s.sendSes(parseKeyValueString(content))
	case "llvl":
		s.llvlResponse(kind)
	case "glea":
	case "gdel":
	case "cper":
	case "llvs":
		s.dummyResponse(kind)
	case "gsea":
		s.gseaResponse(kind)
	case "pers":
		s.persResponse(kind)
		s.sendWho()
	case "skey":
		s.skeyResponse(kind)
	case "news":
		s.newsResponse(kind)
	case "auth":
		s.authResponse(kind)
	default:
		slog.Warn("Unknown packet type", "kind", kind)
	}
}

func (s *AriesParser) gseaResponse(kind string) {
	content := map[string]string{
		"COUNT": "0",
	}
	s.send(kind, content)
}

func (s *AriesParser) sendSes(receivedContent map[string]string) {
	content := map[string]string{
		"IDENT":   "12",
		"NAME":    "Player",
		"HOST":    "Player",
		"GPSHOST": "Player",
		// "PARAMS":  "8,12d,,,-1,,,1e,,-1,1,1,1,1,1,1,1,1,20,,,15f90,122d0022",
		"PARAMS": receivedContent["PARAMS"],
		// ["PLATPARAMS": "0",  // ???
		"ROOM":      "13",
		"CUSTFLAGS": "0",
		"SYSFLAGS":  "262656",
		"COUNT":     "1",
		"PRIV":      "0",
		"MINSIZE":   "0",
		"MAXSIZE":   "33",
		"NUMPART":   "1",
		"SEED":      "012345", // random seed
		"WHEN":      "2009.2.8-9:44:15",
		"GAMEPORT":  "21172",
		"VOIPPORT":  "21172",
		// ["GAMEMODE": "0", // ???
		// ["AUTH": "0", // ???

		// loop 0x80022058 only if COUNT>=0
		"OPID0":  "0",             // OPID%d
		"OPPO0":  "Player",        // OPPO%d
		"ADDR0":  "127.0.0.1",     // ADDR%d
		"LADDR0": "127.0.0.1",     // LADDR%d
		"MADDR0": "$0017ab8f4451", // MADDR%d
		// ["OPPART0": "0", // OPPART%d
		"OPPARAM0": "AAAAAAAAAAAAAAAAAAAAAQBuDCgAAAAC", // OPPARAM%d
		// ["OPFLAGS0": "0", // OPFLAGS%d
		// ["PRES0": "0", // PRES%d ???

		// another loop 0x8002225C only if NUMPART>=0
		"PARTSIZE0":   "16", // PARTSIZE%d
		"PARTPARAMS0": "0",  // PARTPARAMS%d
		// "SESS": "0", %s-%s-%08x 0--498ea96f
	}
	s.send("+ses", content)
}

func (s *AriesParser) sendGam(index int, name string) {
	content := map[string]string{
		"IDENT":    fmt.Sprintf("%d", index),
		"NAME":     name,
		"PARAMS":   "8,12d,,,-1,,,1e,,-1,1,1,1,1,1,1,1,1,20,,,15f90,122d0022",
		"SYSFLAGS": "262656",
		"COUNT":    "0",
		"MAXSIZE":  "33",
	}
	s.send("+gam", content)
}

func (s *AriesParser) sendWho() {
	content := map[string]string{
		"I":   "71615",
		"N":   "Player",
		"F":   "U",
		"P":   "211",
		"S":   "1,2,3,4,5,6,7,493E0,C350",
		"X":   "0",
		"G":   "0",
		"AT":  "",
		"CL":  "511",
		"LV":  "1049601",
		"MD":  "0",
		"R":   "0",
		"US":  "0",
		"HW":  "0",
		"RP":  "0",
		"LO":  "frFR",
		"CI":  "0",
		"CT":  "0",
		"A":   "127.0.0.1",
		"LA":  "127.0.0.1",
		"C":   "4000,,7,1,1,,1,1,5553",
		"RI":  "0",
		"RT":  "0",
		"RG":  "0",
		"RGC": "0",
		"RM":  "0",
		"RF":  "0",
	}
	s.send("+who", content)
}

func (s *AriesParser) dummyResponse(kind string) {
	content := map[string]string{
		"DUMMY": "DUMMY",
	}
	s.send(kind, content)
}

func (s *AriesParser) ping() {
	// content := map[string]string{
	//	 "REF": "123",
	// }
	// s.send("~png", content)
}

func (s *AriesParser) llvlResponse(kind string) {
	content := map[string]string{
		"SKILL_PTS": "211",
		"SKILL_LVL": "1049601",
		"SKILL":     "",
	}
	s.send(kind, content)
}

func (s *AriesParser) authResponse(kind string) {
	content := map[string]string{
		"NAME":     "Player",
		"ADDR":     "127.0.0.1",
		"PERSONAS": "Player",
		"LOC":      "frFR",
		"MAIL":     "player@gmail.com",
		"SPAM":     "NN",
	}
	s.send(kind, content)
}

func (s *AriesParser) seleResponse(kind string) {
	content := map[string]string{
		"MORE":  "0",
		"SLOTS": "4",
		"STATS": "0",
	}
	s.send(kind, content)
}

func (s *AriesParser) newsResponse(kind string) {
	content := map[string]string{
		"BUDDY_SERVER": "127.0.0.1",
		"BUDDY_PORT":   "21172",
	}
	s.send(kind, content)
}

func (s *AriesParser) skeyResponse(kind string) {
	content := map[string]string{
		"SKEY": "$51ba8aee64ddfacae5baefa6bf61e009",
	}
	s.send(kind, content)
}

func (s *AriesParser) dirResponse(kind string) {
	content := map[string]string{
		"ADDR": "127.0.0.1",
		"PORT": "21172",
	}
	s.send(kind, content)
}

func (s *AriesParser) sendRom(kind string, index int, name string) {
	content := map[string]string{
		"N": name,
		"I": strconv.Itoa(index),
		"H": "test",
		"F": "CK",
		"T": "0",
		"L": "50",
	}
	s.send(kind, content)
}

func (s *AriesParser) persResponse(kind string) {
	content := map[string]string{
		"PERS":      "Player",
		"LKEY":      "3fcf27540c92935b0a66fd3b0000283c",
		"EX-ticker": "",
		"LOC":       "frFR",
		"A":         "127.0.0.1",
		"LA":        "127.0.0.1",
		"IDLE":      "6000000",
	}
	s.send(kind, content)
}

func (s *AriesParser) send(kind string, content map[string]string) {
	var msgBuffer bytes.Buffer
	totalItems := len(content)
	for key, value := range content {
		msgBuffer.WriteString(fmt.Sprintf("%s=%s", key, value))
		totalItems--
		if totalItems > 0 {
			msgBuffer.WriteString("\n")
		}
	}

	msgBuffer.WriteByte(0)

	buffer := make([]byte, len(msgBuffer.Bytes())+12)
	copy(buffer, kind)
	binary.BigEndian.PutUint32(buffer[4:], 0)
	binary.BigEndian.PutUint32(buffer[8:], uint32(len(buffer)))
	copy(buffer[12:], msgBuffer.Bytes())
	slog.Debug(color.MagentaString("TCP -->\n") + color.YellowString(hex.Dump(buffer)))

	_, err := s.conn.Write(buffer)
	if err != nil {
		slog.Error("Could not write to TCP connection", "localAddr", s.conn.LocalAddr(), "remoteAddr", s.conn.RemoteAddr(), "err", err)
		return
	}
}
