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
	"time"

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
	NPCs        []NPC             `json:"npcs"`
}

type NPC struct {
	Name      string `json:"name"`
	AutoGreet string `json:"auto_greet"`
}

// StartRoom is the default entry point for new players.
const StartRoom RoomID = "start"

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
	Name      string
	Account   string
	Session   *TelnetSession
	Room      RoomID
	Home      RoomID
	Output    chan string
	Alive     bool
	IsAdmin   bool
	IsBuilder bool
	Channels  map[Channel]bool
	history   []time.Time
}

// PlayerProfile captures persistent player state and preferences.
type PlayerProfile struct {
	Room     RoomID
	Home     RoomID
	Channels map[Channel]bool
}

const (
	commandLimit  = 5
	commandWindow = time.Second
)

func (p *Player) allowCommand(now time.Time) bool {
	cutoff := now.Add(-commandWindow)
	filtered := p.history[:0]
	for _, t := range p.history {
		if t.After(cutoff) {
			filtered = append(filtered, t)
		}
	}
	p.history = filtered
	if len(p.history) >= commandLimit {
		return false
	}
	p.history = append(p.history, now)
	return true
}

type World struct {
	mu        sync.RWMutex
	rooms     map[RoomID]*Room
	players   map[string]*Player
	areasPath string
	accounts  *AccountManager
}

type accountRecord struct {
	Password string          `json:"password"`
	Room     RoomID          `json:"room,omitempty"`
	Home     RoomID          `json:"home,omitempty"`
	Channels map[string]bool `json:"channels,omitempty"`
}

// PlayerLocation describes the room occupied by a connected player.
type PlayerLocation struct {
	Name string
	Room RoomID
}

type AccountManager struct {
	mu       sync.RWMutex
	accounts map[string]accountRecord
	path     string
}

func NewAccountManager(path string) (*AccountManager, error) {
	manager := &AccountManager{
		accounts: make(map[string]accountRecord),
		path:     path,
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
		a.accounts = make(map[string]accountRecord)
		return nil
	}
	if err != nil {
		return fmt.Errorf("read accounts file: %w", err)
	}
	if len(data) == 0 {
		a.accounts = make(map[string]accountRecord)
		return nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("decode accounts file: %w", err)
	}
	accounts := make(map[string]accountRecord, len(raw))
	for name, blob := range raw {
		var password string
		if err := json.Unmarshal(blob, &password); err == nil {
			accounts[name] = accountRecord{Password: password}
			continue
		}
		var record accountRecord
		if err := json.Unmarshal(blob, &record); err != nil {
			return fmt.Errorf("decode account %s: %w", name, err)
		}
		accounts[name] = record
	}
	a.accounts = accounts
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
	if err := enc.Encode(a.accounts); err != nil {
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
	_, ok := a.accounts[name]
	return ok
}

func (a *AccountManager) Register(name, pass string) error {
	hashed, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.accounts[name]; ok {
		return fmt.Errorf("account already exists")
	}
	a.accounts[name] = accountRecord{
		Password: string(hashed),
		Room:     StartRoom,
		Home:     StartRoom,
		Channels: encodeChannelSettings(defaultChannelSettings()),
	}
	if err := a.saveLocked(); err != nil {
		delete(a.accounts, name)
		return err
	}
	return nil
}

func (a *AccountManager) Authenticate(name, pass string) bool {
	a.mu.RLock()
	record, ok := a.accounts[name]
	a.mu.RUnlock()
	if !ok {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(record.Password), []byte(pass)) == nil
}

// Profile retrieves the persisted state for a player. Defaults are returned for
// unknown accounts.
func (a *AccountManager) Profile(name string) PlayerProfile {
	a.mu.RLock()
	defer a.mu.RUnlock()
	profile := PlayerProfile{
		Room:     StartRoom,
		Home:     StartRoom,
		Channels: defaultChannelSettings(),
	}
	record, ok := a.accounts[name]
	if !ok {
		return profile
	}
	if record.Room != "" {
		profile.Room = record.Room
	}
	if record.Home != "" {
		profile.Home = record.Home
	}
	if record.Channels != nil {
		profile.Channels = decodeChannelSettings(record.Channels)
	}
	return profile
}

// SaveProfile persists the provided state for the named account.
func (a *AccountManager) SaveProfile(name string, profile PlayerProfile) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	record, ok := a.accounts[name]
	if !ok {
		return fmt.Errorf("account not found")
	}
	record.Room = profile.Room
	record.Home = profile.Home
	record.Channels = encodeChannelSettings(profile.Channels)
	a.accounts[name] = record
	return a.saveLocked()
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

// AttachAccountManager wires the account persistence layer into the world.
func (w *World) AttachAccountManager(accounts *AccountManager) {
	w.mu.Lock()
	w.accounts = accounts
	w.mu.Unlock()
}

// AddPlayerForTest inserts a player into the world's tracking structures.
func (w *World) AddPlayerForTest(p *Player) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.players == nil {
		w.players = make(map[string]*Player)
	}
	if p.Account == "" {
		p.Account = p.Name
	}
	if p.Home == "" {
		if p.Room != "" {
			p.Home = p.Room
		} else {
			p.Home = StartRoom
		}
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

func cloneChannelSettings(settings map[Channel]bool) map[Channel]bool {
	if settings == nil {
		return nil
	}
	clone := make(map[Channel]bool, len(settings))
	for channel, enabled := range settings {
		clone[channel] = enabled
	}
	return clone
}

func encodeChannelSettings(settings map[Channel]bool) map[string]bool {
	if settings == nil {
		return nil
	}
	encoded := make(map[string]bool, len(settings))
	for channel, enabled := range settings {
		encoded[string(channel)] = enabled
	}
	return encoded
}

func decodeChannelSettings(raw map[string]bool) map[Channel]bool {
	settings := defaultChannelSettings()
	if len(raw) == 0 {
		return settings
	}
	for name, enabled := range raw {
		if channel, ok := channelLookup[name]; ok {
			settings[channel] = enabled
		}
	}
	return settings
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

func (w *World) addPlayer(name string, session *TelnetSession, isAdmin bool, profile PlayerProfile) (*Player, error) {
	room := profile.Room
	if room == "" {
		room = StartRoom
	}
	home := profile.Home
	if home == "" {
		home = StartRoom
	}
	channels := profile.Channels
	if channels == nil {
		channels = defaultChannelSettings()
	}

	w.mu.Lock()
	if existing, ok := w.players[name]; ok {
		if existing.Alive {
			w.mu.Unlock()
			return nil, fmt.Errorf("%s is already connected", name)
		}
		existing.Session = session
		existing.Output = make(chan string, 32)
		existing.Room = room
		existing.Home = home
		existing.Alive = true
		existing.IsAdmin = isAdmin
		existing.Account = name
		existing.Channels = cloneChannelSettings(channels)
		persistChannels := cloneChannelSettings(existing.Channels)
		account := existing.Account
		currentRoom := existing.Room
		currentHome := existing.Home
		w.mu.Unlock()
		w.persistPlayerState(account, currentRoom, currentHome, persistChannels)
		return existing, nil
	}

	playerChannels := cloneChannelSettings(channels)
	p := &Player{
		Name:      name,
		Account:   name,
		Session:   session,
		Room:      room,
		Home:      home,
		Output:    make(chan string, 32),
		Alive:     true,
		IsAdmin:   isAdmin,
		IsBuilder: false,
		Channels:  cloneChannelSettings(playerChannels),
	}
	w.players[name] = p
	persistChannels := cloneChannelSettings(playerChannels)
	account := p.Account
	currentRoom := p.Room
	currentHome := p.Home
	w.mu.Unlock()
	w.persistPlayerState(account, currentRoom, currentHome, persistChannels)
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
		p.Room = StartRoom
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
	if _, ok := w.players[p.Name]; !ok {
		w.mu.Unlock()
		return
	}
	if p.Channels == nil {
		p.Channels = defaultChannelSettings()
	}
	p.Channels[channel] = enabled
	channels := cloneChannelSettings(p.Channels)
	account := p.Account
	room := p.Room
	home := p.Home
	w.mu.Unlock()
	w.persistPlayerState(account, room, home, channels)
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

func (w *World) persistPlayerState(account string, room, home RoomID, channels map[Channel]bool) {
	if account == "" {
		return
	}
	accounts := w.accounts
	if accounts == nil {
		return
	}
	profile := PlayerProfile{Room: room, Home: home, Channels: channels}
	if err := accounts.SaveProfile(account, profile); err != nil {
		fmt.Printf("failed to persist state for %s: %v\n", account, err)
	}
}

// PersistPlayer flushes the current state for the player to the backing store.
func (w *World) PersistPlayer(p *Player) {
	if p == nil {
		return
	}
	w.mu.RLock()
	account := p.Account
	room := p.Room
	home := p.Home
	channels := cloneChannelSettings(p.Channels)
	w.mu.RUnlock()
	w.persistPlayerState(account, room, home, channels)
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
	r, ok := w.rooms[p.Room]
	if !ok {
		w.mu.Unlock()
		return "", fmt.Errorf("unknown room: %s", p.Room)
	}
	next, ok := r.Exits[dir]
	if !ok {
		w.mu.Unlock()
		return "", fmt.Errorf("you can't go that way")
	}
	p.Room = next
	channels := cloneChannelSettings(p.Channels)
	account := p.Account
	home := p.Home
	w.mu.Unlock()
	w.persistPlayerState(account, next, home, channels)
	return string(next), nil
}

func (w *World) findPlayerLocked(name string) (*Player, bool) {
	if p, ok := w.players[name]; ok && p.Alive {
		return p, true
	}
	lower := strings.ToLower(name)
	for _, p := range w.players {
		if !p.Alive {
			continue
		}
		if strings.ToLower(p.Name) == lower {
			return p, true
		}
	}
	return nil, false
}

// FindPlayer locates an online player by name, performing a case-insensitive match.
func (w *World) FindPlayer(name string) (*Player, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	p, ok := w.findPlayerLocked(name)
	if !ok {
		return nil, false
	}
	return p, true
}

// SetBuilder toggles the builder flag for a connected player.
func (w *World) SetBuilder(name string, enabled bool) (*Player, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	p, ok := w.findPlayerLocked(name)
	if !ok {
		return nil, fmt.Errorf("%s is not online", name)
	}
	p.IsBuilder = enabled
	return p, nil
}

// MoveToRoom relocates the provided player to the specified room.
func (w *World) MoveToRoom(p *Player, room RoomID) error {
	w.mu.Lock()
	if _, ok := w.rooms[room]; !ok {
		w.mu.Unlock()
		return fmt.Errorf("unknown room: %s", room)
	}
	stored, ok := w.players[p.Name]
	if !ok || stored != p || !p.Alive {
		w.mu.Unlock()
		return fmt.Errorf("%s is not online", p.Name)
	}
	p.Room = room
	account := p.Account
	home := p.Home
	channels := cloneChannelSettings(p.Channels)
	w.mu.Unlock()
	w.persistPlayerState(account, room, home, channels)
	return nil
}

// SetHome updates the player's recall location and persists it.
func (w *World) SetHome(p *Player, room RoomID) error {
	w.mu.Lock()
	if _, ok := w.rooms[room]; !ok {
		w.mu.Unlock()
		return fmt.Errorf("unknown room: %s", room)
	}
	stored, ok := w.players[p.Name]
	if !ok || stored != p || !p.Alive {
		w.mu.Unlock()
		return fmt.Errorf("%s is not online", p.Name)
	}
	p.Home = room
	account := p.Account
	channels := cloneChannelSettings(p.Channels)
	currentRoom := p.Room
	w.mu.Unlock()
	w.persistPlayerState(account, currentRoom, room, channels)
	return nil
}

// PlayerLocations returns the set of connected players and their rooms sorted by name.
func (w *World) PlayerLocations() []PlayerLocation {
	w.mu.RLock()
	defer w.mu.RUnlock()
	locations := make([]PlayerLocation, 0, len(w.players))
	for _, p := range w.players {
		if !p.Alive {
			continue
		}
		locations = append(locations, PlayerLocation{Name: p.Name, Room: p.Room})
	}
	sort.Slice(locations, func(i, j int) bool {
		return locations[i].Name < locations[j].Name
	})
	return locations
}
