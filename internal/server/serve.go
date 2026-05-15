package server

import (
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/lmittmann/tint"
)

type Room struct {
	ID      int
	Host    *Client
	Clients map[string]*Client
	Game    *Game
	mu      sync.RWMutex
}

func NewRoom(id int, host *Client) *Room {
	return &Room{
		ID:      id,
		Host:    host,
		Clients: make(map[string]*Client),
	}
}

type Game struct {
	ID        int
	Name      string
	Params    string
	Sysflags  string
	Pass      string
	MinSize   int
	MaxSize   int
	CreatedAt time.Time
	Host      *Client
	Room      *Room
	Clients   map[string]*Client
	mu        sync.RWMutex
}

func NewGame(id int, host *Client, room *Room, params map[string]string) *Game {
	return &Game{
		ID:        id,
		Name:      valueOrDefault(params, "NAME", "Game"),
		Params:    valueOrDefault(params, "PARAMS", ""),
		Sysflags:  valueOrDefault(params, "SYSFLAGS", "262656"),
		Pass:      params["PASS"],
		MinSize:   intOrDefault(params, "MINSIZE", 1),
		MaxSize:   intOrDefault(params, "MAXSIZE", 17),
		CreatedAt: time.Now(),
		Host:      host,
		Room:      room,
		Clients:   make(map[string]*Client),
	}
}

func (g *Game) addClient(c *Client) {
	g.mu.Lock()
	defer g.mu.Unlock()

	for key, existing := range g.Clients {
		if existing != c && sameUser(existing, c) {
			delete(g.Clients, key)
			existing.currentGame = nil
			existing.currentRoom = nil
			existing.status = "Idle"
		}
	}

	g.Clients[c.remoteKey] = c
	c.currentGame = g
	c.currentRoom = g.Room
	c.status = "In Game"
}

func (g *Game) playerCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.playerCountLocked()
}

func valueOrDefault(params map[string]string, key, fallback string) string {
	if value, ok := params[key]; ok {
		return value
	}
	return fallback
}

func intOrDefault(params map[string]string, key string, fallback int) int {
	value, ok := params[key]
	if !ok || value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func sameUser(a, b *Client) bool {
	if a == nil || b == nil || a.currentUser == nil || b.currentUser == nil {
		return false
	}
	return a.currentUser.ID == b.currentUser.ID
}

type server struct {
	mu         sync.RWMutex
	rooms      map[int]*Room
	nextRoomID int
	games      map[int]*Game
	nextGameID int
	clients    map[string]*Client
	sslPort    string
	tcpPort    string
	udpPort    string
	db         *memoryDB
}

func newServer(sslPort, tcpPort, udpPort string) *server {
	return &server{
		rooms:      make(map[int]*Room),
		nextRoomID: 1,
		games:      make(map[int]*Game),
		nextGameID: 1,
		clients:    make(map[string]*Client),
		sslPort:    sslPort,
		tcpPort:    tcpPort,
		udpPort:    udpPort,
		db:         newDB(),
	}
}

func (s *server) addClient(c *Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clients[c.remoteKey] = c
}

func (s *server) removeClient(c *Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.clients, c.remoteKey)

	if c.currentGame != nil {
		game := c.currentGame
		game.mu.Lock()
		if game.Host == c {
			delete(s.games, game.ID)
			if game.Room != nil {
				delete(s.rooms, game.Room.ID)
			}
			for _, client := range game.Clients {
				client.currentGame = nil
				client.currentRoom = nil
				client.status = "Idle"
			}
		} else {
			delete(game.Clients, c.remoteKey)
		}
		game.mu.Unlock()
	}

	if c.currentRoom != nil {
		c.currentRoom.mu.Lock()
		if c.isGps {
			// If GPS host leaves, we might want to handle room cleanup
			delete(s.rooms, c.currentRoom.ID)
		} else {
			delete(c.currentRoom.Clients, c.remoteKey)
		}
		c.currentRoom.mu.Unlock()
	}
}

func (s *server) getFreeGpsHost() *Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range s.clients {
		if c.isGps && c.status == "Idle" {
			return c
		}
	}
	return nil
}

func (s *server) createRoom(host *Client) *Room {
	s.mu.Lock()
	defer s.mu.Unlock()

	room := NewRoom(s.nextRoomID, host)
	s.rooms[s.nextRoomID] = room
	s.nextRoomID++

	host.currentRoom = room
	host.status = "In Game"

	return room
}

func (s *server) createGame(host *Client, params map[string]string) *Game {
	s.mu.Lock()
	defer s.mu.Unlock()

	room := host.currentRoom
	if room == nil {
		room = NewRoom(s.nextRoomID, host)
		s.rooms[s.nextRoomID] = room
		s.nextRoomID++
		host.currentRoom = room
	}

	game := NewGame(s.nextGameID, host, room, params)
	s.games[game.ID] = game
	s.nextGameID++

	room.Game = game
	host.currentGame = game
	host.status = "In Game"

	return game
}

func (s *server) activeGames() []*Game {
	s.mu.RLock()
	defer s.mu.RUnlock()

	games := make([]*Game, 0, len(s.games))
	for _, game := range s.games {
		games = append(games, game)
	}
	return games
}

func (s *server) findGame(params map[string]string, client *Client) *Game {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if ident := params["IDENT"]; ident != "" {
		id, err := strconv.Atoi(ident)
		if err == nil {
			return s.games[id]
		}
	}

	if name := params["NAME"]; name != "" {
		for _, game := range s.games {
			if game.Name == name {
				return game
			}
		}
	}

	if params["PERS"] != "" && client != nil {
		return client.currentGame
	}

	return nil
}

func (s *server) removeClientFromGame(c *Client) *Game {
	if c.currentGame == nil {
		return nil
	}

	game := c.currentGame
	game.mu.Lock()
	delete(game.Clients, c.remoteKey)
	game.mu.Unlock()

	c.currentGame = nil
	c.currentRoom = nil
	c.status = "Idle"

	return game
}

func (s *server) deleteGame(game *Game) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.games, game.ID)
	if game.Room != nil {
		delete(s.rooms, game.Room.ID)
		game.Room.mu.Lock()
		game.Room.Game = nil
		game.Room.Clients = make(map[string]*Client)
		game.Room.mu.Unlock()
	}

	game.mu.Lock()
	if game.Host != nil {
		game.Host.currentGame = nil
		game.Host.currentRoom = nil
		game.Host.status = "Idle"
	}
	for _, client := range game.Clients {
		client.currentGame = nil
		client.currentRoom = nil
		client.status = "Idle"
	}
	game.Clients = make(map[string]*Client)
	game.mu.Unlock()
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
	client := NewClient(conn, s)
	s.addClient(client)

	defer func() {
		s.removeClient(client)
	}()

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
		client.parse(buffer[:n])
	}
}
