package commands

import (
	"strings"
	"testing"

	"LumenClay/internal/game"
)

func TestHelpMentionsPortalForModerators(t *testing.T) {
	world := game.NewWorldWithRooms(map[game.RoomID]*game.Room{
		"start": {
			ID:          "start",
			Title:       "Start",
			Description: "Start room.",
			Exits:       map[string]game.RoomID{},
		},
	})
	moderator := newTestPlayer("Moderator", "start")
	moderator.IsModerator = true
	world.AddPlayerForTest(moderator)

	if quit := Dispatch(world, moderator, "help"); quit {
		t.Fatalf("dispatch returned true, want false")
	}

	msgs := drainOutput(moderator.Output)
	text := strings.Join(msgs, "\n")
	if !strings.Contains(text, "Moderators may type 'portal' to request a moderation portal link.") {
		t.Fatalf("help output missing moderator portal note: %v", msgs)
	}
}

func TestHelpOmitsPortalNoteForAdventurers(t *testing.T) {
	world := game.NewWorldWithRooms(map[game.RoomID]*game.Room{
		"start": {
			ID:          "start",
			Title:       "Start",
			Description: "Start room.",
			Exits:       map[string]game.RoomID{},
		},
	})
	player := newTestPlayer("Traveler", "start")
	world.AddPlayerForTest(player)

	if quit := Dispatch(world, player, "help"); quit {
		t.Fatalf("dispatch returned true, want false")
	}

	msgs := drainOutput(player.Output)
	text := strings.Join(msgs, "\n")
	if strings.Contains(text, "Moderators may type 'portal' to request a moderation portal link.") {
		t.Fatalf("unexpected moderator portal note for regular player: %v", msgs)
	}
}
