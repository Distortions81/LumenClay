package commands

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"LumenClay/internal/game"
)

func TestStatsCommandDisplaysAccountInformation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "accounts.json")

	manager, err := game.NewAccountManager(path)
	if err != nil {
		t.Fatalf("NewAccountManager: %v", err)
	}
	if err := manager.Register("Seeker", "password123"); err != nil {
		t.Fatalf("Register: %v", err)
	}
	loginTime := time.Date(2025, time.January, 2, 15, 4, 5, 0, time.UTC)
	if err := manager.RecordLogin("Seeker", loginTime); err != nil {
		t.Fatalf("RecordLogin: %v", err)
	}

	world := game.NewWorldWithRooms(map[game.RoomID]*game.Room{
		"start": {
			ID:          "start",
			Title:       "Radiant Nexus",
			Description: "Light dances around you.",
			Exits:       map[string]game.RoomID{},
		},
	})
	world.AttachAccountManager(manager)

	player := newTestPlayer("Seeker", "start")
	player.IsAdmin = true
	player.IsBuilder = true
	world.AddPlayerForTest(player)
	world.SetChannel(player, game.ChannelWhisper, false)

	if done := Dispatch(world, player, "stats"); done {
		t.Fatalf("dispatch returned true, want false")
	}

	output := strings.Join(drainOutput(player.Output), "\n")
	if !strings.Contains(output, "Account overview") {
		t.Fatalf("expected account overview in output: %q", output)
	}
	if !strings.Contains(output, "Name: Seeker") {
		t.Fatalf("expected player name in output: %q", output)
	}
	if !strings.Contains(output, "Account: Seeker") {
		t.Fatalf("expected account name in output: %q", output)
	}
	if !strings.Contains(output, "Roles: Player, Builder, Admin") {
		t.Fatalf("expected roles in output: %q", output)
	}
	if !strings.Contains(output, "Home: Radiant Nexus") {
		t.Fatalf("expected home title in output: %q", output)
	}
	if !strings.Contains(output, "Location: Radiant Nexus") {
		t.Fatalf("expected location title in output: %q", output)
	}
	if !strings.Contains(output, "Total logins: 1") {
		t.Fatalf("expected login count in output: %q", output)
	}
	if !strings.Contains(output, "Last login: 2025-01-02 15:04") {
		t.Fatalf("expected last login timestamp in output: %q", output)
	}
	if !strings.Contains(output, "off: WHISPER") {
		t.Fatalf("expected disabled channel indicator in output: %q", output)
	}
}
