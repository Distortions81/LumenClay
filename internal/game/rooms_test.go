package game

import "testing"

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
