package game

import (
	"errors"
	"os"
	"path/filepath"
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
	}
	if err := manager.SaveProfile("alice", updated); err != nil {
		t.Fatalf("SaveProfile: %v", err)
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
}

func TestAccountManagerLoadLegacyFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "accounts.json")
	if err := os.WriteFile(path, []byte(`{"legacy":"hash"}`), 0o644); err != nil {
		t.Fatalf("write legacy file: %v", err)
	}

	manager, err := NewAccountManager(path)
	if err != nil {
		t.Fatalf("NewAccountManager: %v", err)
	}

	profile := manager.Profile("legacy")
	if profile.Room != StartRoom {
		t.Fatalf("legacy profile should default to %q, got %q", StartRoom, profile.Room)
	}
	if profile.Home != StartRoom {
		t.Fatalf("legacy profile should default home to %q, got %q", StartRoom, profile.Home)
	}
	for _, channel := range AllChannels() {
		if !profile.Channels[channel] {
			t.Fatalf("legacy profile channel %s should default to enabled", channel)
		}
	}

	desired := PlayerProfile{Room: "garden", Home: "garden", Channels: defaultChannelSettings()}
	if err := manager.SaveProfile("legacy", desired); err != nil {
		t.Fatalf("SaveProfile legacy: %v", err)
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
