package commands

import (
	"strings"
	"testing"

	"LumenClay/internal/game"
)

func TestCommandToggleRequiresAdmin(t *testing.T) {
	world := game.NewWorldWithRooms(map[game.RoomID]*game.Room{
		"hall": {
			ID:          "hall",
			Title:       "Hall",
			Description: "An empty hall.",
			Exits:       map[string]game.RoomID{},
		},
	})
	player := newTestPlayer("Player", "hall")
	world.AddPlayerForTest(player)

	if quit := Dispatch(world, player, "command say off"); quit {
		t.Fatalf("dispatch returned true, want false")
	}
	output := strings.Join(drainOutput(player.Output), "\n")
	if !strings.Contains(output, "Only admins may manage commands") {
		t.Fatalf("expected admin warning, got %q", output)
	}
}

func TestCommandToggleDisablesAndEnables(t *testing.T) {
	world := game.NewWorldWithRooms(map[game.RoomID]*game.Room{
		"hall": {
			ID:          "hall",
			Title:       "Hall",
			Description: "An empty hall.",
			Exits:       map[string]game.RoomID{},
		},
	})
	admin := newTestPlayer("Admin", "hall")
	admin.IsAdmin = true
	speaker := newTestPlayer("Speaker", "hall")
	listener := newTestPlayer("Listener", "hall")
	world.AddPlayerForTest(admin)
	world.AddPlayerForTest(speaker)
	world.AddPlayerForTest(listener)

	if quit := Dispatch(world, admin, "command say off"); quit {
		t.Fatalf("dispatch returned true, want false")
	}
	if !world.CommandDisabled("say") {
		t.Fatalf("say command should be disabled")
	}
	adminOutput := strings.Join(drainOutput(admin.Output), "\n")
	if !strings.Contains(adminOutput, "Command say is now disabled.") {
		t.Fatalf("unexpected admin output: %q", adminOutput)
	}

	if quit := Dispatch(world, speaker, "say hello"); quit {
		t.Fatalf("dispatch returned true, want false")
	}
	speakerOutput := strings.Join(drainOutput(speaker.Output), "\n")
	if !strings.Contains(speakerOutput, "command is temporarily disabled") {
		t.Fatalf("expected disabled notice, got %q", speakerOutput)
	}
	if msgs := drainOutput(listener.Output); len(msgs) != 0 {
		t.Fatalf("listener should not receive broadcast, got %v", msgs)
	}

	if quit := Dispatch(world, admin, "command say on"); quit {
		t.Fatalf("dispatch returned true, want false")
	}
	if world.CommandDisabled("say") {
		t.Fatalf("say command should be enabled")
	}
	adminOutput = strings.Join(drainOutput(admin.Output), "\n")
	if !strings.Contains(adminOutput, "Command say is now enabled.") {
		t.Fatalf("unexpected admin output after enable: %q", adminOutput)
	}

	if quit := Dispatch(world, speaker, "say hello again"); quit {
		t.Fatalf("dispatch returned true, want false")
	}
	speakerMsgs := drainOutput(speaker.Output)
	if len(speakerMsgs) == 0 || !strings.Contains(speakerMsgs[len(speakerMsgs)-1], "You say: hello again") {
		t.Fatalf("expected say output, got %v", speakerMsgs)
	}
}

func TestCommandToggleCannotDisableItself(t *testing.T) {
	world := game.NewWorldWithRooms(map[game.RoomID]*game.Room{
		"hall": {
			ID:          "hall",
			Title:       "Hall",
			Description: "An empty hall.",
			Exits:       map[string]game.RoomID{},
		},
	})
	admin := newTestPlayer("Admin", "hall")
	admin.IsAdmin = true
	world.AddPlayerForTest(admin)

	if quit := Dispatch(world, admin, "command command off"); quit {
		t.Fatalf("dispatch returned true, want false")
	}
	output := strings.Join(drainOutput(admin.Output), "\n")
	if !strings.Contains(output, "cannot disable itself") {
		t.Fatalf("expected self-disable warning, got %q", output)
	}
	if world.CommandDisabled("command") {
		t.Fatalf("command toggle should not be disabled")
	}
}
