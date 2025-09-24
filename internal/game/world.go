package game

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

// DefaultAreasPath is the on-disk location of bundled areas.
const DefaultAreasPath = "data/areas"

type RoomID string

type Room struct {
	ID          RoomID            `json:"id"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Exits       map[string]RoomID `json:"exits"`
}

type Channel string

const (
	ChannelSay     Channel = "say"
	ChannelWhisper Channel = "whisper"
	ChannelYell    Channel = "yell"
	ChannelOOC     Channel = "ooc"
)

var allChannels = []Channel{ChannelSay, ChannelWhisper, ChannelYell, ChannelOOC}

var channelLookup = map[string]Channel{
	"say":     ChannelSay,
	"whisper": ChannelWhisper,
	"yell":    ChannelYell,
	"ooc":     ChannelOOC,
}

// AllChannels returns the set of available chat channels.
func AllChannels() []Channel {
	out := make([]Channel, len(allChannels))
	copy(out, allChannels)
	return out
}

// ChannelFromString resolves a textual channel name into the canonical identifier.
func ChannelFromString(name string) (Channel, bool) {
	channel, ok := channelLookup[strings.ToLower(name)]
	return channel, ok
}

type Player struct {
	Name     string
	Session  *TelnetSession
	Room     RoomID
	Output   chan string
	Alive    bool
	IsAdmin  bool
	Channels map[Channel]bool
}

type World struct {
	mu        sync.RWMutex
	rooms     map[RoomID]*Room
	players   map[string]*Player
	areasPath string
}

type AccountManager struct {
	mu    sync.RWMutex
	creds map[string]string
	path  string
}

func NewAccountManager(path string) (*AccountManager, error) {
	manager := &AccountManager{
		creds: make(map[string]string),
		path:  path,
	}
	if err := manager.load(); err != nil {
		return nil, err
	}
	return manager, nil
}

func (a *AccountManager) load() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	data, err := os.ReadFile(a.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read accounts file: %w", err)
	}
	if len(data) == 0 {
		a.creds = make(map[string]string)
		return nil
	}
	var creds map[string]string
	if err := json.Unmarshal(data, &creds); err != nil {
		return fmt.Errorf("decode accounts file: %w", err)
	}
	a.creds = creds
	return nil
}

func (a *AccountManager) saveLocked() error {
	dir := filepath.Dir(a.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create accounts directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, "accounts-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp accounts file: %w", err)
	}
	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(a.creds); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return fmt.Errorf("write accounts file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("close temp accounts file: %w", err)
	}
	if err := os.Rename(tmp.Name(), a.path); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("replace accounts file: %w", err)
	}
	return nil
}

func (a *AccountManager) Exists(name string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	_, ok := a.creds[name]
	return ok
}

func (a *AccountManager) Register(name, pass string) error {
	hashed, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.creds[name]; ok {
		return fmt.Errorf("account already exists")
	}
	a.creds[name] = string(hashed)
	if err := a.saveLocked(); err != nil {
		delete(a.creds, name)
		return err
	}
	return nil
}

func (a *AccountManager) Authenticate(name, pass string) bool {
	a.mu.RLock()
	hashed, ok := a.creds[name]
	a.mu.RUnlock()
	if !ok {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(pass)) == nil
}

func NewWorld(areasPath string) (*World, error) {
	rooms, err := loadRooms(areasPath)
	if err != nil {
		return nil, err
	}
	return &World{
		rooms:     rooms,
		players:   make(map[string]*Player),
		areasPath: areasPath,
	}, nil
}

// NewWorldWithRooms constructs a world populated with the provided rooms.
func NewWorldWithRooms(rooms map[RoomID]*Room) *World {
	return &World{
		rooms:   rooms,
		players: make(map[string]*Player),
	}
}

// AddPlayerForTest inserts a player into the world's tracking structures.
func (w *World) AddPlayerForTest(p *Player) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.players == nil {
		w.players = make(map[string]*Player)
	}
	w.players[p.Name] = p
}

type areaFile struct {
	Name  string `json:"name"`
	Rooms []Room `json:"rooms"`
}

func loadRooms(areasPath string) (map[RoomID]*Room, error) {
	entries, err := os.ReadDir(areasPath)
	if err != nil {
		return nil, fmt.Errorf("read areas: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	rooms := make(map[RoomID]*Room)
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(areasPath, name))
		if err != nil {
			return nil, fmt.Errorf("read area %s: %w", name, err)
		}
		var file areaFile
		if err := json.Unmarshal(data, &file); err != nil {
			return nil, fmt.Errorf("decode area %s: %w", name, err)
		}
		for i := range file.Rooms {
			room := file.Rooms[i]
			if room.ID == "" {
				return nil, fmt.Errorf("area %s contains a room without an id", name)
			}
			if room.Exits == nil {
				room.Exits = make(map[string]RoomID)
			}
			if _, exists := rooms[room.ID]; exists {
				return nil, fmt.Errorf("duplicate room id %s", room.ID)
			}
			r := room
			rooms[room.ID] = &r
		}
	}
	if len(rooms) == 0 {
		return nil, fmt.Errorf("no rooms loaded")
	}
	return rooms, nil
}

func defaultChannelSettings() map[Channel]bool {
	return map[Channel]bool{
		ChannelSay:     true,
		ChannelWhisper: true,
		ChannelYell:    true,
		ChannelOOC:     true,
	}
}

// DefaultChannelSettings exposes the default channel configuration.
func DefaultChannelSettings() map[Channel]bool {
	return map[Channel]bool{
		ChannelSay:     true,
		ChannelWhisper: true,
		ChannelYell:    true,
		ChannelOOC:     true,
	}
}

func (p *Player) channelEnabled(channel Channel) bool {
	if p.Channels == nil {
		return true
	}
	enabled, ok := p.Channels[channel]
	if !ok {
		return true
	}
	return enabled
}

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
		if existing.Channels == nil {
			existing.Channels = defaultChannelSettings()
		}
		return existing, nil
	}
	p := &Player{
		Name:     name,
		Session:  session,
		Room:     "start",
		Output:   make(chan string, 32),
		Alive:    true,
		IsAdmin:  isAdmin,
		Channels: defaultChannelSettings(),
	}
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

func (w *World) Reboot() ([]*Player, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.areasPath == "" {
		return nil, fmt.Errorf("world does not have an areas path configured")
	}
	rooms, err := loadRooms(w.areasPath)
	if err != nil {
		return nil, err
	}
	w.rooms = rooms
	revived := make([]*Player, 0, len(w.players))
	for _, p := range w.players {
		p.Room = "start"
		revived = append(revived, p)
	}
	return revived, nil
}

func (w *World) GetRoom(id RoomID) (*Room, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	r, ok := w.rooms[id]
	return r, ok
}

func (w *World) BroadcastToRoom(room RoomID, msg string, except *Player) {
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

func (w *World) BroadcastToRoomChannel(room RoomID, msg string, except *Player, channel Channel) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	for _, target := range w.players {
		if target.Room != room || target == except || !target.Alive {
			continue
		}
		if !target.channelEnabled(channel) {
			continue
		}
		select {
		case target.Output <- msg:
		default:
		}
	}
}

func (w *World) BroadcastToRoomsChannel(rooms []RoomID, msg string, except *Player, channel Channel) {
	if len(rooms) == 0 {
		return
	}
	roomSet := make(map[RoomID]struct{}, len(rooms))
	for _, room := range rooms {
		roomSet[room] = struct{}{}
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	for _, target := range w.players {
		if target == except || !target.Alive {
			continue
		}
		if _, ok := roomSet[target.Room]; !ok {
			continue
		}
		if !target.channelEnabled(channel) {
			continue
		}
		select {
		case target.Output <- msg:
		default:
		}
	}
}

func (w *World) BroadcastToAllChannel(msg string, except *Player, channel Channel) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	for _, target := range w.players {
		if target == except || !target.Alive {
			continue
		}
		if !target.channelEnabled(channel) {
			continue
		}
		select {
		case target.Output <- msg:
		default:
		}
	}
}

func (w *World) AdjacentRooms(room RoomID) []RoomID {
	w.mu.RLock()
	defer w.mu.RUnlock()
	current, ok := w.rooms[room]
	if !ok {
		return nil
	}
	seen := make(map[RoomID]struct{}, len(current.Exits))
	neighbors := make([]RoomID, 0, len(current.Exits))
	for _, next := range current.Exits {
		if _, ok := seen[next]; ok {
			continue
		}
		seen[next] = struct{}{}
		neighbors = append(neighbors, next)
	}
	return neighbors
}

func (w *World) SetChannel(p *Player, channel Channel, enabled bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, ok := w.players[p.Name]; !ok {
		return
	}
	if p.Channels == nil {
		p.Channels = defaultChannelSettings()
	}
	p.Channels[channel] = enabled
}

func (w *World) ChannelStatuses(p *Player) map[Channel]bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	statuses := make(map[Channel]bool, len(allChannels))
	for _, channel := range allChannels {
		statuses[channel] = p.channelEnabled(channel)
	}
	return statuses
}

func (w *World) RenamePlayer(p *Player, newName string) error {
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

func (w *World) ListPlayers(roomOnly bool, room RoomID) []string {
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

func (w *World) Move(p *Player, dir string) (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	r, ok := w.rooms[p.Room]
	if !ok {
		return "", fmt.Errorf("unknown room: %s", p.Room)
	}
	next, ok := r.Exits[dir]
	if !ok {
		return "", fmt.Errorf("you can't go that way")
	}
	p.Room = next
	return string(next), nil
}
