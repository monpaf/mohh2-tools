package server

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strings"

	"github.com/fatih/color"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type AriesParser struct {
	conn          net.Conn
	server        *server
	messageBuffer []byte
	currentUser   *User
	isGps         bool
	remoteAddr    string
}

func NewAriesParser(conn net.Conn, s *server) *AriesParser {
	return &AriesParser{
		conn:          conn,
		server:        s,
		messageBuffer: make([]byte, 0),
		currentUser:   nil,
		isGps:         false,
		remoteAddr:    conn.RemoteAddr().(*net.TCPAddr).IP.String(),
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
			if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
				value = value[1 : len(value)-1]
			}
			result[key] = value
		}
	}
	return result
}

func (s *AriesParser) parseMessage(message []byte) {
	kind := string(message[0:4])
	packetSize := binary.BigEndian.Uint32(message[8:12])
	content := ""
	if packetSize > 12 {
		content = string(message[12:packetSize])
		content = strings.TrimRight(content, "\x00")
	}

	username := ""
	if s.currentUser != nil {
		username = s.currentUser.Name
		if s.isGps {
			username = color.RedString(" [GPS]")
		}
	}

	slog.Debug(fmt.Sprintf("%s %s\n%s",
		color.MagentaString("TCP <--"),
		username,
		color.CyanString("\n"+hex.Dump(message))))

	switch kind {
	case "~png":
	case "@tic":
	case "addr":
	case "@dir":
		s.dirResponse(kind, parseKeyValueString(content))
	case "sele":
		s.seleResponse(kind)
	case "gpsc":
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
		s.persResponse(kind, parseKeyValueString(content))
	case "skey":
		s.skeyResponse(kind)
	case "news":
		s.newsResponse(kind)
	case "auth":
		s.authResponse(kind, parseKeyValueString(content))
	default:
		slog.Warn("Unknown packet type", "kind", kind)
	}
}

func (s *AriesParser) authResponse(kind string, params map[string]string) {
	name := strings.TrimPrefix(params["NAME"], "@")
	encryptedPass := params["PASS"]

	user, ok := s.server.db.GetUserByName(name)
	if !ok {
		slog.Warn("Auth failed: user not found", "name", name)
		return
	}

	passToDecrypt := strings.TrimPrefix(encryptedPass, "~")

	decodedPass, err := url.QueryUnescape(passToDecrypt)
	if err != nil {
		slog.Error("Failed to URL decode password", "err", err)
		decodedPass = passToDecrypt
	}

	key := []byte{0x51, 0xba, 0x8a, 0xee, 0x64, 0xdd, 0xfa, 0xca, 0xe5, 0xba, 0xef, 0xa6, 0xbf, 0x61, 0xe0, 0x09}
	decryptedPass := CryptSSC2StringDecrypt([]byte(decodedPass), key, 16)
	slog.Debug("Password decryption", "encrypted", encryptedPass, "decrypted", decryptedPass, "expected", user.Password)

	if decryptedPass != user.Password {
		slog.Warn("Auth failed: password mismatch", "name", name)
		return
	}

	s.currentUser = user

	content := map[string]string{
		"LOC":      "enCZ",
		"MAIL":     user.Email,
		"PERSONAS": cases.Title(language.English).String(user.Name),
		"NAME":     user.Name,
		"ADDR":     s.remoteAddr,
		"SPAM":     "NN",
	}
	s.send(kind, content)
}

func (s *AriesParser) persResponse(kind string, params map[string]string) {
	persona := params["PERS"]

	name := "Player"
	if s.currentUser != nil {
		name = cases.Title(language.English).String(s.currentUser.Name)
	}

	content := map[string]string{
		"LKEY":      "95feb0c73354764fc8446aee5644c429bc9ce4ddb6fe4d953e897e05f717a9c",
		"LOC":       "enCZ",
		"A":         s.remoteAddr,
		"PERS":      name,
		"LA":        s.remoteAddr,
		"IDLE":      "100000",
		"EX-ticker": "",
	}

	if persona != "" {
		content["PERS"] = persona
	}

	s.send(kind, content)
	s.sendWho()
}

func (s *AriesParser) sendWho() {
	name := "Player"
	if s.currentUser != nil {
		name = cases.Title(language.English).String(s.currentUser.Name)
	}

	content := map[string]string{
		"I":   "71615",
		"N":   name,
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
		"LO":  "enCZ",
		"CI":  "0",
		"CT":  "0",
		"A":   s.remoteAddr,
		"LA":  s.remoteAddr,
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

func (s *AriesParser) dirResponse(kind string, params map[string]string) {
	version := params["VERS"]

	if version == "PSP/MOHGPS071" {
		s.isGps = true
	}

	content := map[string]string{
		"ADDR": s.remoteAddr,
		"PORT": s.server.tcpPort,
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

func (s *AriesParser) gseaResponse(kind string) {
	content := map[string]string{
		"COUNT": "0",
	}
	s.send(kind, content)
}

func (s *AriesParser) newsResponse(kind string) {
	content := map[string]string{
		"BUDDY_SERVER": s.remoteAddr,
		"BUDDY_PORT":   s.server.tcpPort,
	}
	s.send(kind, content)
}

func (s *AriesParser) skeyResponse(kind string) {
	content := map[string]string{
		"SKEY": "$51ba8aee64ddfacae5baefa6bf61e009",
	}
	s.send(kind, content)
}

func (s *AriesParser) llvlResponse(kind string) {
	content := map[string]string{
		"SKILL_PTS": "211",
		"SKILL_LVL": "1049601",
		"SKILL":     "",
	}
	s.send(kind, content)
}

func (s *AriesParser) dummyResponse(kind string) {
	content := map[string]string{
		"DUMMY": "DUMMY",
	}
	s.send(kind, content)
}

func (s *AriesParser) sendSes(receivedContent map[string]string) {
	name := "Player"
	if s.currentUser != nil {
		name = cases.Title(language.English).String(s.currentUser.Name)
	}

	content := map[string]string{
		"IDENT":       "12",
		"NAME":        name,
		"HOST":        name,
		"GPSHOST":     name,
		"PARAMS":      receivedContent["PARAMS"],
		"ROOM":        "13",
		"CUSTFLAGS":   "0",
		"SYSFLAGS":    "262656",
		"COUNT":       "1",
		"PRIV":        "0",
		"MINSIZE":     "0",
		"MAXSIZE":     "33",
		"NUMPART":     "1",
		"SEED":        "012345",
		"WHEN":        "2009.2.8-9:44:15",
		"GAMEPORT":    s.server.tcpPort,
		"VOIPPORT":    s.server.tcpPort,
		"OPID0":       "0",
		"OPPO0":       name,
		"ADDR0":       s.remoteAddr,
		"LADDR0":      s.remoteAddr,
		"MADDR0":      "$0017ab8f4451",
		"OPPARAM0":    "AAAAAAAAAAAAAAAAAAAAAQBuDCgAAAAC",
		"PARTSIZE0":   "16",
		"PARTPARAMS0": "0",
	}
	s.send("+ses", content)
}

func (s *AriesParser) send(kind string, content map[string]string) {
	var msgBuffer bytes.Buffer

	keys := make([]string, 0, len(content))
	for k := range content {
		keys = append(keys, k)
	}

	for i, key := range keys {
		fmt.Fprintf(&msgBuffer, "%s=%s", key, content[key])
		if i < len(keys)-1 {
			msgBuffer.WriteString("\n")
		}
	}

	msgBuffer.WriteByte(0)

	buffer := make([]byte, len(msgBuffer.Bytes())+12)
	copy(buffer, kind)
	binary.BigEndian.PutUint32(buffer[4:], 0)
	binary.BigEndian.PutUint32(buffer[8:], uint32(len(buffer)))
	copy(buffer[12:], msgBuffer.Bytes())

	username := ""
	if s.currentUser != nil {
		username = s.currentUser.Name
		if s.isGps {
			username = color.RedString(" [GPS]")
		}
	}

	slog.Debug(fmt.Sprintf("%s %s\n%s",
		color.MagentaString("TCP -->"),
		username,
		color.YellowString("\n"+hex.Dump(buffer))))

	_, err := s.conn.Write(buffer)
	if err != nil {
		slog.Error("Could not write to TCP connection", "err", err)
		return
	}
}
