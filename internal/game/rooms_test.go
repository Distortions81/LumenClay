package game

import (
	"strings"
	"testing"
)

func TestExitListSortsDirections(t *testing.T) {
	r := &Room{
		Exits: map[string]RoomID{
			"west":  "room_w",
			"north": "room_n",
			"east":  "room_e",
		},
	}

	const want = "east north west"
	for i := 0; i < 5; i++ {
		if got := ExitList(r); got != want {
			t.Fatalf("ExitList() = %q, want %q", got, want)
		}
	}
}

func TestExitListHandlesNoExits(t *testing.T) {
	r := &Room{Exits: map[string]RoomID{}}
	if got := ExitList(r); got != "none" {
		t.Fatalf("ExitList() = %q, want %q", got, "none")
	}
}

func TestFilterOutRemovesName(t *testing.T) {
	list := []string{"hero", "villain", "sidekick"}
	got := FilterOut(list, "hero")
	want := []string{"villain", "sidekick"}
	if len(got) != len(want) {
		t.Fatalf("FilterOut() len = %d, want %d", len(got), len(want))
	}
	for i, name := range want {
		if got[i] != name {
			t.Fatalf("FilterOut()[%d] = %q, want %q", i, got[i], name)
		}
	}
	if len(list) != 3 {
		t.Fatalf("FilterOut() modified input slice: %v", list)
	}
}

func TestEnterRoomTriggersNPCGreeting(t *testing.T) {
	world := &World{
		rooms: map[RoomID]*Room{
			"start": {
				ID:          "start",
				Title:       "Test Room",
				Description: "A place for testing.",
				Exits:       map[string]RoomID{},
				NPCs: []NPC{
					{
						Name:      "Guide",
						AutoGreet: "Welcome to the test hall!",
					},
				},
			},
		},
		players: make(map[string]*Player),
	}
	player := &Player{
		Name:   "Hero",
		Room:   "start",
		Output: make(chan string, 4),
		Alive:  true,
	}
	world.players[player.Name] = player

	EnterRoom(world, player, "")

	// First output is the room description.
	<-player.Output
	// Second output should contain the NPC greeting.
	greet := <-player.Output
	if !strings.Contains(greet, "Guide") {
		t.Fatalf("NPC greeting missing name: %q", greet)
	}
	if !strings.Contains(greet, "Welcome to the test hall!") {
		t.Fatalf("NPC greeting missing text: %q", greet)
	}
}

func TestEnterRoomListsItems(t *testing.T) {
	world := &World{
		rooms: map[RoomID]*Room{
			"start": {
				ID:          "start",
				Title:       "Item Room",
				Description: "A room stocked with treasures.",
				Exits:       map[string]RoomID{},
				Items: []Item{
					{Name: "Lantern"},
					{Name: "Rope"},
				},
			},
		},
		players: make(map[string]*Player),
	}
	player := &Player{
		Name:   "Hero",
		Room:   "start",
		Output: make(chan string, 4),
		Alive:  true,
	}
	world.players[player.Name] = player

	EnterRoom(world, player, "")

	<-player.Output // room description
	items := <-player.Output
	if !strings.Contains(items, "Lantern") || !strings.Contains(items, "Rope") {
		t.Fatalf("expected item list to mention Lantern and Rope, got %q", items)
	}
}
