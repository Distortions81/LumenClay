package commands

import (
	"strings"
	"testing"

	"aiMud/internal/game"
)

func TestDigRequiresBuilder(t *testing.T) {
	world := game.NewWorldWithRooms(map[game.RoomID]*game.Room{
		"start": {
			ID:          "start",
			Title:       "Start",
			Description: "Start room.",
			Exits:       map[string]game.RoomID{},
		},
	})
	player := newTestPlayer("Seeker", "start")
	world.AddPlayerForTest(player)

	if quit := Dispatch(world, player, "dig cavern Cavern of Echoes"); quit {
		t.Fatalf("dispatch returned true, want false")
	}
	msgs := drainOutput(player.Output)
	sawWarning := false
	for _, msg := range msgs {
		if strings.Contains(msg, "Only builders or admins may use dig") {
			sawWarning = true
		}
	}
	if !sawWarning {
		t.Fatalf("expected warning, got %v", msgs)
	}
}

func TestDigCreatesRoom(t *testing.T) {
	world := game.NewWorldWithRooms(map[game.RoomID]*game.Room{
		"start": {
			ID:          "start",
			Title:       "Start",
			Description: "Start room.",
			Exits:       map[string]game.RoomID{},
		},
	})
	builder := newTestPlayer("Builder", "start")
	builder.IsBuilder = true
	world.AddPlayerForTest(builder)

	if quit := Dispatch(world, builder, "dig cavern Cavern of Echoes"); quit {
		t.Fatalf("dispatch returned true, want false")
	}
	room, ok := world.GetRoom("cavern")
	if !ok {
		t.Fatalf("new room not created")
	}
	if room.Title != "Cavern of Echoes" {
		t.Fatalf("room title = %q, want Cavern of Echoes", room.Title)
	}
	msgs := drainOutput(builder.Output)
	sawConfirmation := false
	for _, msg := range msgs {
		if strings.Contains(msg, "Created room cavern") {
			sawConfirmation = true
		}
	}
	if !sawConfirmation {
		t.Fatalf("expected confirmation message, got %v", msgs)
	}
}

func TestDescribeUpdatesRoom(t *testing.T) {
	world := game.NewWorldWithRooms(map[game.RoomID]*game.Room{
		"start": {
			ID:          "start",
			Title:       "Start",
			Description: "Start room.",
			Exits:       map[string]game.RoomID{},
		},
	})
	builder := newTestPlayer("Builder", "start")
	builder.IsBuilder = true
	world.AddPlayerForTest(builder)

	if quit := Dispatch(world, builder, "describe A quiet alcove"); quit {
		t.Fatalf("dispatch returned true, want false")
	}
	room, ok := world.GetRoom("start")
	if !ok {
		t.Fatalf("current room missing")
	}
	if room.Description != "A quiet alcove" {
		t.Fatalf("description = %q, want updated", room.Description)
	}
}

func TestSetExitAndClearExit(t *testing.T) {
	world := game.NewWorldWithRooms(map[game.RoomID]*game.Room{
		"start": {
			ID:          "start",
			Title:       "Start",
			Description: "Start room.",
			Exits:       map[string]game.RoomID{},
		},
		"hall": {
			ID:          "hall",
			Title:       "Hall",
			Description: "Long hall.",
			Exits:       map[string]game.RoomID{},
		},
	})
	builder := newTestPlayer("Builder", "start")
	builder.IsBuilder = true
	world.AddPlayerForTest(builder)

	if quit := Dispatch(world, builder, "setexit north hall"); quit {
		t.Fatalf("dispatch returned true, want false")
	}
	start, _ := world.GetRoom("start")
	if dest, ok := start.Exits["north"]; !ok || dest != "hall" {
		t.Fatalf("exit not set correctly: %v", start.Exits)
	}
	if quit := Dispatch(world, builder, "setexit north none"); quit {
		t.Fatalf("dispatch returned true on clear")
	}
	start, _ = world.GetRoom("start")
	if _, ok := start.Exits["north"]; ok {
		t.Fatalf("exit should be cleared")
	}
}

func TestLinkCreatesBidirectionalExits(t *testing.T) {
	world := game.NewWorldWithRooms(map[game.RoomID]*game.Room{
		"start": {
			ID:          "start",
			Title:       "Start",
			Description: "Start room.",
			Exits:       map[string]game.RoomID{},
		},
		"hall": {
			ID:          "hall",
			Title:       "Hall",
			Description: "Long hall.",
			Exits:       map[string]game.RoomID{},
		},
	})
	builder := newTestPlayer("Builder", "start")
	builder.IsBuilder = true
	world.AddPlayerForTest(builder)

	if quit := Dispatch(world, builder, "link east hall west"); quit {
		t.Fatalf("dispatch returned true, want false")
	}
	start, _ := world.GetRoom("start")
	if dest, ok := start.Exits["east"]; !ok || dest != "hall" {
		t.Fatalf("forward exit incorrect: %v", start.Exits)
	}
	hall, _ := world.GetRoom("hall")
	if dest, ok := hall.Exits["west"]; !ok || dest != "start" {
		t.Fatalf("reverse exit incorrect: %v", hall.Exits)
	}
}
