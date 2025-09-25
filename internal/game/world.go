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
)

// DefaultAreasPath is the on-disk location of bundled areas.
const DefaultAreasPath = "data/areas"

// builderAreaFile stores rooms created or modified in-game.
const builderAreaFile = "builder.json"

type RoomID string

type Room struct {
	ID          RoomID            `json:"id"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Exits       map[string]RoomID `json:"exits"`
	NPCs        []NPC             `json:"npcs"`
	Items       []Item            `json:"items"`
	Resets      []RoomReset       `json:"resets,omitempty"`
}

type NPC struct {
	Name      string `json:"name"`
	AutoGreet string `json:"auto_greet"`
}

// ResetKind identifies the type of entity governed by a room reset.
type ResetKind string

const (
	ResetKindNPC  ResetKind = "npc"
	ResetKindItem ResetKind = "item"
)

// RoomReset describes how a room repopulates persistent content.
type RoomReset struct {
	Kind        ResetKind `json:"kind"`
	Name        string    `json:"name"`
	Count       int       `json:"count,omitempty"`
	AutoGreet   string    `json:"auto_greet,omitempty"`
	Description string    `json:"description,omitempty"`
}

// Item represents an object that can exist in rooms or player inventories.
type Item struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// StartRoom is the default entry point for new players.
const StartRoom RoomID = "start"

var (
	// ErrItemNotFound indicates a requested item could not be located.
	ErrItemNotFound = errors.New("item not found")
	// ErrItemNotCarried indicates the player is not carrying the requested item.
	ErrItemNotCarried = errors.New("item not carried")
)

type World struct {
	mu          sync.RWMutex
	rooms       map[RoomID]*Room
	players     map[string]*Player
	playerOrder []string
	areasPath   string
	accounts    *AccountManager
	mail        *MailSystem
	roomSources map[RoomID]string
	builderPath string
}

// PlayerLocation describes the room occupied by a connected player.
type PlayerLocation struct {
	Name string
	Room RoomID
}

func NewWorld(areasPath string) (*World, error) {
	rooms, sources, err := loadRooms(areasPath)
	if err != nil {
		return nil, err
	}
	return &World{
		rooms:       rooms,
		players:     make(map[string]*Player),
		playerOrder: make([]string, 0),
		areasPath:   areasPath,
		roomSources: sources,
		builderPath: filepath.Join(areasPath, builderAreaFile),
	}, nil
}

// NewWorldWithRooms constructs a world populated with the provided rooms.
func NewWorldWithRooms(rooms map[RoomID]*Room) *World {
	return &World{
		rooms:       rooms,
		players:     make(map[string]*Player),
		playerOrder: make([]string, 0),
		roomSources: make(map[RoomID]string, len(rooms)),
	}
}

// AttachAccountManager wires the account persistence layer into the world.
func (w *World) AttachAccountManager(accounts *AccountManager) {
	w.mu.Lock()
	w.accounts = accounts
	w.mu.Unlock()
}

// AttachMailSystem connects the persistent mail board storage to the world.
func (w *World) AttachMailSystem(mail *MailSystem) {
	w.mu.Lock()
	w.mail = mail
	w.mu.Unlock()
}

// MailSystem exposes the shared mail manager, when configured.
func (w *World) MailSystem() *MailSystem {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.mail
}

// AccountStats exposes account metadata for the provided name.
func (w *World) AccountStats(name string) (AccountStats, bool) {
	w.mu.RLock()
	accounts := w.accounts
	w.mu.RUnlock()
	if accounts == nil {
		return AccountStats{}, false
	}
	return accounts.Stats(name)
}

// AddPlayerForTest inserts a player into the world's tracking structures.
func (w *World) AddPlayerForTest(p *Player) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.players == nil {
		w.players = make(map[string]*Player)
	}
	if w.playerOrder == nil {
		w.playerOrder = make([]string, 0, len(w.players)+1)
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
	now := time.Now()
	p.JoinedAt = now
	w.players[p.Name] = p
	w.removePlayerOrderLocked(p.Name)
	w.playerOrder = append(w.playerOrder, p.Name)
}

type areaFile struct {
	Name  string `json:"name"`
	Rooms []Room `json:"rooms"`
}

func loadRooms(areasPath string) (map[RoomID]*Room, map[RoomID]string, error) {
	entries, err := os.ReadDir(areasPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read areas: %w", err)
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
	sources := make(map[RoomID]string)
	var builderFileName string
	for _, name := range names {
		if name == builderAreaFile {
			builderFileName = name
			continue
		}
		if err := loadAreaFile(areasPath, name, rooms, sources, false); err != nil {
			return nil, nil, err
		}
	}
	if builderFileName != "" {
		if err := loadAreaFile(areasPath, builderFileName, rooms, sources, true); err != nil {
			return nil, nil, err
		}
	}
	if len(rooms) == 0 {
		return nil, nil, fmt.Errorf("no rooms loaded")
	}
	return rooms, sources, nil
}

func loadAreaFile(areasPath, name string, rooms map[RoomID]*Room, sources map[RoomID]string, allowOverride bool) error {
	data, err := os.ReadFile(filepath.Join(areasPath, name))
	if err != nil {
		return fmt.Errorf("read area %s: %w", name, err)
	}
	var file areaFile
	if err := json.Unmarshal(data, &file); err != nil {
		return fmt.Errorf("decode area %s: %w", name, err)
	}
	for i := range file.Rooms {
		room := file.Rooms[i]
		if room.ID == "" {
			return fmt.Errorf("area %s contains a room without an id", name)
		}
		if room.Exits == nil {
			room.Exits = make(map[string]RoomID)
		}
		if _, exists := rooms[room.ID]; exists && !allowOverride {
			return fmt.Errorf("duplicate room id %s", room.ID)
		}
		r := room
		rooms[room.ID] = &r
		sources[room.ID] = name
	}
	return nil
}

func (w *World) markRoomAsBuilderLocked(id RoomID) (string, bool) {
	if w.roomSources == nil {
		w.roomSources = make(map[RoomID]string)
	}
	prev, existed := w.roomSources[id]
	w.roomSources[id] = builderAreaFile
	return prev, existed
}

func (w *World) setExitLocked(roomID RoomID, direction string, target *RoomID) (func(), error) {
	room, ok := w.rooms[roomID]
	if !ok {
		return nil, fmt.Errorf("unknown room: %s", roomID)
	}
	if target != nil {
		if _, ok := w.rooms[*target]; !ok {
			return nil, fmt.Errorf("unknown room: %s", *target)
		}
	}
	var prevTarget RoomID
	hadExit := false
	if room.Exits != nil {
		prevTarget, hadExit = room.Exits[direction]
	}
	if target == nil {
		if room.Exits != nil {
			delete(room.Exits, direction)
		}
	} else {
		if room.Exits == nil {
			room.Exits = make(map[string]RoomID)
		}
		room.Exits[direction] = *target
	}
	prevSource, hadSource := w.markRoomAsBuilderLocked(roomID)
	undo := func() {
		if hadExit {
			if room.Exits == nil {
				room.Exits = make(map[string]RoomID)
			}
			room.Exits[direction] = prevTarget
		} else if room.Exits != nil {
			delete(room.Exits, direction)
		}
		if hadSource {
			w.roomSources[roomID] = prevSource
		} else {
			delete(w.roomSources, roomID)
		}
	}
	return undo, nil
}

func (w *World) persistBuilderRoomsLocked() error {
	if w.builderPath == "" {
		return nil
	}
	rooms := make([]Room, 0, len(w.roomSources))
	for id, source := range w.roomSources {
		if source != builderAreaFile {
			continue
		}
		room, ok := w.rooms[id]
		if !ok {
			continue
		}
		copyRoom := *room
		copyRoom.ID = id
		if room.Exits == nil {
			copyRoom.Exits = make(map[string]RoomID)
		} else {
			copyRoom.Exits = cloneExits(room.Exits)
		}
		if room.NPCs != nil {
			npcs := make([]NPC, len(room.NPCs))
			copy(npcs, room.NPCs)
			copyRoom.NPCs = npcs
		}
		if room.Items != nil {
			items := make([]Item, len(room.Items))
			copy(items, room.Items)
			copyRoom.Items = items
		}
		if room.Resets != nil {
			resets := make([]RoomReset, len(room.Resets))
			copy(resets, room.Resets)
			copyRoom.Resets = resets
		}
		rooms = append(rooms, copyRoom)
	}
	sort.Slice(rooms, func(i, j int) bool {
		return rooms[i].ID < rooms[j].ID
	})
	file := areaFile{Name: "Builder Rooms", Rooms: rooms}
	dir := filepath.Dir(w.builderPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create builder area directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, "builder-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp builder area file: %w", err)
	}
	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(file); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return fmt.Errorf("write builder area: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("close builder area: %w", err)
	}
	if err := os.Rename(tmp.Name(), w.builderPath); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("replace builder area: %w", err)
	}
	return nil
}

func cloneExits(exits map[string]RoomID) map[string]RoomID {
	if exits == nil {
		return nil
	}
	clone := make(map[string]RoomID, len(exits))
	for dir, dest := range exits {
		clone[dir] = dest
	}
	return clone
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
	now := time.Now()
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
		existing.JoinedAt = now
		w.removePlayerOrderLocked(name)
		w.playerOrder = append(w.playerOrder, name)
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
		JoinedAt:  now,
	}
	w.players[name] = p
	w.removePlayerOrderLocked(name)
	w.playerOrder = append(w.playerOrder, name)
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
		w.removePlayerOrderLocked(name)
		close(p.Output)
	}
}

func (w *World) Reboot() ([]*Player, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.areasPath == "" {
		return nil, fmt.Errorf("world does not have an areas path configured")
	}
	rooms, sources, err := loadRooms(w.areasPath)
	if err != nil {
		return nil, err
	}
	w.rooms = rooms
	w.roomSources = sources
	if w.areasPath != "" {
		w.builderPath = filepath.Join(w.areasPath, builderAreaFile)
	}
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
	oldName := p.Name
	delete(w.players, p.Name)
	p.Name = newName
	w.players[newName] = p
	w.replacePlayerOrderLocked(oldName, newName)
	return nil
}

func (w *World) ListPlayers(roomOnly bool, room RoomID) []string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	names := make([]string, 0, len(w.playerOrder))
	seen := make(map[string]struct{}, len(w.playerOrder))
	for _, name := range w.playerOrder {
		p, ok := w.players[name]
		if !ok {
			continue
		}
		if !p.Alive {
			continue
		}
		if roomOnly && p.Room != room {
			continue
		}
		names = append(names, p.Name)
		seen[p.Name] = struct{}{}
	}
	if len(seen) != len(w.players) {
		for _, p := range w.players {
			if !p.Alive {
				continue
			}
			if roomOnly && p.Room != room {
				continue
			}
			if _, ok := seen[p.Name]; ok {
				continue
			}
			names = append(names, p.Name)
		}
	}
	return names
}

func findItemIndex(items []Item, target string) int {
	if target == "" {
		return -1
	}
	names := make([]string, len(items))
	for i, item := range items {
		names[i] = item.Name
	}
	idx, ok := uniqueMatch(target, names, true)
	if !ok {
		return -1
	}
	return idx
}

func findNPCIndex(npcs []NPC, target string) int {
	if target == "" {
		return -1
	}
	names := make([]string, len(npcs))
	for i, npc := range npcs {
		names[i] = npc.Name
	}
	idx, ok := uniqueMatch(target, names, true)
	if !ok {
		return -1
	}
	return idx
}

func findResetIndex(resets []RoomReset, kind ResetKind, target string) int {
	if target == "" {
		return -1
	}
	candidates := make([]string, 0, len(resets))
	indexes := make([]int, 0, len(resets))
	for i, reset := range resets {
		if reset.Kind != kind {
			continue
		}
		candidates = append(candidates, reset.Name)
		indexes = append(indexes, i)
	}
	idx, ok := uniqueMatch(target, candidates, true)
	if !ok {
		return -1
	}
	return indexes[idx]
}

// RoomItems returns a copy of the items present in the specified room.
func (w *World) RoomItems(room RoomID) []Item {
	w.mu.RLock()
	defer w.mu.RUnlock()
	r, ok := w.rooms[room]
	if !ok || len(r.Items) == 0 {
		return nil
	}
	items := make([]Item, len(r.Items))
	copy(items, r.Items)
	return items
}

// RoomResets returns a copy of the reset definitions for the specified room.
func (w *World) RoomResets(room RoomID) []RoomReset {
	w.mu.RLock()
	defer w.mu.RUnlock()
	r, ok := w.rooms[room]
	if !ok || len(r.Resets) == 0 {
		return nil
	}
	resets := make([]RoomReset, len(r.Resets))
	copy(resets, r.Resets)
	return resets
}

// PlayerInventory returns a copy of the player's carried items.
func (w *World) PlayerInventory(p *Player) []Item {
	w.mu.RLock()
	defer w.mu.RUnlock()
	stored, ok := w.players[p.Name]
	if !ok || stored != p || len(stored.Inventory) == 0 {
		return nil
	}
	inv := make([]Item, len(stored.Inventory))
	copy(inv, stored.Inventory)
	return inv
}

// TakeItem moves an item from the player's current room into their inventory.
func (w *World) TakeItem(p *Player, name string) (*Item, error) {
	target := strings.TrimSpace(name)
	if target == "" {
		return nil, fmt.Errorf("item name must not be empty")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	stored, ok := w.players[p.Name]
	if !ok || stored != p || !p.Alive {
		return nil, fmt.Errorf("%s is not online", p.Name)
	}
	room, ok := w.rooms[p.Room]
	if !ok {
		return nil, fmt.Errorf("unknown room: %s", p.Room)
	}
	idx := findItemIndex(room.Items, target)
	if idx == -1 {
		return nil, ErrItemNotFound
	}
	item := room.Items[idx]
	room.Items = append(room.Items[:idx], room.Items[idx+1:]...)
	p.Inventory = append(p.Inventory, item)
	return &item, nil
}

// DropItem places an item from the player's inventory into their current room.
func (w *World) DropItem(p *Player, name string) (*Item, error) {
	target := strings.TrimSpace(name)
	if target == "" {
		return nil, fmt.Errorf("item name must not be empty")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	stored, ok := w.players[p.Name]
	if !ok || stored != p || !p.Alive {
		return nil, fmt.Errorf("%s is not online", p.Name)
	}
	room, ok := w.rooms[p.Room]
	if !ok {
		return nil, fmt.Errorf("unknown room: %s", p.Room)
	}
	idx := findItemIndex(p.Inventory, target)
	if idx == -1 {
		return nil, ErrItemNotCarried
	}
	item := p.Inventory[idx]
	p.Inventory = append(p.Inventory[:idx], p.Inventory[idx+1:]...)
	room.Items = append(room.Items, item)
	return &item, nil
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
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return nil, false
	}
	if p, ok := w.players[trimmed]; ok && p.Alive {
		return p, true
	}
	candidates := make([]*Player, 0, len(w.players))
	names := make([]string, 0, len(w.players))
	for _, p := range w.players {
		if !p.Alive {
			continue
		}
		candidates = append(candidates, p)
		names = append(names, p.Name)
	}
	idx, ok := uniqueMatch(trimmed, names, false)
	if !ok {
		return nil, false
	}
	return candidates[idx], true
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

// CreateRoom adds a new room to the world and persists it to the builder area.
func (w *World) CreateRoom(id RoomID, title string) (*Room, error) {
	trimmed := strings.TrimSpace(string(id))
	if trimmed == "" {
		return nil, fmt.Errorf("room id must not be empty")
	}
	normalizedID := RoomID(trimmed)
	w.mu.Lock()
	if _, exists := w.rooms[normalizedID]; exists {
		w.mu.Unlock()
		return nil, fmt.Errorf("room %s already exists", normalizedID)
	}
	if title = strings.TrimSpace(title); title == "" {
		title = trimmed
	}
	room := &Room{
		ID:          normalizedID,
		Title:       title,
		Description: "",
		Exits:       make(map[string]RoomID),
	}
	if w.rooms == nil {
		w.rooms = make(map[RoomID]*Room)
	}
	w.rooms[normalizedID] = room
	prevSource, hadSource := w.markRoomAsBuilderLocked(normalizedID)
	if err := w.persistBuilderRoomsLocked(); err != nil {
		if hadSource {
			w.roomSources[normalizedID] = prevSource
		} else {
			delete(w.roomSources, normalizedID)
		}
		delete(w.rooms, normalizedID)
		w.mu.Unlock()
		return nil, err
	}
	w.mu.Unlock()
	return room, nil
}

// UpdateRoomDescription modifies a room's description and persists the change.
func (w *World) UpdateRoomDescription(id RoomID, description string) (*Room, error) {
	w.mu.Lock()
	room, ok := w.rooms[id]
	if !ok {
		w.mu.Unlock()
		return nil, fmt.Errorf("unknown room: %s", id)
	}
	prevDesc := room.Description
	prevSource, hadSource := w.markRoomAsBuilderLocked(id)
	room.Description = description
	if err := w.persistBuilderRoomsLocked(); err != nil {
		room.Description = prevDesc
		if hadSource {
			w.roomSources[id] = prevSource
		} else {
			delete(w.roomSources, id)
		}
		w.mu.Unlock()
		return nil, err
	}
	w.mu.Unlock()
	return room, nil
}

// SetExit updates (or creates) an exit from one room to another.
func (w *World) SetExit(from RoomID, direction string, to RoomID) error {
	dir := strings.ToLower(strings.TrimSpace(direction))
	if dir == "" {
		return fmt.Errorf("direction must not be empty")
	}
	target := to
	w.mu.Lock()
	undo, err := w.setExitLocked(from, dir, &target)
	if err != nil {
		w.mu.Unlock()
		return err
	}
	if err := w.persistBuilderRoomsLocked(); err != nil {
		undo()
		w.mu.Unlock()
		return err
	}
	w.mu.Unlock()
	return nil
}

// ClearExit removes an exit from the specified room.
func (w *World) ClearExit(from RoomID, direction string) error {
	dir := strings.ToLower(strings.TrimSpace(direction))
	if dir == "" {
		return fmt.Errorf("direction must not be empty")
	}
	w.mu.Lock()
	undo, err := w.setExitLocked(from, dir, nil)
	if err != nil {
		w.mu.Unlock()
		return err
	}
	if err := w.persistBuilderRoomsLocked(); err != nil {
		undo()
		w.mu.Unlock()
		return err
	}
	w.mu.Unlock()
	return nil
}

// LinkRooms wires exits between two rooms, optionally adding a return path.
func (w *World) LinkRooms(from RoomID, direction string, to RoomID, back string) error {
	dir := strings.ToLower(strings.TrimSpace(direction))
	if dir == "" {
		return fmt.Errorf("direction must not be empty")
	}
	reverse := strings.ToLower(strings.TrimSpace(back))
	w.mu.Lock()
	undoForward, err := w.setExitLocked(from, dir, &to)
	if err != nil {
		w.mu.Unlock()
		return err
	}
	undos := []func(){undoForward}
	if reverse != "" {
		undoBack, err := w.setExitLocked(to, reverse, &from)
		if err != nil {
			for _, undo := range undos {
				undo()
			}
			w.mu.Unlock()
			return err
		}
		undos = append(undos, undoBack)
	}
	if err := w.persistBuilderRoomsLocked(); err != nil {
		for _, undo := range undos {
			undo()
		}
		w.mu.Unlock()
		return err
	}
	w.mu.Unlock()
	return nil
}

// UpsertRoomNPC creates or updates an NPC reset for the specified room.
func (w *World) UpsertRoomNPC(roomID RoomID, name, autoGreet string) (*NPC, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return nil, fmt.Errorf("npc name must not be empty")
	}
	greet := strings.TrimSpace(autoGreet)
	w.mu.Lock()
	room, ok := w.rooms[roomID]
	if !ok {
		w.mu.Unlock()
		return nil, fmt.Errorf("unknown room: %s", roomID)
	}
	prevNPCs := append([]NPC(nil), room.NPCs...)
	prevResets := append([]RoomReset(nil), room.Resets...)
	npc := NPC{Name: trimmed, AutoGreet: greet}
	idx := findNPCIndex(room.NPCs, trimmed)
	if idx >= 0 {
		room.NPCs[idx] = npc
	} else {
		room.NPCs = append(room.NPCs, npc)
	}
	resetIdx := findResetIndex(room.Resets, ResetKindNPC, trimmed)
	if resetIdx >= 0 {
		room.Resets[resetIdx].Name = trimmed
		room.Resets[resetIdx].AutoGreet = greet
		if room.Resets[resetIdx].Count < 1 {
			room.Resets[resetIdx].Count = 1
		}
	} else {
		room.Resets = append(room.Resets, RoomReset{Kind: ResetKindNPC, Name: trimmed, AutoGreet: greet, Count: 1})
	}
	prevSource, hadSource := w.markRoomAsBuilderLocked(roomID)
	if err := w.persistBuilderRoomsLocked(); err != nil {
		room.NPCs = prevNPCs
		room.Resets = prevResets
		if hadSource {
			w.roomSources[roomID] = prevSource
		} else {
			delete(w.roomSources, roomID)
		}
		w.mu.Unlock()
		return nil, err
	}
	w.mu.Unlock()
	return &npc, nil
}

// RemoveRoomNPC deletes an NPC definition and associated reset from a room.
func (w *World) RemoveRoomNPC(roomID RoomID, name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return fmt.Errorf("npc name must not be empty")
	}
	w.mu.Lock()
	room, ok := w.rooms[roomID]
	if !ok {
		w.mu.Unlock()
		return fmt.Errorf("unknown room: %s", roomID)
	}
	idx := findNPCIndex(room.NPCs, trimmed)
	if idx == -1 {
		w.mu.Unlock()
		return fmt.Errorf("npc %s not found", trimmed)
	}
	prevNPCs := append([]NPC(nil), room.NPCs...)
	prevResets := append([]RoomReset(nil), room.Resets...)
	room.NPCs = append(room.NPCs[:idx], room.NPCs[idx+1:]...)
	resetIdx := findResetIndex(room.Resets, ResetKindNPC, trimmed)
	if resetIdx >= 0 {
		room.Resets = append(room.Resets[:resetIdx], room.Resets[resetIdx+1:]...)
	}
	prevSource, hadSource := w.markRoomAsBuilderLocked(roomID)
	if err := w.persistBuilderRoomsLocked(); err != nil {
		room.NPCs = prevNPCs
		room.Resets = prevResets
		if hadSource {
			w.roomSources[roomID] = prevSource
		} else {
			delete(w.roomSources, roomID)
		}
		w.mu.Unlock()
		return err
	}
	w.mu.Unlock()
	return nil
}

// UpsertRoomItemReset creates or updates an item reset for a room.
func (w *World) UpsertRoomItemReset(roomID RoomID, name, description string) (*RoomReset, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return nil, fmt.Errorf("item name must not be empty")
	}
	desc := strings.TrimSpace(description)
	w.mu.Lock()
	room, ok := w.rooms[roomID]
	if !ok {
		w.mu.Unlock()
		return nil, fmt.Errorf("unknown room: %s", roomID)
	}
	prevItems := append([]Item(nil), room.Items...)
	prevResets := append([]RoomReset(nil), room.Resets...)
	idx := findResetIndex(room.Resets, ResetKindItem, trimmed)
	if idx >= 0 {
		room.Resets[idx].Name = trimmed
		room.Resets[idx].Description = desc
		if room.Resets[idx].Count < 1 {
			room.Resets[idx].Count = 1
		}
	} else {
		room.Resets = append(room.Resets, RoomReset{Kind: ResetKindItem, Name: trimmed, Description: desc, Count: 1})
		idx = len(room.Resets) - 1
	}
	w.applyRoomResetsLocked(room)
	result := room.Resets[idx]
	prevSource, hadSource := w.markRoomAsBuilderLocked(roomID)
	if err := w.persistBuilderRoomsLocked(); err != nil {
		room.Items = prevItems
		room.Resets = prevResets
		if hadSource {
			w.roomSources[roomID] = prevSource
		} else {
			delete(w.roomSources, roomID)
		}
		w.mu.Unlock()
		return nil, err
	}
	w.mu.Unlock()
	return &result, nil
}

// RemoveRoomItemReset deletes an item reset and any matching items from a room.
func (w *World) RemoveRoomItemReset(roomID RoomID, name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return fmt.Errorf("item name must not be empty")
	}
	w.mu.Lock()
	room, ok := w.rooms[roomID]
	if !ok {
		w.mu.Unlock()
		return fmt.Errorf("unknown room: %s", roomID)
	}
	resetIdx := findResetIndex(room.Resets, ResetKindItem, trimmed)
	if resetIdx == -1 {
		w.mu.Unlock()
		return fmt.Errorf("item %s not found", trimmed)
	}
	prevItems := append([]Item(nil), room.Items...)
	prevResets := append([]RoomReset(nil), room.Resets...)
	room.Resets = append(room.Resets[:resetIdx], room.Resets[resetIdx+1:]...)
	filtered := room.Items[:0]
	for _, item := range room.Items {
		if strings.EqualFold(item.Name, trimmed) {
			continue
		}
		filtered = append(filtered, item)
	}
	room.Items = filtered
	prevSource, hadSource := w.markRoomAsBuilderLocked(roomID)
	if err := w.persistBuilderRoomsLocked(); err != nil {
		room.Items = prevItems
		room.Resets = prevResets
		if hadSource {
			w.roomSources[roomID] = prevSource
		} else {
			delete(w.roomSources, roomID)
		}
		w.mu.Unlock()
		return err
	}
	w.mu.Unlock()
	return nil
}

// ApplyRoomResets enforces the configured resets for a room.
func (w *World) ApplyRoomResets(roomID RoomID) error {
	w.mu.Lock()
	room, ok := w.rooms[roomID]
	if !ok {
		w.mu.Unlock()
		return fmt.Errorf("unknown room: %s", roomID)
	}
	prevItems := append([]Item(nil), room.Items...)
	prevNPCs := append([]NPC(nil), room.NPCs...)
	prevResets := append([]RoomReset(nil), room.Resets...)
	w.applyRoomResetsLocked(room)
	prevSource, hadSource := w.markRoomAsBuilderLocked(roomID)
	if err := w.persistBuilderRoomsLocked(); err != nil {
		room.Items = prevItems
		room.NPCs = prevNPCs
		room.Resets = prevResets
		if hadSource {
			w.roomSources[roomID] = prevSource
		} else {
			delete(w.roomSources, roomID)
		}
		w.mu.Unlock()
		return err
	}
	w.mu.Unlock()
	return nil
}

// CloneRoomPopulation copies NPCs, items, and resets from one room into another.
func (w *World) CloneRoomPopulation(source, target RoomID) error {
	if source == "" {
		return fmt.Errorf("source room must not be empty")
	}
	w.mu.Lock()
	from, ok := w.rooms[source]
	if !ok {
		w.mu.Unlock()
		return fmt.Errorf("unknown room: %s", source)
	}
	to, ok := w.rooms[target]
	if !ok {
		w.mu.Unlock()
		return fmt.Errorf("unknown room: %s", target)
	}
	prevItems := append([]Item(nil), to.Items...)
	prevNPCs := append([]NPC(nil), to.NPCs...)
	prevResets := append([]RoomReset(nil), to.Resets...)

	if len(from.Items) > 0 {
		items := make([]Item, len(from.Items))
		copy(items, from.Items)
		to.Items = items
	} else {
		to.Items = nil
	}
	if len(from.NPCs) > 0 {
		npcs := make([]NPC, len(from.NPCs))
		copy(npcs, from.NPCs)
		to.NPCs = npcs
	} else {
		to.NPCs = nil
	}
	if len(from.Resets) > 0 {
		resets := make([]RoomReset, len(from.Resets))
		copy(resets, from.Resets)
		to.Resets = resets
	} else {
		to.Resets = nil
	}
	w.applyRoomResetsLocked(to)
	prevSource, hadSource := w.markRoomAsBuilderLocked(target)
	if err := w.persistBuilderRoomsLocked(); err != nil {
		to.Items = prevItems
		to.NPCs = prevNPCs
		to.Resets = prevResets
		if hadSource {
			w.roomSources[target] = prevSource
		} else {
			delete(w.roomSources, target)
		}
		w.mu.Unlock()
		return err
	}
	w.mu.Unlock()
	return nil
}

func (w *World) applyRoomResetsLocked(room *Room) {
	if room == nil {
		return
	}
	for i := range room.Resets {
		reset := room.Resets[i]
		if reset.Count < 1 {
			reset.Count = 1
			room.Resets[i].Count = 1
		}
		switch reset.Kind {
		case ResetKindNPC:
			npc := NPC{Name: reset.Name, AutoGreet: reset.AutoGreet}
			idx := findNPCIndex(room.NPCs, reset.Name)
			if idx >= 0 {
				room.NPCs[idx] = npc
			} else {
				room.NPCs = append(room.NPCs, npc)
			}
		case ResetKindItem:
			existing := 0
			for j := range room.Items {
				if strings.EqualFold(room.Items[j].Name, reset.Name) {
					existing++
					if reset.Description != "" {
						room.Items[j].Description = reset.Description
					}
				}
			}
			for existing < reset.Count {
				room.Items = append(room.Items, Item{Name: reset.Name, Description: reset.Description})
				existing++
			}
		}
	}
}

// PlayerLocations returns the set of connected players and their rooms in login order.
func (w *World) PlayerLocations() []PlayerLocation {
	w.mu.RLock()
	defer w.mu.RUnlock()
	locations := make([]PlayerLocation, 0, len(w.players))
	seen := make(map[string]struct{}, len(w.playerOrder))
	for _, name := range w.playerOrder {
		p, ok := w.players[name]
		if !ok || !p.Alive {
			continue
		}
		locations = append(locations, PlayerLocation{Name: p.Name, Room: p.Room})
		seen[p.Name] = struct{}{}
	}
	if len(seen) != len(w.players) {
		for _, p := range w.players {
			if !p.Alive {
				continue
			}
			if _, ok := seen[p.Name]; ok {
				continue
			}
			locations = append(locations, PlayerLocation{Name: p.Name, Room: p.Room})
		}
	}
	return locations
}

func (w *World) removePlayerOrderLocked(name string) {
	for i, existing := range w.playerOrder {
		if existing == name {
			w.playerOrder = append(w.playerOrder[:i], w.playerOrder[i+1:]...)
			return
		}
	}
}

func (w *World) replacePlayerOrderLocked(oldName, newName string) {
	for i, existing := range w.playerOrder {
		if existing == oldName {
			w.playerOrder[i] = newName
			return
		}
	}
	w.playerOrder = append(w.playerOrder, newName)
}
