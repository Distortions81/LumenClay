package commands

import (
	"strings"
	"testing"

	"LumenClay/internal/game"
)

func TestRebootDisabledWhenCriticalOperationsLocked(t *testing.T) {
	world := game.NewWorldWithRooms(map[game.RoomID]*game.Room{
		"start": {
			ID:          "start",
			Title:       "Start",
			Description: "Starting point.",
			Exits:       map[string]game.RoomID{},
		},
	})
	world.ConfigurePrivileges(true, true)

	admin := newTestPlayer("Admin", "start")
	world.AddPlayerForTest(admin)

	if quit := Dispatch(world, admin, "reboot"); quit {
		t.Fatalf("dispatch returned true, want false")
	}
	msgs := drainOutput(admin.Output)
	sawDisabled := false
	for _, msg := range msgs {
		if strings.Contains(msg, "World reboot is temporarily disabled.") {
			sawDisabled = true
			break
		}
	}
	if !sawDisabled {
		t.Fatalf("expected disabled message, got %v", msgs)
	}
}
