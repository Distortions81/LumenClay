package commands

import (
	"strings"
	"testing"

	"LumenClay/internal/game"
)

func TestDreamCommandPersonalisedAndDeterministic(t *testing.T) {
	world := game.NewWorldWithRooms(map[game.RoomID]*game.Room{
		"hall": {
			ID:          "hall",
			Title:       "Hall",
			Description: "An empty hall.",
			Exits:       map[string]game.RoomID{},
		},
	})
	dreamer := newTestPlayer("Lyra", "hall")
	world.AddPlayerForTest(dreamer)

	if quit := Dispatch(world, dreamer, "dream"); quit {
		t.Fatalf("dispatch returned true, want false")
	}
	first := strings.Join(drainOutput(dreamer.Output), "\n")
	if !strings.Contains(first, "Lyra") {
		t.Fatalf("expected player's name in dream, got %q", first)
	}
	if !strings.Contains(strings.ToLower(first), "dream") {
		t.Fatalf("expected dream motif in output, got %q", first)
	}

	if quit := Dispatch(world, dreamer, "dream"); quit {
		t.Fatalf("dispatch returned true, want false")
	}
	second := strings.Join(drainOutput(dreamer.Output), "\n")
	if first != second {
		t.Fatalf("dream output should be deterministic, first %q second %q", first, second)
	}
}
