package commands

import (
	"strings"
	"testing"

	"LumenClay/internal/game"
)

func TestLookListsNPCs(t *testing.T) {
	world := game.NewWorldWithRooms(map[game.RoomID]*game.Room{
		"start": {
			ID:          "start",
			Title:       "Starting Room",
			Description: "A quiet foyer.",
			Exits:       map[string]game.RoomID{"north": "hall"},
			NPCs:        []game.NPC{{Name: "Guide"}},
		},
		"hall": {
			ID:          "hall",
			Title:       "Hallway",
			Description: "A long corridor.",
			Exits:       map[string]game.RoomID{"south": "start"},
		},
	})
	player := newTestPlayer("Hero", "start")
	world.AddPlayerForTest(player)

	if done := Dispatch(world, player, "look"); done {
		t.Fatalf("look returned true, want false")
	}
	msgs := drainOutput(player.Output)
	sawNPCs := false
	for _, msg := range msgs {
		if strings.Contains(msg, "You notice: Guide") {
			sawNPCs = true
			break
		}
	}
	if !sawNPCs {
		t.Fatalf("expected NPC list in look output: %v", msgs)
	}
}

func TestLookAtNPC(t *testing.T) {
	world := game.NewWorldWithRooms(map[game.RoomID]*game.Room{
		"start": {
			ID:          "start",
			Title:       "Starting Room",
			Description: "A quiet foyer.",
			Exits:       map[string]game.RoomID{},
			NPCs:        []game.NPC{{Name: "Guide", AutoGreet: "Welcome!"}},
		},
	})
	player := newTestPlayer("Hero", "start")
	world.AddPlayerForTest(player)

	if done := Dispatch(world, player, "look guide"); done {
		t.Fatalf("look returned true, want false")
	}
	msgs := drainOutput(player.Output)
	if len(msgs) == 0 || !strings.Contains(msgs[len(msgs)-1], "Guide stands here.") {
		t.Fatalf("expected NPC description, got %v", msgs)
	}
}

func TestLookAtItem(t *testing.T) {
	world := game.NewWorldWithRooms(map[game.RoomID]*game.Room{
		"start": {
			ID:          "start",
			Title:       "Starting Room",
			Description: "A quiet foyer.",
			Exits:       map[string]game.RoomID{},
			Items:       []game.Item{{Name: "Golden Key", Description: "It's engraved with runes."}},
		},
	})
	player := newTestPlayer("Hero", "start")
	world.AddPlayerForTest(player)

	if done := Dispatch(world, player, "look key"); done {
		t.Fatalf("look returned true, want false")
	}
	msgs := drainOutput(player.Output)
	matched := false
	for _, msg := range msgs {
		if strings.Contains(msg, "You study Golden Key. It's engraved with runes.") {
			matched = true
			break
		}
	}
	if !matched {
		t.Fatalf("expected item description, got %v", msgs)
	}
}

func TestLookAtExit(t *testing.T) {
	world := game.NewWorldWithRooms(map[game.RoomID]*game.Room{
		"start": {
			ID:          "start",
			Title:       "Starting Room",
			Description: "A quiet foyer.",
			Exits:       map[string]game.RoomID{"north": "garden"},
		},
		"garden": {
			ID:          "garden",
			Title:       "Verdant Garden",
			Description: "A lush garden bathed in sunlight.",
			Exits:       map[string]game.RoomID{"south": "start"},
		},
	})
	player := newTestPlayer("Hero", "start")
	world.AddPlayerForTest(player)

	if done := Dispatch(world, player, "look north"); done {
		t.Fatalf("look returned true, want false")
	}
	msgs := drainOutput(player.Output)
	matched := false
	for _, msg := range msgs {
		if strings.Contains(msg, "Looking north you glimpse Verdant Garden. A lush garden bathed in sunlight.") {
			matched = true
			break
		}
	}
	if !matched {
		t.Fatalf("expected exit description, got %v", msgs)
	}
}

func TestLookUnknownTarget(t *testing.T) {
	world := game.NewWorldWithRooms(map[game.RoomID]*game.Room{
		"start": {
			ID:          "start",
			Title:       "Starting Room",
			Description: "A quiet foyer.",
			Exits:       map[string]game.RoomID{},
		},
	})
	player := newTestPlayer("Hero", "start")
	world.AddPlayerForTest(player)

	if done := Dispatch(world, player, "look dragon"); done {
		t.Fatalf("look returned true, want false")
	}
	msgs := drainOutput(player.Output)
	if len(msgs) == 0 || msgs[len(msgs)-1] != "You don't see that here." {
		t.Fatalf("expected not found message, got %v", msgs)
	}
}

func TestExamineInventoryItem(t *testing.T) {
	world := game.NewWorldWithRooms(map[game.RoomID]*game.Room{
		"start": {
			ID:          "start",
			Title:       "Starting Room",
			Description: "A quiet foyer.",
			Exits:       map[string]game.RoomID{},
		},
	})
	player := newTestPlayer("Hero", "start")
	player.Inventory = []game.Item{{Name: "Lantern", Description: "It glows softly."}}
	world.AddPlayerForTest(player)

	if done := Dispatch(world, player, "examine lantern"); done {
		t.Fatalf("examine returned true, want false")
	}
	msgs := drainOutput(player.Output)
	if len(msgs) == 0 || msgs[len(msgs)-1] != "You examine Lantern. It glows softly." {
		t.Fatalf("expected inventory description, got %v", msgs)
	}
}

func TestExamineUnknownInventoryItem(t *testing.T) {
	world := game.NewWorldWithRooms(map[game.RoomID]*game.Room{
		"start": {
			ID:          "start",
			Title:       "Starting Room",
			Description: "A quiet foyer.",
			Exits:       map[string]game.RoomID{},
		},
	})
	player := newTestPlayer("Hero", "start")
	player.Inventory = []game.Item{{Name: "Lantern"}}
	world.AddPlayerForTest(player)

	if done := Dispatch(world, player, "examine coin"); done {
		t.Fatalf("examine returned true, want false")
	}
	msgs := drainOutput(player.Output)
	if len(msgs) == 0 || msgs[len(msgs)-1] != "You aren't carrying that." {
		t.Fatalf("expected missing item message, got %v", msgs)
	}
}
