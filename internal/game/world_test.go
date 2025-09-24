package game

import (
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
