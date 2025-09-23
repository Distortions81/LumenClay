package main

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// ---------- World Model ----------

type RoomID string

type Room struct {
	ID          RoomID
	Title       string
	Description string
	Exits       map[string]RoomID // "n","s","e","w", etc.
}

type Player struct {
	Name   string
	Conn   net.Conn
	Room   RoomID
	Output chan string
	Alive  bool
}

type World struct {
	mu      sync.RWMutex
	rooms   map[RoomID]*Room
	players map[string]*Player // by name
	guests  int
}

func NewWorld() *World {
	w := &World{
		rooms:   make(map[RoomID]*Room),
		players: make(map[string]*Player),
	}
	// Tiny starter map
	w.rooms["start"] = &Room{
		ID:    "start",
		Title: "Worn Stone Landing",
		Description: "You stand on a time-polished stone platform. " +
			"Four archways lead to shadowed corridors: north (the library), " +
			"east (the workshop), south (the garden), west (the market).",
		Exits: map[string]RoomID{"n": "library", "e": "workshop", "s": "garden", "w": "market"},
	}
	w.rooms["library"] = &Room{
		ID:          "library",
		Title:       "Dustlit Library",
		Description: "Shelves lean with the weight of forgotten ideas. A faint smell of paper and ozone.",
		Exits:       map[string]RoomID{"s": "start"},
	}
	w.rooms["workshop"] = &Room{
		ID:          "workshop",
		Title:       "Crackle Workshop",
		Description: "Benches, tools, and half-built contraptions hum with patient possibility.",
		Exits:       map[string]RoomID{"w": "start"},
	}
	w.rooms["garden"] = &Room{
		ID:          "garden",
		Title:       "Night Garden",
		Description: "Bioluminescent vines twine overhead. Footsteps hush on moss.",
		Exits:       map[string]RoomID{"n": "start"},
	}
	w.rooms["market"] = &Room{
		ID:          "market",
		Title:       "Silent Market",
		Description: "Stalls stand ready for traders that never quite arrive.",
		Exits:       map[string]RoomID{"e": "start"},
	}
	return w
}

// ---------- Utility ----------

func trim(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, "\r", ""))
}

func ansi(c string) string { return c } // placeholder for later colorization

func prompt(p *Player) string { return ansi("\r\n> ") }

// ---------- World Methods (concurrency-safe) ----------

func (w *World) addPlayer(conn net.Conn) *Player {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.guests++
	name := fmt.Sprintf("Guest%d", w.guests)
	p := &Player{Name: name, Conn: conn, Room: "start", Output: make(chan string, 32), Alive: true}
	w.players[name] = p
	return p
}

func (w *World) removePlayer(name string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if p, ok := w.players[name]; ok {
		delete(w.players, name)
		close(p.Output)
	}
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

func handleConn(conn net.Conn, world *World) {
	defer conn.Close()
	p := world.addPlayer(conn)

	// Writer goroutine
	go func() {
		for out := range p.Output {
			_, _ = conn.Write([]byte(out))
		}
	}()

	// Welcome
	p.Output <- ansi("\r\nWelcome to the tiny Go MUD. Type 'help' for commands.")
	enterRoom(world, p, "")

	sc := bufio.NewScanner(conn)
	_ = conn.SetReadDeadline(time.Time{}) // no deadline

	// Reader loop
	for sc.Scan() {
		line := trim(sc.Text())
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
	world.broadcastToRoom(p.Room, ansi(fmt.Sprintf("\r\n%s leaves.", p.Name)), p)
	world.removePlayer(p.Name)
}

func enterRoom(world *World, p *Player, via string) {
	r, _ := world.getRoom(p.Room)
	if via != "" {
		world.broadcastToRoom(p.Room, ansi(fmt.Sprintf("\r\n%s arrives from %s.", p.Name, via)), p)
	}
	p.Output <- ansi(fmt.Sprintf("\r\n\r\n%s\r\n%s\r\nExits: %s",
		r.Title, r.Description, exitList(r)))
	others := world.listPlayers(true, p.Room)
	if len(others) > 1 { // include self in list, so >1 means someone else is here
		p.Output <- ansi(fmt.Sprintf("\r\nYou see: %s", strings.Join(filterOut(others, p.Name), ", ")))
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
		p.Output <- ansi("\r\nCommands:\r\n" +
			"  help               - show this message\r\n" +
			"  look               - describe your room\r\n" +
			"  say <msg>          - chat to the room\r\n" +
			"  emote <action>     - emote to the room (e.g. 'emote shrugs')\r\n" +
			"  who                - list connected players\r\n" +
			"  name <newname>     - change your display name\r\n" +
			"  go <n|s|e|w>       - move by direction\r\n" +
			"  n/s/e/w            - shorthand for movement\r\n" +
			"  quit               - disconnect")
	case "look", "l":
		r, _ := world.getRoom(p.Room)
		p.Output <- ansi(fmt.Sprintf("\r\n%s\r\n%s\r\nExits: %s",
			r.Title, r.Description, exitList(r)))
		others := world.listPlayers(true, p.Room)
		if len(others) > 1 {
			p.Output <- ansi(fmt.Sprintf("\r\nYou see: %s", strings.Join(filterOut(others, p.Name), ", ")))
		}
	case "say":
		if arg == "" {
			p.Output <- ansi("\r\nSay what?")
			return false
		}
		world.broadcastToRoom(p.Room, ansi(fmt.Sprintf("\r\n%s says: %s", p.Name, arg)), p)
		p.Output <- ansi(fmt.Sprintf("\r\nYou say: %s", arg))
	case "emote", ":":
		if arg == "" {
			p.Output <- ansi("\r\nEmote what?")
			return false
		}
		world.broadcastToRoom(p.Room, ansi(fmt.Sprintf("\r\n%s %s", p.Name, arg)), p)
		p.Output <- ansi(fmt.Sprintf("\r\nYou %s", arg))
	case "who":
		names := world.listPlayers(false, "")
		p.Output <- ansi("\r\nPlayers: " + strings.Join(names, ", "))
	case "name":
		newName := strings.TrimSpace(arg)
		if newName == "" {
			p.Output <- ansi("\r\nUsage: name <newname>")
			return false
		}
		if strings.ContainsAny(newName, " \t\r\n") || len(newName) > 24 {
			p.Output <- ansi("\r\nInvalid name.")
			return false
		}
		old := p.Name
		if err := world.renamePlayer(p, newName); err != nil {
			p.Output <- ansi("\r\n" + err.Error())
			return false
		}
		world.broadcastToRoom(p.Room, ansi(fmt.Sprintf("\r\n%s is now known as %s.", old, newName)), p)
		p.Output <- ansi(fmt.Sprintf("\r\nYou are now known as %s.", newName))
	case "go":
		dir := strings.ToLower(strings.TrimSpace(arg))
		if dir == "" {
			p.Output <- ansi("\r\nUsage: go <n|s|e|w>")
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
	world.broadcastToRoom(RoomID(prev), ansi(fmt.Sprintf("\r\n%s leaves %s.", p.Name, dir)), p)
	enterRoom(world, p, dir)
	_ = next
	return false
}

// ---------- Server ----------

func main() {
	addr := ":4000"
	world := NewWorld()

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		panic(err)
	}
	fmt.Printf("MUD listening on %s (telnet compatible)\n", addr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		// Basic telnet: turn off line mode & echo? Many clients work fine as-is.
		// Keep it simple; avoid complex TN negotiations in this minimal server.
		go handleConn(conn, world)
	}
}
