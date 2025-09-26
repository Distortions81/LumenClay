package commands

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"LumenClay/internal/game"
)

func TestTellCommandSendsToOnlinePlayer(t *testing.T) {
	world := game.NewWorldWithRooms(map[game.RoomID]*game.Room{
		"hall": {
			ID:          "hall",
			Title:       "Hall",
			Description: "An empty hall.",
			Exits:       map[string]game.RoomID{},
		},
	})
	speaker := newTestPlayer("Speaker", "hall")
	listener := newTestPlayer("Listener", "hall")
	world.AddPlayerForTest(speaker)
	world.AddPlayerForTest(listener)

	if quit := Dispatch(world, speaker, "tell listener Hello there"); quit {
		t.Fatalf("dispatch returned true, want false")
	}

	speakerMsgs := drainOutput(speaker.Output)
	if len(speakerMsgs) == 0 || speakerMsgs[len(speakerMsgs)-1] != "You tell Listener: Hello there" {
		t.Fatalf("unexpected speaker output: %v", speakerMsgs)
	}
	listenerMsgs := drainOutput(listener.Output)
	if len(listenerMsgs) == 0 || listenerMsgs[len(listenerMsgs)-1] != "Speaker tells you: Hello there" {
		t.Fatalf("unexpected listener output: %v", listenerMsgs)
	}
}

func TestTellCommandQueuesOfflineMessage(t *testing.T) {
	world := game.NewWorldWithRooms(map[game.RoomID]*game.Room{
		"hall": {
			ID:          "hall",
			Title:       "Hall",
			Description: "An empty hall.",
			Exits:       map[string]game.RoomID{},
		},
	})
	dir := t.TempDir()
	accounts, err := game.NewAccountManager(filepath.Join(dir, "accounts.json"))
	if err != nil {
		t.Fatalf("NewAccountManager: %v", err)
	}
	if err := accounts.Register("Listener", "password"); err != nil {
		t.Fatalf("register listener: %v", err)
	}
	world.AttachAccountManager(accounts)
	tells, err := game.NewTellSystem("")
	if err != nil {
		t.Fatalf("NewTellSystem: %v", err)
	}
	world.AttachTellSystem(tells)

	speaker := newTestPlayer("Speaker", "hall")
	world.AddPlayerForTest(speaker)

	if quit := Dispatch(world, speaker, "tell listener Catch you later"); quit {
		t.Fatalf("dispatch returned true, want false")
	}
	speakerMsgs := drainOutput(speaker.Output)
	if len(speakerMsgs) == 0 || speakerMsgs[len(speakerMsgs)-1] != "You queue an offline tell for Listener: Catch you later" {
		t.Fatalf("unexpected speaker output: %v", speakerMsgs)
	}
	pending := tells.PendingFor("Listener")
	if len(pending) != 1 || pending[0].Sender != "Speaker" || pending[0].Body != "Catch you later" {
		t.Fatalf("unexpected queued tell: %#v", pending)
	}
}

func TestTellCommandRespectsSenderLimit(t *testing.T) {
	world := game.NewWorldWithRooms(map[game.RoomID]*game.Room{
		"hall": {
			ID:          "hall",
			Title:       "Hall",
			Description: "An empty hall.",
			Exits:       map[string]game.RoomID{},
		},
	})
	dir := t.TempDir()
	accounts, err := game.NewAccountManager(filepath.Join(dir, "accounts.json"))
	if err != nil {
		t.Fatalf("NewAccountManager: %v", err)
	}
	if err := accounts.Register("Listener", "password"); err != nil {
		t.Fatalf("register listener: %v", err)
	}
	world.AttachAccountManager(accounts)
	tells, err := game.NewTellSystem("")
	if err != nil {
		t.Fatalf("NewTellSystem: %v", err)
	}
	world.AttachTellSystem(tells)
	for i := 0; i < game.OfflineTellLimitPerSender; i++ {
		body := fmt.Sprintf("Message %d", i)
		if _, err := tells.Queue("Speaker", "Listener", body, time.Time{}); err != nil {
			t.Fatalf("seed queue #%d: %v", i+1, err)
		}
	}
	speaker := newTestPlayer("Speaker", "hall")
	world.AddPlayerForTest(speaker)

	if quit := Dispatch(world, speaker, "tell listener Another"); quit {
		t.Fatalf("dispatch returned true, want false")
	}
	speakerMsgs := drainOutput(speaker.Output)
	if len(speakerMsgs) == 0 || speakerMsgs[len(speakerMsgs)-1] != fmt.Sprintf("You already have %d offline tells queued for Listener.", game.OfflineTellLimitPerSender) {
		t.Fatalf("unexpected speaker output: %v", speakerMsgs)
	}
}
