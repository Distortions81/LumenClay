package game

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMailSystemWriteAndPersist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mail.json")
	mail, err := NewMailSystem(path)
	if err != nil {
		t.Fatalf("NewMailSystem error: %v", err)
	}
	if _, err := mail.Write("general", "Author", []string{"Hero", "Sage"}, "Welcome to the world!"); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected mail file to exist: %v", err)
	}
	boards := mail.Boards()
	if len(boards) != 1 || boards[0] != "general" {
		t.Fatalf("Boards() = %v, want [general]", boards)
	}
	msgs := mail.Messages("general")
	if len(msgs) != 1 {
		t.Fatalf("Messages len = %d, want 1", len(msgs))
	}
	if msgs[0].ID != 1 {
		t.Fatalf("msg ID = %d, want 1", msgs[0].ID)
	}
	if !msgs[0].AddressedTo("hero") {
		t.Fatalf("expected message to address hero")
	}

	reloaded, err := NewMailSystem(path)
	if err != nil {
		t.Fatalf("reload NewMailSystem error: %v", err)
	}
	msgs = reloaded.Messages("general")
	if len(msgs) != 1 {
		t.Fatalf("reloaded message count = %d, want 1", len(msgs))
	}
	if msgs[0].ID != 1 {
		t.Fatalf("reloaded ID = %d, want 1", msgs[0].ID)
	}
	if reloaded.nextID != 2 {
		t.Fatalf("nextID = %d, want 2", reloaded.nextID)
	}
}

func TestMailSystemMessagesForPlayerFilters(t *testing.T) {
	mail, err := NewMailSystem("")
	if err != nil {
		t.Fatalf("NewMailSystem error: %v", err)
	}
	if _, err := mail.Write("general", "Author", nil, "For everyone"); err != nil {
		t.Fatalf("write public message: %v", err)
	}
	if _, err := mail.Write("general", "Author", []string{"Hero"}, "Private to Hero"); err != nil {
		t.Fatalf("write private message: %v", err)
	}
	heroMsgs := mail.MessagesForPlayer("general", "Hero")
	if len(heroMsgs) != 2 {
		t.Fatalf("heroMsgs len = %d, want 2", len(heroMsgs))
	}
	sageMsgs := mail.MessagesForPlayer("general", "Sage")
	if len(sageMsgs) != 1 {
		t.Fatalf("sageMsgs len = %d, want 1", len(sageMsgs))
	}
}
