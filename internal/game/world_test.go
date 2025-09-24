package game

import (
	"testing"
	"time"
)

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

func TestPlayerAllowCommandThrottles(t *testing.T) {
	p := &Player{}
	base := time.Now()
	for i := 0; i < commandLimit; i++ {
		if !p.allowCommand(base.Add(time.Duration(i) * (commandWindow / commandLimit))) {
			t.Fatalf("command %d should be allowed", i)
		}
	}
	if p.allowCommand(base.Add(commandWindow / 2)) {
		t.Fatalf("command should have been throttled")
	}
	if !p.allowCommand(base.Add(commandWindow + time.Millisecond)) {
		t.Fatalf("command should be allowed after window")
	}
}
