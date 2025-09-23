package main

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
		if got := exitList(r); got != want {
			t.Fatalf("exitList() = %q, want %q", got, want)
		}
	}
}
