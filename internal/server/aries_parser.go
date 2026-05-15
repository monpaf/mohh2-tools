package server

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/fatih/color"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type Client struct {
	conn          net.Conn
	server        *server
	messageBuffer []byte
	currentUser   *User
	isGps         bool
	status        string // "Idle" or "In Game"
	remoteAddr    string
	remoteKey     string
	currentRoom   *Room
	currentGame   *Game
}

func NewClient(conn net.Conn, s *server) *Client {
	remoteAddr := conn.RemoteAddr().String()
	if tcpAddr, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
		remoteAddr = tcpAddr.IP.String()
	}

	return &Client{
		conn:          conn,
		server:        s,
		messageBuffer: make([]byte, 0),
		currentUser:   nil,
		isGps:         false,
		status:        "Idle",
		remoteAddr:    remoteAddr,
		remoteKey:     conn.RemoteAddr().String(),
	}
}

func (s *Client) parse(data []byte) {
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

func (s *Client) parseMessage(message []byte) {
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
	}
	if s.isGps {
		username += color.RedString(" [GPS]")
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
		s.seleResponse(kind, parseKeyValueString(content))
	case "PSET":
		s.send(kind, nil)
	case "gpss":
		s.gpssResponse(kind, parseKeyValueString(content))
	case "gpsc":
		s.gpscResponse(parseKeyValueString(content))
	case "gcre":
		s.gcreResponse(parseKeyValueString(content))
	case "gget":
		s.ggetResponse(parseKeyValueString(content))
	case "gjoi":
		s.gjoiResponse(parseKeyValueString(content))
	case "gsta":
		s.send(kind, nil)
	case "llvl":
		s.llvlResponse(kind)
	case "glea":
		s.gleaResponse(kind)
	case "gdel":
		s.gdelResponse(kind)
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

func (s *Client) authResponse(kind string, params map[string]string) {
	if params["VERS"] == "PSP/MOHGPS071" {
		s.isGps = true
		s.status = "Idle"
		slog.Info("GPS Host registered", "name", params["NAME"], "remoteAddr", s.remoteAddr)
	}

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

func (s *Client) persResponse(kind string, params map[string]string) {
	persona := params["PERS"]

	displayName := "Player"
	if s.currentUser != nil {
		displayName = s.currentUser.DisplayName
	}

	content := map[string]string{
		"LKEY":      "95feb0c73354764fc8446aee5644c429bc9ce4ddb6fe4d953e897e05f717a9c",
		"LOC":       "enCZ",
		"A":         s.remoteAddr,
		"PERS":      displayName,
		"LA":        s.remoteAddr,
		"IDLE":      "100000",
		"EX-ticker": "",
	}

	if persona != "" {
		content["PERS"] = persona
	}

	s.send(kind, content)
}

func (s *Client) sendWho() {
	name := "Player"
	displayName := "Player"
	if s.currentUser != nil {
		name = s.currentUser.Name
		displayName = s.currentUser.DisplayName
		displayName = quoteProtocolName(displayName)
		if s.isGps {
			name = "@" + name
			displayName = "@" + displayName
		}
	}

	gameID := "0"
	if s.currentGame != nil {
		gameID = fmt.Sprintf("%d", s.currentGame.ID)
	}

	roomID := "0"
	roomType := "0"
	roomName := "0"
	roomFlags := "0"
	if s.currentRoom != nil {
		roomID = fmt.Sprintf("%d", s.currentRoom.ID)
		roomType = "1"
		roomName = "room"
		roomFlags = "C"
	}

	content := map[string]string{
		"I":   "71615",                    // ID
		"M":   name,                       // Account
		"N":   displayName,                // Name
		"F":   "U",                        // Flags
		"P":   "211",                      // Persona
		"S":   "1,2,3,4,5,6,7,493E0,C350", // Status
		"X":   "0",                        // Extended info
		"G":   gameID,                     // Current game
		"AT":  "",                         // Attributes
		"CL":  "511",                      // Color
		"LV":  "1049601",                  // Level
		"MD":  "0",                        // Model/Data
		"R":   "0",                        // Rank
		"US":  "0",                        // Usage system usage flags
		"HW":  "0",                        // Hardware flags
		"RP":  "0",                        // Reputation
		"LO":  "enCZ",                     // Locale
		"CI":  "0",                        // Country ID
		"CT":  "0",                        // Connection Type
		"A":   s.remoteAddr,
		"LA":  s.remoteAddr,
		"C":   "4000,,7,1,1,,1,1,5553",
		"RI":  roomID,    // Room ID
		"RT":  roomType,  // Room Type
		"RG":  "0",       // Region
		"RGC": "0",       // Region category
		"RM":  roomName,  // Room name
		"RF":  roomFlags, // Room flags
	}
	s.send("+who", content)
}

func (s *Client) sendRom() {
	roomID := "1"
	if s.currentRoom != nil {
		roomID = fmt.Sprintf("%d", s.currentRoom.ID)
	}
	content := map[string]string{
		"I": roomID,
		"N": "room",
	}
	s.send("+rom", content)
}

func (s *Client) dirResponse(kind string, params map[string]string) {
	version := params["VERS"]

	if version == "PSP/MOHGPS071" {
		s.isGps = true
		s.status = "Idle"
	}

	content := map[string]string{
		"ADDR": s.server.tcpHostForClient(s.remoteAddr),
		"PORT": s.server.tcpPort,
	}
	s.send(kind, content)
}

func (s *server) tcpHostForClient(fallback string) string {
	if s.tcpHostIP != "" {
		return s.tcpHostIP
	}
	return fallback
}

func (s *Client) seleResponse(kind string, params map[string]string) {
	stats := params["STATS"]
	inGame := params["INGAME"]

	if stats == "" && inGame == "" {
		content := map[string]string{
			"MORE":  "0",
			"SLOTS": "4",
			"STATS": "0",
		}
		s.send(kind, content)
		s.sendWho()
		s.sendRom()
		return
	}

	content := map[string]string{
		"INGAME": inGame,
	}
	if inGame == "1" {
		content = map[string]string{
			"INGAME":    inGame,
			"MESGS":     params["MESGS"],
			"MESGTYPES": params["MESGTYPES"],
			"USERS":     params["USERS"],
			"GAMES":     params["GAMES"],
			"MYGAME":    params["MYGAME"],
			"ROOMS":     params["ROOMS"],
			"ASYNC":     params["ASYNC"],
			"USERSETS":  params["USERSETS"],
			"STATS":     stats,
		}
	}
	s.sendWithJoiner(kind, content, " ")
	s.sendWho()
}

func (s *Client) gpssResponse(kind string, params map[string]string) {
	status := params["STATUS"]
	switch status {
	case "A":
		s.status = "Idle"
	case "G":
		s.status = "In Game"
	}
	s.send(kind, nil)
}

func (s *Client) gseaResponse(kind string) {
	games := s.server.activeGames()
	sort.Slice(games, func(i, j int) bool {
		return games[i].ID < games[j].ID
	})

	content := map[string]string{
		"COUNT": fmt.Sprintf("%d", len(games)),
	}
	s.send(kind, content)

	for _, game := range games {
		s.send("+gam", s.gameSummary(game))
	}
}

func (s *Client) newsResponse(kind string) {
	content := map[string]string{
		"BUDDY_SERVER": s.server.tcpHostForClient(s.remoteAddr),
		"BUDDY_PORT":   s.server.tcpPort,
	}
	s.send(kind, content)
}

func (s *Client) skeyResponse(kind string) {
	content := map[string]string{
		"SKEY": "$51ba8aee64ddfacae5baefa6bf61e009",
	}
	s.send(kind, content)
}

func (s *Client) llvlResponse(kind string) {
	content := map[string]string{
		"SKILL_PTS": "211",
		"SKILL_LVL": "1049601",
		"SKILL":     "",
	}
	s.send(kind, content)
}

func (s *Client) dummyResponse(kind string) {
	content := map[string]string{
		"DUMMY": "DUMMY",
	}
	s.send(kind, content)
}

func (s *Client) gpscResponse(params map[string]string) {
	host := s.server.getFreeGpsHost()
	if host == nil {
		slog.Warn("Received gpsc but no free GPS host is available")
		s.send("gpscnfnd", nil)
		return
	}

	room := s.server.createRoom(host)
	s.currentRoom = room
	s.status = "In Game"

	room.mu.Lock()
	room.Clients[s.remoteKey] = s
	room.mu.Unlock()

	s.send("gpsc", nil)

	creParams := map[string]string{
		"MINSIZE":  params["MINSIZE"],
		"PASS":     params["PASS"],
		"SYSFLAGS": params["SYSFLAGS"],
		"PARAMS":   params["PARAMS"],
		"MAXSIZE":  params["MAXSIZE"],
		"NAME":     params["NAME"],
	}
	host.send("$cre", creParams)
}

func (s *Client) gcreResponse(params map[string]string) {
	game := s.server.createGame(s, params)

	s.send("gcre", nil)
	s.sendWho()
	s.sendMgm()
	s.sendRom()

	waitingClients := game.waitingClients()
	for _, client := range waitingClients {
		game.addClient(client)
		client.sendSes()
	}

	if len(waitingClients) > 0 {
		s.sendMgm()
		s.sendSes()
	}
}

func (s *Client) ggetResponse(params map[string]string) {
	game := s.server.findGame(params, s)
	if game == nil {
		s.send("gget", nil)
		return
	}

	s.send("gget", s.gameInfo(game))
}

func (s *Client) gjoiResponse(params map[string]string) {
	game := s.server.findGame(params, s)
	if game == nil {
		s.send("gjoiugam", nil)
		return
	}

	if game.Pass != "" && params["PASS"] != "" && params["PASS"] != game.Pass {
		s.send("gjoipass", nil)
		return
	}

	if game.playerCount() >= game.MaxSize {
		s.send("gjoifull", nil)
		return
	}

	if s.currentGame != nil && s.currentGame != game {
		if oldGame := s.server.removeClientFromGame(s); oldGame != nil && oldGame.Host != nil {
			oldGame.Host.sendMgm()
			oldGame.Host.sendSes()
		}
	}

	game.addClient(s)
	s.send("gjoi", nil)

	if game.Host != nil {
		game.Host.sendMgm()
		game.Host.sendSes()
	}
	s.sendSes()
}

func (s *Client) gleaResponse(kind string) {
	if game := s.server.removeClientFromGame(s); game != nil && game.Host != nil {
		game.Host.sendMgm()
		game.Host.sendSes()
	}
	s.send(kind, nil)
}

func (s *Client) gdelResponse(kind string) {
	if s.currentGame != nil && s.currentGame.Host == s {
		s.server.deleteGame(s.currentGame)
	}
	s.send(kind, nil)
}

func (g *Game) waitingClients() []*Client {
	if g.Room == nil {
		return nil
	}

	g.Room.mu.RLock()
	defer g.Room.mu.RUnlock()

	keys := make([]string, 0, len(g.Room.Clients))
	for key := range g.Room.Clients {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	clients := make([]*Client, 0, len(keys))
	for _, key := range keys {
		clients = append(clients, g.Room.Clients[key])
	}
	return clients
}

func (s *Client) sendMgm() {
	if s.currentGame == nil {
		slog.Warn("Cannot send +mgm: client is not in a game", "remoteAddr", s.remoteKey)
		return
	}
	s.send("+mgm", s.gameInfo(s.currentGame))
}

func (s *Client) sendSes() {
	if s.currentGame == nil {
		slog.Warn("Cannot send +ses: client is not in a game", "remoteAddr", s.remoteKey)
		return
	}
	s.send("+ses", s.gameInfo(s.currentGame))
}

func (s *Client) gameSummary(game *Game) map[string]string {
	game.mu.RLock()
	defer game.mu.RUnlock()

	sysflags := game.Sysflags
	if game.Pass != "" {
		if flags, err := strconv.Atoi(sysflags); err == nil {
			sysflags = strconv.Itoa(flags | (1 << 16))
		}
	}

	return map[string]string{
		"IDENT":    fmt.Sprintf("%d", game.ID),
		"NAME":     game.Name,
		"PARAMS":   s.gameParamsForClient(game),
		"SYSFLAGS": sysflags,
		"COUNT":    fmt.Sprintf("%d", game.playerCountLocked()),
		"MAXSIZE":  fmt.Sprintf("%d", game.MaxSize),
	}
}

func (s *Client) gameInfo(game *Game) map[string]string {
	game.mu.RLock()
	defer game.mu.RUnlock()

	roomID := "1"
	if game.Room != nil {
		roomID = fmt.Sprintf("%d", game.Room.ID)
	}

	auth := ""
	if game.Sysflags == "262656" {
		auth = "098f6bcd4621d373cade4e832627b4f6"
	}

	content := map[string]string{
		"IDENT":      fmt.Sprintf("%d", game.ID),
		"NAME":       game.Name,
		"HOST":       game.Host.personaName(true),
		"PARAMS":     s.gameParamsForClient(game),
		"PLATPARAMS": "0",
		"ROOM":       roomID,
		"CUSTFLAGS":  "413082880",
		"SYSFLAGS":   game.Sysflags,
		"COUNT":      fmt.Sprintf("%d", game.playerCountLocked()),
		"PRIV":       "0",
		"MINSIZE":    fmt.Sprintf("%d", game.MinSize),
		"MAXSIZE":    fmt.Sprintf("%d", game.MaxSize),
		"NUMPART":    "1",
		"SEED":       "3",
		"WHEN":       game.CreatedAt.Format("2006.1.2-15:04:05"),
		"AUTH":       auth,
		"SESS":       "0",
		"EVID":       "0",
		"EVGID":      "0",
	}

	players := game.playersLocked()
	for idx, player := range players {
		isHost := idx == 0
		idxStr := strconv.Itoa(idx)
		content["OPID"+idxStr] = player.userID()
		content["OPPO"+idxStr] = player.personaName(isHost)
		content["ADDR"+idxStr] = player.remoteAddr
		content["LADDR"+idxStr] = player.remoteAddr
		content["MADDR"+idxStr] = ""
		content["OPPART"+idxStr] = "0"
		content["OPPARAM"+idxStr] = player.opParam()
		content["OPFLAG"+idxStr] = "413082880"
		content["OPFLAGS"+idxStr] = "413082880"
		content["PRES"+idxStr] = "0"
		content["PARTSIZE"+idxStr] = fmt.Sprintf("%d", game.MaxSize)
		content["PARTPARAMS"+idxStr] = ""
	}

	return content
}

func (s *Client) gameParamsForClient(game *Game) string {
	if game == nil {
		return ""
	}
	if s.isGps || s.server.uhsProxyPort <= 0 {
		return game.Params
	}
	if game.Params == "" {
		return ""
	}

	const uhsUDPPortParamIndex = 10

	parts := strings.Split(game.Params, ",")
	for len(parts) <= uhsUDPPortParamIndex {
		parts = append(parts, "")
	}

	parts[uhsUDPPortParamIndex] = strconv.FormatInt(int64(s.server.uhsProxyPort), 16)

	return strings.Join(parts, ",")
}

func (g *Game) playerCountLocked() int {
	count := len(g.Clients)
	if g.Host != nil {
		count++
	}
	return count
}

func (g *Game) playersLocked() []*Client {
	players := make([]*Client, 0, g.playerCountLocked())
	if g.Host != nil {
		players = append(players, g.Host)
	}

	keys := make([]string, 0, len(g.Clients))
	for key := range g.Clients {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		players = append(players, g.Clients[key])
	}
	return players
}

func (s *Client) userID() string {
	if s.currentUser == nil {
		return "0"
	}
	return fmt.Sprintf("%d", s.currentUser.ID)
}

func (s *Client) personaName(host bool) string {
	name := "Player"
	if s.currentUser != nil {
		name = s.currentUser.DisplayName
	}

	name = quoteProtocolName(name)
	if host {
		return "@" + name
	}
	return name
}

func quoteProtocolName(name string) string {
	if strings.Contains(name, " ") && !(strings.HasPrefix(name, "\"") && strings.HasSuffix(name, "\"")) {
		return "\"" + name + "\""
	}
	return name
}

func (s *Client) opParam() string {
	if s.isGps {
		return "AAAAAAAAAAAAAAAARUQAAAUAAAABAAAA"
	}
	return "AAAAAAAAAAAAAAAAWkMAAAUAAAABAAAA"
}

func (s *Client) send(kind string, content map[string]string) {
	s.sendWithJoiner(kind, content, "\n")
}

func (s *Client) sendWithJoiner(kind string, content map[string]string, joiner string) {
	var msgBuffer bytes.Buffer

	if len(content) > 0 {
		keys := make([]string, 0, len(content))
		for k := range content {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for i, key := range keys {
			fmt.Fprintf(&msgBuffer, "%s=%s", key, content[key])
			if i < len(keys)-1 {
				msgBuffer.WriteString(joiner)
			}
		}

		msgBuffer.WriteByte(0)
	}

	buffer := buildPacket(kind, msgBuffer.Bytes())

	username := ""
	if s.currentUser != nil {
		username = s.currentUser.Name
	}
	if s.isGps {
		username += color.RedString(" [GPS]")
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

func buildPacket(kind string, payload []byte) []byte {
	buffer := make([]byte, len(payload)+12)
	copy(buffer, kind)
	if len(kind) == 4 {
		binary.BigEndian.PutUint32(buffer[4:], 0)
	}
	binary.BigEndian.PutUint32(buffer[8:], uint32(len(buffer)))
	copy(buffer[12:], payload)
	return buffer
}
