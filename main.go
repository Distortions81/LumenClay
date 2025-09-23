package main

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// ---------- Telnet & ANSI Utilities ----------

const (
	telnetIAC  byte = 255
	telnetDONT byte = 254
	telnetDO   byte = 253
	telnetWONT byte = 252
	telnetWILL byte = 251
	telnetSB   byte = 250
	telnetSE   byte = 240
	telnetNOP  byte = 241
	telnetDM   byte = 242
	telnetBRK  byte = 243
	telnetIP   byte = 244
	telnetAO   byte = 245
	telnetAYT  byte = 246
	telnetEC   byte = 247
	telnetEL   byte = 248
	telnetGA   byte = 249
)

const (
	telnetOptEcho         byte = 1
	telnetOptSuppressGA   byte = 3
	telnetOptTerminalType byte = 24
	telnetOptWindowSize   byte = 31
	telnetOptLineMode     byte = 34
)

var (
	serverSupportedOptions = map[byte]bool{
		telnetOptSuppressGA: true,
	}
	clientSupportedOptions = map[byte]bool{
		telnetOptTerminalType: true,
		telnetOptWindowSize:   true,
	}
)

const (
	ansiReset     = "\x1b[0m"
	ansiBold      = "\x1b[1m"
	ansiDim       = "\x1b[2m"
	ansiItalic    = "\x1b[3m"
	ansiUnderline = "\x1b[4m"
	ansiCyan      = "\x1b[36m"
	ansiYellow    = "\x1b[33m"
	ansiGreen     = "\x1b[32m"
	ansiMagenta   = "\x1b[35m"
)

type TelnetSession struct {
	conn   net.Conn
	reader *bufio.Reader
	mu     sync.Mutex
	width  int
	height int
	term   string
}

func NewTelnetSession(conn net.Conn) *TelnetSession {
	s := &TelnetSession{
		conn:   conn,
		reader: bufio.NewReader(conn),
		width:  80,
		height: 24,
	}
	s.performHandshake()
	return s
}

func (s *TelnetSession) performHandshake() {
	_ = s.writeCommand(telnetWILL, telnetOptSuppressGA)
	_ = s.writeCommand(telnetWONT, telnetOptEcho)
	_ = s.writeCommand(telnetDONT, telnetOptLineMode)
	_ = s.writeCommand(telnetDO, telnetOptTerminalType)
	_ = s.writeCommand(telnetDO, telnetOptWindowSize)
}

func (s *TelnetSession) writeCommand(cmd, opt byte) error {
	return s.writeRaw([]byte{telnetIAC, cmd, opt})
}

func (s *TelnetSession) writeRaw(payload []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.conn.Write(payload)
	return err
}

func (s *TelnetSession) WriteString(msg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data := translateForTelnet(msg)
	_, err := s.conn.Write(data)
	return err
}

func translateForTelnet(msg string) []byte {
	var buf bytes.Buffer
	var prev byte
	for i := 0; i < len(msg); i++ {
		b := msg[i]
		switch b {
		case '\n':
			if prev != '\r' {
				buf.WriteByte('\r')
			}
			buf.WriteByte('\n')
		case telnetIAC:
			buf.WriteByte(telnetIAC)
			buf.WriteByte(telnetIAC)
		default:
			buf.WriteByte(b)
		}
		prev = b
	}
	return buf.Bytes()
}

func (s *TelnetSession) ReadLine() (string, error) {
	var buf bytes.Buffer
	for {
		b, err := s.reader.ReadByte()
		if err != nil {
			return "", err
		}
		switch b {
		case '\r':
			if next, err := s.reader.Peek(1); err == nil && next[0] == '\n' {
				_, _ = s.reader.ReadByte()
			}
			return buf.String(), nil
		case '\n':
			return buf.String(), nil
		case 0x08, 0x7f:
			bs := buf.Bytes()
			if len(bs) > 0 {
				buf.Truncate(len(bs) - 1)
			}
		case 0x00:
			// ignore NULs
		case telnetIAC:
			if err := s.handleIAC(&buf); err != nil {
				return "", err
			}
		default:
			buf.WriteByte(b)
		}
	}
}

func (s *TelnetSession) handleIAC(buf *bytes.Buffer) error {
	cmd, err := s.reader.ReadByte()
	if err != nil {
		return err
	}
	switch cmd {
	case telnetIAC:
		buf.WriteByte(telnetIAC)
	case telnetDO, telnetDONT, telnetWILL, telnetWONT:
		opt, err := s.reader.ReadByte()
		if err != nil {
			return err
		}
		s.handleNegotiation(cmd, opt)
	case telnetSB:
		return s.handleSubnegotiation()
	case telnetNOP, telnetDM, telnetBRK, telnetIP, telnetAO, telnetAYT, telnetEC, telnetEL, telnetGA:
		// ignored control commands
	default:
		// ignore anything unknown to keep stream resilient
	}
	return nil
}

func (s *TelnetSession) handleNegotiation(cmd, opt byte) {
	switch cmd {
	case telnetDO:
		if serverSupportedOptions[opt] {
			_ = s.writeCommand(telnetWILL, opt)
		} else {
			_ = s.writeCommand(telnetWONT, opt)
		}
	case telnetDONT:
		_ = s.writeCommand(telnetWONT, opt)
	case telnetWILL:
		if clientSupportedOptions[opt] {
			_ = s.writeCommand(telnetDO, opt)
		} else {
			_ = s.writeCommand(telnetDONT, opt)
		}
	case telnetWONT:
		_ = s.writeCommand(telnetDONT, opt)
	}
}

func (s *TelnetSession) handleSubnegotiation() error {
	opt, err := s.reader.ReadByte()
	if err != nil {
		return err
	}
	payload := make([]byte, 0, 16)
	for {
		b, err := s.reader.ReadByte()
		if err != nil {
			return err
		}
		if b == telnetIAC {
			esc, err := s.reader.ReadByte()
			if err != nil {
				return err
			}
			if esc == telnetIAC {
				payload = append(payload, telnetIAC)
				continue
			}
			if esc == telnetSE {
				break
			}
			// unexpected command inside subnegotiation, ignore and continue
			continue
		}
		payload = append(payload, b)
	}

	switch opt {
	case telnetOptTerminalType:
		if len(payload) > 1 && payload[0] == 0 { // IS
			s.term = strings.ToUpper(string(payload[1:]))
		}
	case telnetOptWindowSize:
		if len(payload) >= 4 {
			s.width = int(payload[0])<<8 | int(payload[1])
			s.height = int(payload[2])<<8 | int(payload[3])
		}
	}
	return nil
}

func (s *TelnetSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn.Close()
}

func (s *TelnetSession) Size() (int, int) {
	return s.width, s.height
}

func (s *TelnetSession) Terminal() string {
	return s.term
}

func style(text string, attrs ...string) string {
	if len(attrs) == 0 {
		return text
	}
	return strings.Join(attrs, "") + text + ansiReset
}

func highlightName(name string) string {
	return style(name, ansiBold, ansiCyan)
}

func highlightNames(list []string) []string {
	out := make([]string, len(list))
	for i, name := range list {
		out[i] = highlightName(name)
	}
	return out
}

// ---------- World Model ----------

type RoomID string

type Room struct {
	ID          RoomID
	Title       string
	Description string
	Exits       map[string]RoomID // "n","s","e","w", etc.
}

type Player struct {
	Name    string
	Session *TelnetSession
	Room    RoomID
	Output  chan string
	Alive   bool
	IsAdmin bool
}

type World struct {
	mu      sync.RWMutex
	rooms   map[RoomID]*Room
	players map[string]*Player // by name
}

type AccountManager struct {
	mu    sync.RWMutex
	creds map[string]string
}

func NewAccountManager() *AccountManager {
	return &AccountManager{creds: make(map[string]string)}
}

func (a *AccountManager) Exists(name string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	_, ok := a.creds[name]
	return ok
}

func (a *AccountManager) Register(name, pass string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.creds[name]; ok {
		return fmt.Errorf("account already exists")
	}
	a.creds[name] = pass
	return nil
}

func (a *AccountManager) Authenticate(name, pass string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	expected, ok := a.creds[name]
	if !ok {
		return false
	}
	return expected == pass
}

func NewWorld() *World {
	w := &World{
		rooms:   make(map[RoomID]*Room),
		players: make(map[string]*Player),
	}
	w.rooms = defaultRooms()
	return w
}

func defaultRooms() map[RoomID]*Room {
	rooms := map[RoomID]*Room{}
	rooms["start"] = &Room{
		ID:    "start",
		Title: "Worn Stone Landing",
		Description: "You stand on a time-polished stone platform. " +
			"Four archways lead to shadowed corridors: north (the library), " +
			"east (the workshop), south (the garden), west (the market).",
		Exits: map[string]RoomID{"n": "library", "e": "workshop", "s": "garden", "w": "market"},
	}
	rooms["library"] = &Room{
		ID:          "library",
		Title:       "Dustlit Library",
		Description: "Shelves lean with the weight of forgotten ideas. A faint smell of paper and ozone.",
		Exits:       map[string]RoomID{"s": "start"},
	}
	rooms["workshop"] = &Room{
		ID:          "workshop",
		Title:       "Crackle Workshop",
		Description: "Benches, tools, and half-built contraptions hum with patient possibility.",
		Exits:       map[string]RoomID{"w": "start"},
	}
	rooms["garden"] = &Room{
		ID:          "garden",
		Title:       "Night Garden",
		Description: "Bioluminescent vines twine overhead. Footsteps hush on moss.",
		Exits:       map[string]RoomID{"n": "start"},
	}
	rooms["market"] = &Room{
		ID:          "market",
		Title:       "Silent Market",
		Description: "Stalls stand ready for traders that never quite arrive.",
		Exits:       map[string]RoomID{"e": "start"},
	}
	return rooms
}

// ---------- Utility ----------

func trim(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, "\r", ""))
}

func ansi(c string) string {
	if strings.Contains(c, "\x1b[") && !strings.HasSuffix(c, ansiReset) {
		return c + ansiReset
	}
	return c
}

func prompt(p *Player) string { return ansi(style("\r\n> ", ansiBold, ansiYellow)) }

func validateUsername(name string) error {
	if name == "" {
		return fmt.Errorf("name cannot be empty")
	}
	if strings.ContainsAny(name, " \t\r\n") {
		return fmt.Errorf("name cannot contain spaces")
	}
	if len(name) > 24 {
		return fmt.Errorf("name must be 24 characters or fewer")
	}
	return nil
}

func login(session *TelnetSession, accounts *AccountManager) (string, bool, error) {
	_ = session.WriteString(ansi(style("\r\nLogin required.\r\n", ansiMagenta, ansiBold)))
	for attempts := 0; attempts < 5; attempts++ {
		_ = session.WriteString(ansi("\r\nUsername: "))
		username, err := session.ReadLine()
		if err != nil {
			return "", false, err
		}
		username = trim(username)
		if err := validateUsername(username); err != nil {
			_ = session.WriteString(ansi(style("\r\n"+err.Error(), ansiYellow)))
			continue
		}
		if accounts.Exists(username) {
			for tries := 0; tries < 3; tries++ {
				_ = session.WriteString(ansi("\r\nPassword: "))
				password, err := session.ReadLine()
				if err != nil {
					return "", false, err
				}
				password = trim(password)
				if accounts.Authenticate(username, password) {
					_ = session.WriteString(ansi(style("\r\nWelcome back, "+username+"!", ansiGreen)))
					return username, strings.EqualFold(username, "admin"), nil
				}
				_ = session.WriteString(ansi(style("\r\nIncorrect password.", ansiYellow)))
			}
			_ = session.WriteString(ansi("\r\nToo many failed attempts.\r\n"))
			return "", false, fmt.Errorf("authentication failed")
		}

		for {
			_ = session.WriteString(ansi("\r\nSet a password: "))
			password, err := session.ReadLine()
			if err != nil {
				return "", false, err
			}
			password = trim(password)
			if password == "" {
				_ = session.WriteString(ansi(style("\r\nPassword cannot be blank.", ansiYellow)))
				continue
			}
			if err := accounts.Register(username, password); err != nil {
				_ = session.WriteString(ansi(style("\r\n"+err.Error(), ansiYellow)))
				break
			}
			_ = session.WriteString(ansi(style("\r\nAccount created. Welcome, "+username+"!", ansiGreen)))
			return username, strings.EqualFold(username, "admin"), nil
		}
	}
	_ = session.WriteString(ansi("\r\nLogin cancelled.\r\n"))
	return "", false, fmt.Errorf("login cancelled")
}

// ---------- World Methods (concurrency-safe) ----------

func (w *World) addPlayer(name string, session *TelnetSession, isAdmin bool) (*Player, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if existing, ok := w.players[name]; ok {
		if existing.Alive {
			return nil, fmt.Errorf("%s is already connected", name)
		}
		existing.Session = session
		existing.Output = make(chan string, 32)
		existing.Room = "start"
		existing.Alive = true
		existing.IsAdmin = isAdmin
		return existing, nil
	}
	p := &Player{Name: name, Session: session, Room: "start", Output: make(chan string, 32), Alive: true, IsAdmin: isAdmin}
	w.players[name] = p
	return p, nil
}

func (w *World) removePlayer(name string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if p, ok := w.players[name]; ok {
		delete(w.players, name)
		close(p.Output)
	}
}

func (w *World) reboot() []*Player {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.rooms = defaultRooms()
	revived := make([]*Player, 0, len(w.players))
	for _, p := range w.players {
		p.Room = "start"
		revived = append(revived, p)
	}
	return revived
}

func (w *World) getRoom(id RoomID) (*Room, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	r, ok := w.rooms[id]
	return r, ok
}

func (w *World) broadcastToRoom(room RoomID, msg string, except *Player) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	for _, p := range w.players {
		if p.Room == room && p != except && p.Alive {
			select {
			case p.Output <- msg:
			default:
			}
		}
	}
}

func (w *World) renamePlayer(p *Player, newName string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, taken := w.players[newName]; taken {
		return fmt.Errorf("that name is taken")
	}
	delete(w.players, p.Name)
	p.Name = newName
	w.players[newName] = p
	return nil
}

func (w *World) listPlayers(roomOnly bool, room RoomID) []string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	names := []string{}
	for _, p := range w.players {
		if !p.Alive {
			continue
		}
		if roomOnly && p.Room != room {
			continue
		}
		names = append(names, p.Name)
	}
	return names
}

func (w *World) move(p *Player, dir string) (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	r := w.rooms[p.Room]
	next, ok := r.Exits[dir]
	if !ok {
		return "", fmt.Errorf("you can't go that way")
	}
	p.Room = next
	return string(next), nil
}

// ---------- Connection Handling ----------

func handleConn(conn net.Conn, world *World, accounts *AccountManager) {
	session := NewTelnetSession(conn)
	defer session.Close()
	username, isAdmin, err := login(session, accounts)
	if err != nil {
		return
	}

	p, err := world.addPlayer(username, session, isAdmin)
	if err != nil {
		_ = session.WriteString(ansi(style("\r\n"+err.Error()+"\r\n", ansiYellow)))
		return
	}

	// Writer goroutine
	go func() {
		for out := range p.Output {
			_ = session.WriteString(out)
		}
	}()

	// Welcome
	p.Output <- ansi(style("\r\nWelcome to the tiny Go MUD. Type 'help' for commands.", ansiMagenta, ansiBold))
	enterRoom(world, p, "")

	_ = conn.SetReadDeadline(time.Time{}) // no deadline

	// Reader loop
	for {
		line, err := session.ReadLine()
		if err != nil {
			break
		}
		line = trim(line)
		if line == "" {
			p.Output <- prompt(p)
			continue
		}
		if !p.Alive {
			break
		}
		if quit := dispatch(world, p, line); quit {
			break
		}
		p.Output <- prompt(p)
	}
	// Disconnect
	p.Alive = false
	world.broadcastToRoom(p.Room, ansi(fmt.Sprintf("\r\n%s leaves.", highlightName(p.Name))), p)
	world.removePlayer(p.Name)
}

func enterRoom(world *World, p *Player, via string) {
	r, _ := world.getRoom(p.Room)
	if via != "" {
		world.broadcastToRoom(p.Room, ansi(fmt.Sprintf("\r\n%s arrives from %s.", highlightName(p.Name), via)), p)
	}
	title := style(r.Title, ansiBold, ansiCyan)
	desc := style(r.Description, ansiItalic, ansiDim)
	exits := style(exitList(r), ansiGreen)
	p.Output <- ansi(fmt.Sprintf("\r\n\r\n%s\r\n%s\r\nExits: %s", title, desc, exits))
	others := world.listPlayers(true, p.Room)
	if len(others) > 1 { // include self in list, so >1 means someone else is here
		seen := filterOut(others, p.Name)
		colored := highlightNames(seen)
		p.Output <- ansi(fmt.Sprintf("\r\nYou see: %s", strings.Join(colored, ", ")))
	}
	p.Output <- prompt(p)
}

func exitList(r *Room) string {
	if len(r.Exits) == 0 {
		return "none"
	}
	keys := []string{}
	for k := range r.Exits {
		keys = append(keys, k)
	}
	return strings.Join(keys, " ")
}

func filterOut(list []string, name string) []string {
	out := make([]string, 0, len(list))
	for _, v := range list {
		if v != name {
			out = append(out, v)
		}
	}
	return out
}

// ---------- Command Dispatch ----------

func dispatch(world *World, p *Player, line string) bool {
	parts := strings.Fields(line)
	cmd := strings.ToLower(parts[0])
	arg := strings.TrimSpace(strings.TrimPrefix(line, parts[0]))
	arg = strings.TrimLeft(arg, " ")

	switch cmd {
	case "help", "?":
		header := style("\r\nCommands:\r\n", ansiBold, ansiUnderline)
		body := "  help               - show this message\r\n" +
			"  look               - describe your room\r\n" +
			"  say <msg>          - chat to the room\r\n" +
			"  emote <action>     - emote to the room (e.g. 'emote shrugs')\r\n" +
			"  who                - list connected players\r\n" +
			"  name <newname>     - change your display name\r\n" +
			"  reboot             - reload the world (admin only)\r\n" +
			"  go <n|s|e|w>       - move by direction\r\n" +
			"  n/s/e/w            - shorthand for movement\r\n" +
			"  quit               - disconnect"
		p.Output <- ansi(header + body)
	case "look", "l":
		r, _ := world.getRoom(p.Room)
		title := style(r.Title, ansiBold, ansiCyan)
		desc := style(r.Description, ansiItalic, ansiDim)
		exits := style(exitList(r), ansiGreen)
		p.Output <- ansi(fmt.Sprintf("\r\n%s\r\n%s\r\nExits: %s", title, desc, exits))
		others := world.listPlayers(true, p.Room)
		if len(others) > 1 {
			seen := filterOut(others, p.Name)
			colored := highlightNames(seen)
			p.Output <- ansi(fmt.Sprintf("\r\nYou see: %s", strings.Join(colored, ", ")))
		}
	case "say":
		if arg == "" {
			p.Output <- ansi(style("\r\nSay what?", ansiYellow))
			return false
		}
		world.broadcastToRoom(p.Room, ansi(fmt.Sprintf("\r\n%s says: %s", highlightName(p.Name), arg)), p)
		p.Output <- ansi(fmt.Sprintf("\r\n%s %s", style("You say:", ansiBold, ansiYellow), arg))
	case "emote", ":":
		if arg == "" {
			p.Output <- ansi(style("\r\nEmote what?", ansiYellow))
			return false
		}
		world.broadcastToRoom(p.Room, ansi(fmt.Sprintf("\r\n%s %s", highlightName(p.Name), arg)), p)
		p.Output <- ansi(fmt.Sprintf("\r\n%s %s", style("You", ansiBold, ansiYellow), arg))
	case "who":
		names := world.listPlayers(false, "")
		p.Output <- ansi("\r\nPlayers: " + strings.Join(highlightNames(names), ", "))
	case "name":
		newName := strings.TrimSpace(arg)
		if newName == "" {
			p.Output <- ansi(style("\r\nUsage: name <newname>", ansiYellow))
			return false
		}
		if strings.ContainsAny(newName, " \t\r\n") || len(newName) > 24 {
			p.Output <- ansi(style("\r\nInvalid name.", ansiYellow))
			return false
		}
		old := p.Name
		if err := world.renamePlayer(p, newName); err != nil {
			p.Output <- ansi(style("\r\n"+err.Error(), ansiYellow))
			return false
		}
		world.broadcastToRoom(p.Room, ansi(fmt.Sprintf("\r\n%s is now known as %s.", highlightName(old), highlightName(newName))), p)
		p.Output <- ansi(fmt.Sprintf("\r\nYou are now known as %s.", highlightName(newName)))
	case "reboot":
		if !p.IsAdmin {
			p.Output <- ansi(style("\r\nOnly admins may reboot the world.", ansiYellow))
			return false
		}
		p.Output <- ansi(style("\r\nRebooting the world...", ansiMagenta, ansiBold))
		players := world.reboot()
		for _, target := range players {
			target.Output <- ansi(style("\r\nReality shimmers as the world is rebooted.", ansiMagenta))
			enterRoom(world, target, "")
		}
	case "go":
		dir := strings.ToLower(strings.TrimSpace(arg))
		if dir == "" {
			p.Output <- ansi(style("\r\nUsage: go <n|s|e|w>", ansiYellow))
			return false
		}
		return move(world, p, dir)
	case "n", "s", "e", "w":
		return move(world, p, cmd)
	case "quit", "q":
		p.Output <- ansi("\r\nGoodbye.\r\n")
		return true
	default:
		p.Output <- ansi("\r\nUnknown command. Type 'help'.")
	}
	return false
}

func move(world *World, p *Player, dir string) bool {
	prev := p.Room
	next, err := world.move(p, dir)
	if err != nil {
		p.Output <- ansi("\r\n" + err.Error())
		return false
	}
	world.broadcastToRoom(RoomID(prev), ansi(fmt.Sprintf("\r\n%s leaves %s.", highlightName(p.Name), dir)), p)
	enterRoom(world, p, dir)
	_ = next
	return false
}

// ---------- Server ----------

func main() {
	addr := ":4000"
	world := NewWorld()
	accounts := NewAccountManager()

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		panic(err)
	}
	fmt.Printf("MUD listening on %s (telnet + ANSI ready)\n", addr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		// Telnet negotiation and ANSI handling are done per-connection in handleConn.
		go handleConn(conn, world, accounts)
	}
}
