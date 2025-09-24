package game

import "testing"

func TestWorldMoveUnknownRoom(t *testing.T) {
	w := &World{
		rooms:   map[RoomID]*Room{},
		players: make(map[string]*Player),
	}
	p := &Player{Name: "tester", Room: RoomID("missing")}

	_, err := w.Move(p, "north")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	want := "unknown room: missing"
	if err.Error() != want {
		t.Fatalf("unexpected error: got %q, want %q", err.Error(), want)
	}
}
