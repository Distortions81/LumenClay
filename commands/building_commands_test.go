package commands

import (
	"strings"
	"testing"

	"LumenClay/internal/game"
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

func TestResetRequiresBuilder(t *testing.T) {
	world := game.NewWorldWithRooms(map[game.RoomID]*game.Room{
		"start": {
			ID:          "start",
			Title:       "Start",
			Description: "Start room.",
			Exits:       map[string]game.RoomID{},
		},
	})
	player := newTestPlayer("Visitor", "start")
	world.AddPlayerForTest(player)

	if quit := Dispatch(world, player, "reset add npc Stone Guide"); quit {
		t.Fatalf("dispatch returned true, want false")
	}
	msgs := drainOutput(player.Output)
	sawWarning := false
	for _, msg := range msgs {
		if strings.Contains(msg, "Only builders or admins may manage resets") {
			sawWarning = true
		}
	}
	if !sawWarning {
		t.Fatalf("expected warning, got %v", msgs)
	}
}

func TestResetAddNPCAndList(t *testing.T) {
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

	if quit := Dispatch(world, builder, "reset add npc Stone Guide = Welcome to the crossroads!"); quit {
		t.Fatalf("dispatch returned true, want false")
	}
	room, ok := world.GetRoom("start")
	if !ok {
		t.Fatalf("current room missing")
	}
	if len(room.NPCs) != 1 || room.NPCs[0].Name != "Stone Guide" {
		t.Fatalf("expected npc to be added, got %+v", room.NPCs)
	}
	if len(room.Resets) != 1 || room.Resets[0].Kind != game.ResetKindNPC {
		t.Fatalf("expected npc reset, got %+v", room.Resets)
	}

	if quit := Dispatch(world, builder, "reset list"); quit {
		t.Fatalf("dispatch returned true, want false")
	}
	msgs := drainOutput(builder.Output)
	listed := false
	for _, msg := range msgs {
		if strings.Contains(msg, "Stone Guide") {
			listed = true
		}
	}
	if !listed {
		t.Fatalf("expected npc listing, got %v", msgs)
	}
}

func TestResetAddItemAndApply(t *testing.T) {
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

	if quit := Dispatch(world, builder, "reset add item Shiny Coin = A gleaming coin."); quit {
		t.Fatalf("dispatch returned true, want false")
	}
	room, ok := world.GetRoom("start")
	if !ok {
		t.Fatalf("current room missing")
	}
	if len(room.Items) != 1 || room.Items[0].Name != "Shiny Coin" {
		t.Fatalf("expected item to spawn, got %+v", room.Items)
	}
	// Simulate the item being taken.
	room.Items = nil

	if quit := Dispatch(world, builder, "reset apply"); quit {
		t.Fatalf("dispatch returned true, want false")
	}
	room, _ = world.GetRoom("start")
	if len(room.Items) == 0 {
		t.Fatalf("expected item to respawn after apply")
	}
}

func TestCloneCopiesPopulation(t *testing.T) {
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
			Description: "Marble hall.",
			Exits:       map[string]game.RoomID{},
			NPCs:        []game.NPC{{Name: "Marble Steward", AutoGreet: "Mind the echoes."}},
			Items:       []game.Item{{Name: "Crystal Torch", Description: "A torch that glows softly."}},
			Resets: []game.RoomReset{
				{Kind: game.ResetKindNPC, Name: "Marble Steward", AutoGreet: "Mind the echoes.", Count: 1},
				{Kind: game.ResetKindItem, Name: "Crystal Torch", Description: "A torch that glows softly.", Count: 1},
			},
		},
	})
	builder := newTestPlayer("Builder", "start")
	builder.IsBuilder = true
	world.AddPlayerForTest(builder)

	if quit := Dispatch(world, builder, "clone hall"); quit {
		t.Fatalf("dispatch returned true, want false")
	}
	room, ok := world.GetRoom("start")
	if !ok {
		t.Fatalf("start room missing")
	}
	if len(room.NPCs) != 1 || room.NPCs[0].Name != "Marble Steward" {
		t.Fatalf("expected npc to be cloned, got %+v", room.NPCs)
	}
	if len(room.Items) != 1 || room.Items[0].Name != "Crystal Torch" {
		t.Fatalf("expected item to be cloned, got %+v", room.Items)
	}
	if len(room.Resets) != 2 {
		t.Fatalf("expected resets to be cloned, got %+v", room.Resets)
	}
}

func TestNameRoomUpdatesTitle(t *testing.T) {
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

	if quit := Dispatch(world, builder, "name room Gathering Hall"); quit {
		t.Fatalf("dispatch returned true, want false")
	}
	room, ok := world.GetRoom("start")
	if !ok {
		t.Fatalf("room missing")
	}
	if room.Title != "Gathering Hall" {
		t.Fatalf("room title = %q, want Gathering Hall", room.Title)
	}
	output := strings.Join(drainOutput(builder.Output), "")
	if !strings.Contains(output, "Room name updated") {
		t.Fatalf("expected confirmation, got %q", output)
	}
}

func TestListShowsRoomRevisions(t *testing.T) {
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
	drainOutput(builder.Output)

	if quit := Dispatch(world, builder, "list"); quit {
		t.Fatalf("dispatch returned true, want false")
	}
	output := strings.Join(drainOutput(builder.Output), "")
	if !strings.Contains(output, "#1") || !strings.Contains(output, "#2") {
		t.Fatalf("expected revision numbers, got %q", output)
	}
	if !strings.Contains(output, "Builder") {
		t.Fatalf("expected editor name, got %q", output)
	}
}

func TestRevnumRevertsRoom(t *testing.T) {
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

	if quit := Dispatch(world, builder, "describe First draft"); quit {
		t.Fatalf("dispatch returned true, want false")
	}
	if quit := Dispatch(world, builder, "describe Second draft"); quit {
		t.Fatalf("dispatch returned true, want false")
	}
	drainOutput(builder.Output)

	if quit := Dispatch(world, builder, "revnum 2"); quit {
		t.Fatalf("dispatch returned true, want false")
	}
	msgs := strings.Join(drainOutput(builder.Output), "")
	if !strings.Contains(msgs, "Room reverted to revision #2") {
		t.Fatalf("expected revert confirmation, got %q", msgs)
	}
	room, ok := world.GetRoom("start")
	if !ok {
		t.Fatalf("room missing")
	}
	if room.Description != "First draft" {
		t.Fatalf("room description = %q, want First draft", room.Description)
	}
}
