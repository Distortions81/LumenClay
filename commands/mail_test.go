package commands

import (
	"strings"
	"testing"

	"LumenClay/internal/game"
)

func TestMailBoardsListsBoards(t *testing.T) {
	world := game.NewWorldWithRooms(map[game.RoomID]*game.Room{
		"start": {
			ID:          "start",
			Title:       "Start",
			Description: "A humble origin.",
			Exits:       map[string]game.RoomID{},
		},
	})
	mail, err := game.NewMailSystem("")
	if err != nil {
		t.Fatalf("NewMailSystem error: %v", err)
	}
	world.AttachMailSystem(mail)
	player := newTestPlayer("Hero", "start")
	world.AddPlayerForTest(player)
	if _, err := mail.Write("general", "Author", []string{"Hero"}, "Welcome"); err != nil {
		t.Fatalf("mail.Write error: %v", err)
	}

	if done := Dispatch(world, player, "mail boards"); done {
		t.Fatalf("dispatch returned true, want false")
	}
	output := drainOutput(player.Output)
	if len(output) == 0 {
		t.Fatalf("no output captured")
	}
	found := false
	for _, line := range output {
		if strings.Contains(line, "general") && strings.Contains(line, "for you") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected board listing mentioning personal posts: %v", output)
	}
}

func TestMailWriteCommandStoresPost(t *testing.T) {
	world := game.NewWorldWithRooms(map[game.RoomID]*game.Room{
		"start": {
			ID:          "start",
			Title:       "Start",
			Description: "A humble origin.",
			Exits:       map[string]game.RoomID{},
		},
	})
	mail, err := game.NewMailSystem("")
	if err != nil {
		t.Fatalf("NewMailSystem error: %v", err)
	}
	world.AttachMailSystem(mail)
	poster := newTestPlayer("Sage", "start")
	world.AddPlayerForTest(poster)

	if done := Dispatch(world, poster, "mail write general Hero = Meet me by the fountain."); done {
		t.Fatalf("dispatch returned true, want false")
	}
	output := drainOutput(poster.Output)
	sawConfirmation := false
	for _, line := range output {
		if strings.Contains(line, "You post to") && strings.Contains(line, "Hero") {
			sawConfirmation = true
			break
		}
	}
	if !sawConfirmation {
		t.Fatalf("expected confirmation message, got %v", output)
	}
	messages := mail.Messages("general")
	if len(messages) != 1 {
		t.Fatalf("message count = %d, want 1", len(messages))
	}
	if messages[0].Author != "Sage" {
		t.Fatalf("author = %q, want Sage", messages[0].Author)
	}
	if !messages[0].AddressedTo("Hero") {
		t.Fatalf("expected message to address Hero")
	}
}

func TestMailBoardShowsForYouMarker(t *testing.T) {
	world := game.NewWorldWithRooms(map[game.RoomID]*game.Room{
		"start": {
			ID:          "start",
			Title:       "Start",
			Description: "A humble origin.",
			Exits:       map[string]game.RoomID{},
		},
	})
	mail, err := game.NewMailSystem("")
	if err != nil {
		t.Fatalf("NewMailSystem error: %v", err)
	}
	world.AttachMailSystem(mail)
	poster := newTestPlayer("Sage", "start")
	hero := newTestPlayer("Hero", "start")
	world.AddPlayerForTest(poster)
	world.AddPlayerForTest(hero)
	if _, err := mail.Write("general", "Sage", []string{"Hero"}, "A personal note"); err != nil {
		t.Fatalf("mail.Write error: %v", err)
	}

	if done := Dispatch(world, hero, "mail board general"); done {
		t.Fatalf("dispatch returned true, want false")
	}
	output := drainOutput(hero.Output)
	seenMarker := false
	for _, line := range output {
		if strings.Contains(line, "(for you)") {
			seenMarker = true
			break
		}
	}
	if !seenMarker {
		t.Fatalf("expected '(for you)' marker in output: %v", output)
	}
}
