package commands

import (
	"strings"
	"testing"

	"LumenClay/internal/game"
)

func TestWhoCommandListsOthersInLoginOrder(t *testing.T) {
	world := game.NewWorldWithRooms(map[game.RoomID]*game.Room{
		"start": {
			ID:          "start",
			Title:       "A Quiet Chamber",
			Description: "Soft light filters through the air.",
			Exits:       map[string]game.RoomID{},
		},
	})

	hero := newTestPlayer("Hero", "start")
	watcher := newTestPlayer("Watcher", "start")
	scout := newTestPlayer("Scout", "start")

	world.AddPlayerForTest(hero)
	world.AddPlayerForTest(watcher)
	world.AddPlayerForTest(scout)

	if done := Dispatch(world, hero, "who"); done {
		t.Fatalf("dispatch returned true, want false")
	}

	output := strings.Join(drainOutput(hero.Output), "\n")
	want := "Other adventurers online: Watcher, Scout"
	if !strings.Contains(output, want) {
		t.Fatalf("who output = %q, want substring %q", output, want)
	}
}

func TestWhoCommandHandlesNoOtherPlayers(t *testing.T) {
	world := game.NewWorldWithRooms(map[game.RoomID]*game.Room{
		"start": {
			ID:          "start",
			Title:       "A Quiet Chamber",
			Description: "Soft light filters through the air.",
			Exits:       map[string]game.RoomID{},
		},
	})

	hero := newTestPlayer("Solo", "start")
	world.AddPlayerForTest(hero)

	if done := Dispatch(world, hero, "who"); done {
		t.Fatalf("dispatch returned true, want false")
	}

	output := strings.Join(drainOutput(hero.Output), "\n")
	want := "You are the only adventurer online."
	if !strings.Contains(output, want) {
		t.Fatalf("who output = %q, want substring %q", output, want)
	}
}
