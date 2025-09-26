package game

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestWorldMoveUnknownRoom(t *testing.T) {
	w := &World{
		rooms:   map[RoomID]*Room{},
		players: make(map[string]*Player),
	}
	p := &Player{Name: "tester", Room: RoomID("missing")}

	_, err := w.Move(p, "north")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	want := "unknown room: missing"
	if err.Error() != want {
		t.Fatalf("unexpected error: got %q, want %q", err.Error(), want)
	}
}

func TestWorldTakeDropItem(t *testing.T) {
	roomID := RoomID("store")
	item := Item{Name: "Crystal Key"}
	world := &World{
		rooms: map[RoomID]*Room{
			roomID: {
				ID:    roomID,
				Exits: map[string]RoomID{},
				Items: []Item{item},
			},
		},
		players: make(map[string]*Player),
	}
	player := &Player{Name: "Collector", Room: roomID, Alive: true}
	world.players[player.Name] = player

	taken, err := world.TakeItem(player, "crystal key")
	if err != nil {
		t.Fatalf("TakeItem returned error: %v", err)
	}
	if taken == nil || taken.Name != item.Name {
		t.Fatalf("TakeItem returned %+v, want %+v", taken, item)
	}
	if len(world.rooms[roomID].Items) != 0 {
		t.Fatalf("expected room to be empty after taking item, got %v", world.rooms[roomID].Items)
	}
	if len(player.Inventory) != 1 || player.Inventory[0].Name != item.Name {
		t.Fatalf("player inventory = %#v, want Crystal Key", player.Inventory)
	}

	dropped, err := world.DropItem(player, "Crystal Key")
	if err != nil {
		t.Fatalf("DropItem returned error: %v", err)
	}
	if dropped == nil || dropped.Name != item.Name {
		t.Fatalf("DropItem returned %+v, want %+v", dropped, item)
	}
	if len(player.Inventory) != 0 {
		t.Fatalf("expected empty inventory, got %#v", player.Inventory)
	}
	if len(world.rooms[roomID].Items) != 1 || world.rooms[roomID].Items[0].Name != item.Name {
		t.Fatalf("room items = %#v, want Crystal Key", world.rooms[roomID].Items)
	}

	if _, err := world.DropItem(player, "Crystal Key"); !errors.Is(err, ErrItemNotCarried) {
		t.Fatalf("expected ErrItemNotCarried, got %v", err)
	}
	if _, err := world.TakeItem(player, "missing"); !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("expected ErrItemNotFound, got %v", err)
	}
}

func TestWorldTakeItemMatchesPartialWord(t *testing.T) {
	roomID := RoomID("vault")
	item := Item{Name: "Crystal Key"}
	world := &World{
		rooms: map[RoomID]*Room{
			roomID: {
				ID:    roomID,
				Exits: map[string]RoomID{},
				Items: []Item{item},
			},
		},
		players: make(map[string]*Player),
	}
	player := &Player{Name: "Collector", Room: roomID, Alive: true}
	world.players[player.Name] = player

	taken, err := world.TakeItem(player, "key")
	if err != nil {
		t.Fatalf("TakeItem returned error: %v", err)
	}
	if taken == nil || taken.Name != item.Name {
		t.Fatalf("TakeItem returned %+v, want %+v", taken, item)
	}
}

func TestWorldTakeItemPartialAmbiguous(t *testing.T) {
	roomID := RoomID("closet")
	world := &World{
		rooms: map[RoomID]*Room{
			roomID: {
				ID:    roomID,
				Exits: map[string]RoomID{},
				Items: []Item{{Name: "Silver Key"}, {Name: "Steel Key"}},
			},
		},
		players: make(map[string]*Player),
	}
	player := &Player{Name: "Collector", Room: roomID, Alive: true}
	world.players[player.Name] = player

	if _, err := world.TakeItem(player, "key"); !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("expected ErrItemNotFound for ambiguous match, got %v", err)
	}
}

func TestCloneRoomPopulationUnknownSource(t *testing.T) {
	targetID := RoomID("target")
	world := &World{
		rooms: map[RoomID]*Room{
			targetID: {
				ID:    targetID,
				Items: []Item{{Name: "keep"}},
			},
		},
	}

	err := world.CloneRoomPopulation(RoomID("missing"), targetID)
	if err == nil || err.Error() != "unknown room: missing" {
		t.Fatalf("expected unknown room error, got %v", err)
	}

	if got := world.rooms[targetID].Items; len(got) != 1 || got[0].Name != "keep" {
		t.Fatalf("target room mutated on failure: %+v", got)
	}
}

func TestCloneRoomPopulationUnknownTarget(t *testing.T) {
	sourceID := RoomID("source")
	world := &World{
		rooms: map[RoomID]*Room{
			sourceID: {
				ID:    sourceID,
				Items: []Item{{Name: "copy"}},
			},
		},
	}

	err := world.CloneRoomPopulation(sourceID, RoomID("missing"))
	if err == nil || err.Error() != "unknown room: missing" {
		t.Fatalf("expected unknown room error, got %v", err)
	}

	if got := world.rooms[sourceID].Items; len(got) != 1 || got[0].Name != "copy" {
		t.Fatalf("source room mutated on failure: %+v", got)
	}
}

func TestCloneRoomPopulationPersistFailureRollsBack(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte(""), 0o600); err != nil {
		t.Fatalf("WriteFile blocker: %v", err)
	}

	sourceID := RoomID("source")
	targetID := RoomID("target")
	initialItems := []Item{{Name: "original item"}}
	initialNPCs := []NPC{{Name: "Guide"}}
	initialResets := []RoomReset{{Kind: ResetKindItem, Name: "original item", Count: 1}}
	world := &World{
		rooms: map[RoomID]*Room{
			sourceID: {
				ID:     sourceID,
				Items:  []Item{{Name: "cloned"}},
				NPCs:   []NPC{{Name: "Goblin"}},
				Resets: []RoomReset{{Kind: ResetKindNPC, Name: "Goblin"}},
			},
			targetID: {
				ID:     targetID,
				Items:  append([]Item(nil), initialItems...),
				NPCs:   append([]NPC(nil), initialNPCs...),
				Resets: append([]RoomReset(nil), initialResets...),
			},
		},
		roomSources: map[RoomID]string{
			targetID: "stock.json",
		},
		builderPath: filepath.Join(blocker, "builder.json"),
	}

	err := world.CloneRoomPopulation(sourceID, targetID)
	if err == nil || !strings.Contains(err.Error(), "create builder area directory") {
		t.Fatalf("expected persistence error, got %v", err)
	}

	if !reflect.DeepEqual(world.rooms[targetID].Items, initialItems) {
		t.Fatalf("items not rolled back: got %+v want %+v", world.rooms[targetID].Items, initialItems)
	}
	if !reflect.DeepEqual(world.rooms[targetID].NPCs, initialNPCs) {
		t.Fatalf("npcs not rolled back: got %+v want %+v", world.rooms[targetID].NPCs, initialNPCs)
	}
	if !reflect.DeepEqual(world.rooms[targetID].Resets, initialResets) {
		t.Fatalf("resets not rolled back: got %+v want %+v", world.rooms[targetID].Resets, initialResets)
	}
	if got := world.roomSources[targetID]; got != "stock.json" {
		t.Fatalf("roomSources not restored: got %q want %q", got, "stock.json")
	}
}

func TestWorldFindPlayerPartialMatch(t *testing.T) {
	world := &World{players: make(map[string]*Player)}
	alice := &Player{Name: "Alice", Alive: true}
	alfred := &Player{Name: "Alfred", Alive: true}
	bob := &Player{Name: "Bob", Alive: true}
	world.players[alice.Name] = alice
	world.players[alfred.Name] = alfred
	world.players[bob.Name] = bob

	if p, ok := world.FindPlayer("ali"); !ok || p != alice {
		t.Fatalf("FindPlayer partial prefix = (%v, %t), want Alice, true", p, ok)
	}
	if p, ok := world.FindPlayer("Alice"); !ok || p != alice {
		t.Fatalf("FindPlayer exact case-sensitive = (%v, %t), want Alice, true", p, ok)
	}
	if p, ok := world.FindPlayer("bob"); !ok || p != bob {
		t.Fatalf("FindPlayer case-insensitive = (%v, %t), want Bob, true", p, ok)
	}
	if _, ok := world.FindPlayer("al"); ok {
		t.Fatalf("expected ambiguous partial to fail")
	}

	alfred.Alive = false
	if p, ok := world.FindPlayer("al"); !ok || p != alice {
		t.Fatalf("FindPlayer should ignore offline players, got (%v, %t)", p, ok)
	}
}

func TestPlayerAllowCommandThrottles(t *testing.T) {
	p := &Player{}
	base := time.Now()
	for i := 0; i < commandLimit; i++ {
		if !p.allowCommand(base.Add(time.Duration(i) * (commandWindow / commandLimit))) {
			t.Fatalf("command %d should be allowed", i)
		}
	}
	if p.allowCommand(base.Add(commandWindow / 2)) {
		t.Fatalf("command should have been throttled")
	}
	if !p.allowCommand(base.Add(commandWindow + time.Millisecond)) {
		t.Fatalf("command should be allowed after window")
	}
}

func TestWorldChannelHistoryAndAliasResolution(t *testing.T) {
	world := NewWorldWithRooms(map[RoomID]*Room{StartRoom: {ID: StartRoom}})
	player := &Player{
		Name:     "Alice",
		Room:     StartRoom,
		Output:   make(chan string, 8),
		Alive:    true,
		Channels: DefaultChannelSettings(),
	}
	world.AddPlayerForTest(player)

	msg := Ansi("greetings")
	world.BroadcastToAllChannel(msg, nil, ChannelOOC)
	entries := world.ChannelHistory(player, ChannelOOC, ChannelHistoryLimit)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Message != msg {
		t.Fatalf("expected message %q, got %q", msg, entries[0].Message)
	}

	self := Ansi("self")
	world.RecordPlayerChannelMessage(player, ChannelOOC, self)
	entries = world.ChannelHistory(player, ChannelOOC, 2)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[1].Message != self {
		t.Fatalf("expected second entry %q, got %q", self, entries[1].Message)
	}

	limited := world.ChannelHistory(player, ChannelOOC, 1)
	if len(limited) != 1 || limited[0].Message != self {
		t.Fatalf("expected limited history to contain most recent message")
	}

	world.SetChannelAlias(player, ChannelOOC, "chat")
	if alias := world.ChannelAlias(player, ChannelOOC); alias != "chat" {
		t.Fatalf("alias = %q, want chat", alias)
	}
	resolved, ok := world.ResolveChannelToken(player, "chat")
	if !ok || resolved != ChannelOOC {
		t.Fatalf("ResolveChannelToken = (%v, %t), want (ChannelOOC, true)", resolved, ok)
	}
}

func TestWorldChannelMute(t *testing.T) {
	world := NewWorldWithRooms(map[RoomID]*Room{StartRoom: {ID: StartRoom}})
	player := &Player{
		Name:     "Alice",
		Room:     StartRoom,
		Output:   make(chan string, 8),
		Alive:    true,
		Channels: DefaultChannelSettings(),
	}
	world.AddPlayerForTest(player)

	if world.ChannelMuted(player, ChannelSay) {
		t.Fatalf("player should not begin muted")
	}
	world.SetChannelMute(player, ChannelSay, true)
	if !world.ChannelMuted(player, ChannelSay) {
		t.Fatalf("expected player to be muted")
	}
	world.SetChannelMute(player, ChannelSay, false)
	if world.ChannelMuted(player, ChannelSay) {
		t.Fatalf("expected player to be unmuted")
	}
}

func TestAccountManagerProfilePersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "accounts.json")

	manager, err := NewAccountManager(path)
	if err != nil {
		t.Fatalf("NewAccountManager: %v", err)
	}
	if err := manager.Register("alice", "password123"); err != nil {
		t.Fatalf("Register: %v", err)
	}

	profile := manager.Profile("alice")
	if profile.Room != StartRoom {
		t.Fatalf("expected default room %q, got %q", StartRoom, profile.Room)
	}
	if profile.Home != StartRoom {
		t.Fatalf("expected default home %q, got %q", StartRoom, profile.Home)
	}
	for _, channel := range AllChannels() {
		if !profile.Channels[channel] {
			t.Fatalf("expected channel %s to default to enabled", channel)
		}
	}

	updated := PlayerProfile{
		Room: "lobby",
		Home: "lobby",
		Channels: map[Channel]bool{
			ChannelSay:     true,
			ChannelWhisper: false,
			ChannelYell:    true,
			ChannelOOC:     false,
		},
		Aliases: map[Channel]string{
			ChannelWhisper: "quiet",
		},
	}
	if err := manager.SaveProfile("alice", updated); err != nil {
		t.Fatalf("SaveProfile: %v", err)
	}

	playerPath := manager.playerFilePath("alice")
	if _, err := os.Stat(playerPath); err != nil {
		t.Fatalf("expected player file to exist: %v", err)
	}
	sum := sha256.Sum256([]byte("alice"))
	expectedFile := hex.EncodeToString(sum[:]) + ".json"
	if filepath.Base(playerPath) != expectedFile {
		t.Fatalf("player file = %q, want %q", filepath.Base(playerPath), expectedFile)
	}
	var stored struct {
		Room     string            `json:"room"`
		Home     string            `json:"home"`
		Channels map[string]bool   `json:"channels"`
		Aliases  map[string]string `json:"aliases"`
	}
	data, err := os.ReadFile(playerPath)
	if err != nil {
		t.Fatalf("read player file: %v", err)
	}
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatalf("decode player file: %v", err)
	}
	if stored.Room != string(updated.Room) {
		t.Fatalf("player file room = %q, want %q", stored.Room, updated.Room)
	}
	if stored.Home != string(updated.Home) {
		t.Fatalf("player file home = %q, want %q", stored.Home, updated.Home)
	}
	if len(stored.Channels) != len(updated.Channels) {
		t.Fatalf("player file channels = %v, want %v", stored.Channels, updated.Channels)
	}
	for channel, want := range updated.Channels {
		got, ok := stored.Channels[string(channel)]
		if !ok {
			t.Fatalf("player file missing channel %s", channel)
		}
		if got != want {
			t.Fatalf("player file channel %s = %t, want %t", channel, got, want)
		}
	}
	if len(stored.Aliases) != len(updated.Aliases) {
		t.Fatalf("player file aliases = %v, want %v", stored.Aliases, updated.Aliases)
	}
	for channel, want := range updated.Aliases {
		if got := stored.Aliases[string(channel)]; got != want {
			t.Fatalf("player file alias %s = %q, want %q", channel, got, want)
		}
	}

	profile = manager.Profile("alice")
	if profile.Room != updated.Room {
		t.Fatalf("expected room %q, got %q", updated.Room, profile.Room)
	}
	if profile.Home != updated.Home {
		t.Fatalf("expected home %q, got %q", updated.Home, profile.Home)
	}
	for channel, want := range updated.Channels {
		if got := profile.Channels[channel]; got != want {
			t.Fatalf("channel %s: got %t, want %t", channel, got, want)
		}
	}
	for channel, want := range updated.Aliases {
		if got := profile.Aliases[channel]; got != want {
			t.Fatalf("alias %s: got %q, want %q", channel, got, want)
		}
	}

	reloaded, err := NewAccountManager(path)
	if err != nil {
		t.Fatalf("reload manager: %v", err)
	}
	profile = reloaded.Profile("alice")
	if profile.Room != updated.Room {
		t.Fatalf("expected persisted room %q, got %q", updated.Room, profile.Room)
	}
	if profile.Home != updated.Home {
		t.Fatalf("expected persisted home %q, got %q", updated.Home, profile.Home)
	}
	for channel, want := range updated.Channels {
		if got := profile.Channels[channel]; got != want {
			t.Fatalf("persisted channel %s: got %t, want %t", channel, got, want)
		}
	}
	for channel, want := range updated.Aliases {
		if got := profile.Aliases[channel]; got != want {
			t.Fatalf("persisted alias %s: got %q, want %q", channel, got, want)
		}
	}
}

func TestAccountManagerRecordLoginAndStats(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "accounts.json")

	manager, err := NewAccountManager(path)
	if err != nil {
		t.Fatalf("NewAccountManager: %v", err)
	}
	if err := manager.Register("explorer", "secretpw"); err != nil {
		t.Fatalf("Register: %v", err)
	}

	stats, ok := manager.Stats("explorer")
	if !ok {
		t.Fatalf("Stats should return data for registered account")
	}
	if stats.CreatedAt.IsZero() {
		t.Fatalf("expected CreatedAt to be recorded")
	}
	if !stats.LastLogin.IsZero() {
		t.Fatalf("LastLogin should be zero before first login")
	}
	if stats.TotalLogins != 0 {
		t.Fatalf("TotalLogins = %d, want 0", stats.TotalLogins)
	}

	firstLogin := time.Date(2025, time.January, 2, 15, 4, 5, 0, time.UTC)
	if err := manager.RecordLogin("explorer", firstLogin); err != nil {
		t.Fatalf("RecordLogin first: %v", err)
	}

	stats, ok = manager.Stats("explorer")
	if !ok {
		t.Fatalf("Stats should still return data")
	}
	if stats.TotalLogins != 1 {
		t.Fatalf("TotalLogins = %d, want 1", stats.TotalLogins)
	}
	if !stats.LastLogin.Equal(firstLogin) {
		t.Fatalf("LastLogin = %v, want %v", stats.LastLogin, firstLogin)
	}

	secondLogin := firstLogin.Add(6 * time.Hour)
	if err := manager.RecordLogin("explorer", secondLogin); err != nil {
		t.Fatalf("RecordLogin second: %v", err)
	}

	stats, ok = manager.Stats("explorer")
	if !ok {
		t.Fatalf("Stats should continue returning data")
	}
	if stats.TotalLogins != 2 {
		t.Fatalf("TotalLogins = %d, want 2", stats.TotalLogins)
	}
	if !stats.LastLogin.Equal(secondLogin) {
		t.Fatalf("LastLogin = %v, want %v", stats.LastLogin, secondLogin)
	}

	reloaded, err := NewAccountManager(path)
	if err != nil {
		t.Fatalf("NewAccountManager reload: %v", err)
	}
	persisted, ok := reloaded.Stats("explorer")
	if !ok {
		t.Fatalf("Stats should return data after reload")
	}
	if !persisted.CreatedAt.Equal(stats.CreatedAt) {
		t.Fatalf("CreatedAt mismatch after reload: got %v want %v", persisted.CreatedAt, stats.CreatedAt)
	}
	if !persisted.LastLogin.Equal(stats.LastLogin) {
		t.Fatalf("LastLogin mismatch after reload: got %v want %v", persisted.LastLogin, stats.LastLogin)
	}
	if persisted.TotalLogins != stats.TotalLogins {
		t.Fatalf("TotalLogins mismatch after reload: got %d want %d", persisted.TotalLogins, stats.TotalLogins)
	}
}

func TestWorldPersistsState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "accounts.json")

	manager, err := NewAccountManager(path)
	if err != nil {
		t.Fatalf("NewAccountManager: %v", err)
	}
	if err := manager.Register("traveler", "password123"); err != nil {
		t.Fatalf("Register: %v", err)
	}

	rooms := map[RoomID]*Room{
		StartRoom: {ID: StartRoom, Exits: map[string]RoomID{"east": "hall"}},
		"hall":    {ID: "hall", Exits: map[string]RoomID{}},
	}
	world := NewWorldWithRooms(rooms)
	world.AttachAccountManager(manager)

	profile := manager.Profile("traveler")
	player, err := world.addPlayer("traveler", nil, false, profile)
	if err != nil {
		t.Fatalf("addPlayer: %v", err)
	}

	if _, err := world.Move(player, "east"); err != nil {
		t.Fatalf("Move: %v", err)
	}
	saved := manager.Profile("traveler")
	if saved.Room != "hall" {
		t.Fatalf("expected room 'hall', got %q", saved.Room)
	}
	if saved.Home != StartRoom {
		t.Fatalf("expected home to remain %q, got %q", StartRoom, saved.Home)
	}

	world.SetChannel(player, ChannelWhisper, false)
	saved = manager.Profile("traveler")
	if saved.Channels[ChannelWhisper] {
		t.Fatalf("expected whisper channel to be disabled")
	}
	if !saved.Channels[ChannelSay] {
		t.Fatalf("say channel should remain enabled")
	}
}

func TestWorldPrepareTakeover(t *testing.T) {
	rooms := map[RoomID]*Room{StartRoom: {ID: StartRoom}}
	world := NewWorldWithRooms(rooms)

	profile := PlayerProfile{Room: StartRoom}
	player, err := world.addPlayer("traveler", nil, false, profile)
	if err != nil {
		t.Fatalf("addPlayer: %v", err)
	}

	if _, ok := world.ActivePlayer("traveler"); !ok {
		t.Fatalf("ActivePlayer should report the connected player")
	}

	oldSession, oldOutput, ok := world.PrepareTakeover("traveler")
	if !ok {
		t.Fatalf("PrepareTakeover should succeed when the player is connected")
	}
	if oldSession != nil {
		t.Fatalf("PrepareTakeover should return nil session when none was set")
	}
	if oldOutput == nil {
		t.Fatalf("PrepareTakeover should return the prior output channel")
	}
	if player.Alive {
		t.Fatalf("player should be marked inactive after takeover preparation")
	}
	if player.Session != nil {
		t.Fatalf("player session should be cleared during takeover preparation")
	}
	if player.Output != nil {
		t.Fatalf("player output channel should be cleared during takeover preparation")
	}
	if _, ok := world.ActivePlayer("traveler"); ok {
		t.Fatalf("ActivePlayer should not report a player pending takeover")
	}

	close(oldOutput)

	rejoined, err := world.addPlayer("traveler", nil, false, profile)
	if err != nil {
		t.Fatalf("addPlayer after takeover: %v", err)
	}
	if !rejoined.Alive {
		t.Fatalf("player should be marked alive after rejoining")
	}
	if rejoined.Output == nil {
		t.Fatalf("rejoined player should have a fresh output channel")
	}

	names := world.ListPlayers(false, "")
	if len(names) != 1 || names[0] != "traveler" {
		t.Fatalf("ListPlayers = %v, want [traveler]", names)
	}
}

func TestWorldSetHomePersists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "accounts.json")

	manager, err := NewAccountManager(path)
	if err != nil {
		t.Fatalf("NewAccountManager: %v", err)
	}
	if err := manager.Register("traveler", "password123"); err != nil {
		t.Fatalf("Register: %v", err)
	}

	rooms := map[RoomID]*Room{
		StartRoom: {ID: StartRoom, Exits: map[string]RoomID{"east": "hall"}},
		"hall":    {ID: "hall", Exits: map[string]RoomID{"west": StartRoom}},
	}
	world := NewWorldWithRooms(rooms)
	world.AttachAccountManager(manager)

	profile := manager.Profile("traveler")
	player, err := world.addPlayer("traveler", nil, false, profile)
	if err != nil {
		t.Fatalf("addPlayer: %v", err)
	}
	player.Room = "hall"
	if err := world.SetHome(player, "hall"); err != nil {
		t.Fatalf("SetHome: %v", err)
	}

	saved := manager.Profile("traveler")
	if saved.Home != "hall" {
		t.Fatalf("expected home 'hall', got %q", saved.Home)
	}
	if saved.Room != player.Room {
		t.Fatalf("expected room %q, got %q", player.Room, saved.Room)
	}
}

func TestListPlayersUsesLoginOrder(t *testing.T) {
	rooms := map[RoomID]*Room{
		StartRoom: {ID: StartRoom, Exits: map[string]RoomID{"east": "hall"}},
		"hall":    {ID: "hall", Exits: map[string]RoomID{"west": StartRoom}},
	}
	world := NewWorldWithRooms(rooms)

	alpha := &Player{Name: "Alpha", Room: StartRoom, Output: make(chan string, 1), Alive: true}
	bravo := &Player{Name: "Bravo", Room: "hall", Output: make(chan string, 1), Alive: true}
	charlie := &Player{Name: "Charlie", Room: StartRoom, Output: make(chan string, 1), Alive: true}

	world.AddPlayerForTest(alpha)
	world.AddPlayerForTest(bravo)
	world.AddPlayerForTest(charlie)

	names := world.ListPlayers(false, "")
	want := []string{"Alpha", "Bravo", "Charlie"}
	if len(names) != len(want) {
		t.Fatalf("ListPlayers length = %d, want %d", len(names), len(want))
	}
	for i, name := range want {
		if names[i] != name {
			t.Fatalf("ListPlayers[%d] = %q, want %q", i, names[i], name)
		}
	}

	roomOnly := world.ListPlayers(true, "hall")
	if len(roomOnly) != 1 || roomOnly[0] != "Bravo" {
		t.Fatalf("ListPlayers room filter = %v, want [Bravo]", roomOnly)
	}

	world.removePlayer("Alpha")

	returning := &Player{Name: "Alpha", Room: "hall", Output: make(chan string, 1), Alive: true}
	world.AddPlayerForTest(returning)

	names = world.ListPlayers(false, "")
	want = []string{"Bravo", "Charlie", "Alpha"}
	if len(names) != len(want) {
		t.Fatalf("ListPlayers length after rejoin = %d, want %d", len(names), len(want))
	}
	for i, name := range want {
		if names[i] != name {
			t.Fatalf("ListPlayers after rejoin[%d] = %q, want %q", i, names[i], name)
		}
	}

	locations := world.PlayerLocations()
	if len(locations) != len(want) {
		t.Fatalf("PlayerLocations length = %d, want %d", len(locations), len(want))
	}
	for i, loc := range locations {
		if loc.Name != want[i] {
			t.Fatalf("PlayerLocations[%d] = %q, want %q", i, loc.Name, want[i])
		}
	}
}

func TestWorldCommandDisableToggle(t *testing.T) {
	world := NewWorldWithRooms(nil)
	if world.CommandDisabled("say") {
		t.Fatalf("command should not be disabled by default")
	}
	world.SetCommandDisabled("say", true)
	if !world.CommandDisabled("say") {
		t.Fatalf("command should be disabled")
	}
	if !world.CommandDisabled("SAY") {
		t.Fatalf("command lookup should be case insensitive")
	}
	world.SetCommandDisabled("say", false)
	if world.CommandDisabled("say") {
		t.Fatalf("command should be enabled")
	}
}

func TestWorldDeliverOfflineTells(t *testing.T) {
	world := NewWorldWithRooms(map[RoomID]*Room{
		"hall": {
			ID:          "hall",
			Title:       "Hall",
			Description: "A quiet hall.",
			Exits:       map[string]RoomID{},
		},
	})
	tells, err := NewTellSystem("")
	if err != nil {
		t.Fatalf("NewTellSystem: %v", err)
	}
	world.AttachTellSystem(tells)

	dir := t.TempDir()
	accounts, err := NewAccountManager(filepath.Join(dir, "accounts.json"))
	if err != nil {
		t.Fatalf("NewAccountManager: %v", err)
	}
	if err := accounts.Register("Friend", "password"); err != nil {
		t.Fatalf("register Friend: %v", err)
	}
	if err := accounts.Register("Sender", "password"); err != nil {
		t.Fatalf("register Sender: %v", err)
	}
	world.AttachAccountManager(accounts)

	sender := &Player{
		Name:     "Sender",
		Room:     "hall",
		Home:     "hall",
		Output:   make(chan string, 8),
		Alive:    true,
		Channels: DefaultChannelSettings(),
	}
	world.AddPlayerForTest(sender)

	if _, _, err := world.QueueOfflineTell(sender, "Friend", "Meet me under the lantern"); err != nil {
		t.Fatalf("QueueOfflineTell: %v", err)
	}

	friend := &Player{
		Name:     "Friend",
		Room:     "hall",
		Home:     "hall",
		Output:   make(chan string, 8),
		Alive:    true,
		Channels: DefaultChannelSettings(),
	}
	world.AddPlayerForTest(friend)

	world.DeliverOfflineTells(friend)

	var first string
	select {
	case first = <-friend.Output:
	default:
		t.Fatalf("friend did not receive offline tell output")
	}
	if !strings.Contains(first, "You have 1 offline tell") {
		t.Fatalf("header missing: %q", first)
	}
	if !strings.Contains(first, "Sender") || !strings.Contains(first, "Meet me under the lantern") {
		t.Fatalf("message missing: %q", first)
	}
	var prompt string
	select {
	case prompt = <-friend.Output:
	default:
		t.Fatalf("friend did not receive prompt after offline tells")
	}
	if !strings.Contains(prompt, ">") {
		t.Fatalf("prompt not received: %q", prompt)
	}
	if pending := tells.PendingFor("Friend"); len(pending) != 0 {
		t.Fatalf("offline tells should be cleared, got %#v", pending)
	}
}
