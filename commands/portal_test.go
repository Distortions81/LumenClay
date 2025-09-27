package commands

import (
	"strings"
	"testing"
	"time"

	"LumenClay/internal/game"
)

type fakePortal struct {
	url      string
	expires  time.Time
	lastRole game.PortalRole
	err      error
}

func (f *fakePortal) GenerateLink(role game.PortalRole, player string) (game.PortalLink, error) {
	f.lastRole = role
	if f.err != nil {
		return game.PortalLink{}, f.err
	}
	return game.PortalLink{URL: f.url, Expires: f.expires, Role: role}, nil
}

func TestPortalCommandRequiresPortal(t *testing.T) {
	world := game.NewWorldWithRooms(map[game.RoomID]*game.Room{
		"start": {ID: "start", Title: "Start", Description: "", Exits: map[string]game.RoomID{}},
	})
	builder := newTestPlayer("Builder", "start")
	builder.IsBuilder = true
	world.AddPlayerForTest(builder)

	if quit := Dispatch(world, builder, "portal"); quit {
		t.Fatalf("dispatch returned true, want false")
	}
	output := strings.Join(drainOutput(builder.Output), "\n")
	if !strings.Contains(output, "web portal is not configured") {
		t.Fatalf("expected configuration warning, got %q", output)
	}
}

func TestPortalCommandGeneratesLink(t *testing.T) {
	world := game.NewWorldWithRooms(map[game.RoomID]*game.Room{
		"start": {ID: "start", Title: "Start", Description: "", Exits: map[string]game.RoomID{}},
	})
	admin := newTestPlayer("Admin", "start")
	admin.IsAdmin = true
	world.AddPlayerForTest(admin)

	fake := &fakePortal{url: "https://example.com/portal/token", expires: time.Now().Add(2 * time.Minute)}
	world.AttachPortal(fake)

	if quit := Dispatch(world, admin, "portal moderator"); quit {
		t.Fatalf("dispatch returned true, want false")
	}
	msgs := strings.Join(drainOutput(admin.Output), "\n")
	if !strings.Contains(msgs, "Open portal") || !strings.Contains(msgs, fake.url) {
		t.Fatalf("expected portal link in output, got %q", msgs)
	}
	if fake.lastRole != game.PortalRoleModerator {
		t.Fatalf("portal requested role %q, want %q", fake.lastRole, game.PortalRoleModerator)
	}
}
