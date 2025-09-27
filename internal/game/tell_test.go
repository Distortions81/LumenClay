package game

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTellSystemQueueAndConsume(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tells.json")
	system, err := NewTellSystem(path)
	if err != nil {
		t.Fatalf("NewTellSystem: %v", err)
	}
	when := time.Date(2025, time.January, 2, 15, 4, 5, 0, time.UTC)
	if _, err := system.Queue("Alice", "Bob", "See you soon", when); err != nil {
		t.Fatalf("Queue first: %v", err)
	}
	later := when.Add(time.Minute)
	if _, err := system.Queue("Charlie", "Bob", "Meet at the plaza", later); err != nil {
		t.Fatalf("Queue second: %v", err)
	}

	pending := system.PendingFor("Bob")
	if len(pending) != 2 {
		t.Fatalf("PendingFor returned %d entries, want 2", len(pending))
	}
	if pending[0].Body != "See you soon" || pending[1].Body != "Meet at the plaza" {
		t.Fatalf("pending order incorrect: %#v", pending)
	}

	consumed := system.ConsumeFor("Bob")
	if len(consumed) != 2 {
		t.Fatalf("ConsumeFor returned %d entries, want 2", len(consumed))
	}
	if system.PendingFor("Bob") != nil {
		t.Fatalf("PendingFor should be empty after consumption")
	}

	// Ensure persistence round-trips queued messages.
	if _, err := system.Queue("Dana", "Eli", "Stored", time.Time{}); err != nil {
		t.Fatalf("Queue third: %v", err)
	}
	reloaded, err := NewTellSystem(path)
	if err != nil {
		t.Fatalf("reload TellSystem: %v", err)
	}
	pending = reloaded.PendingFor("Eli")
	if len(pending) != 1 || pending[0].Sender != "Dana" || pending[0].Body != "Stored" {
		t.Fatalf("Reloaded queue incorrect: %#v", pending)
	}
}

func TestTellSystemEnforcesPerSenderLimit(t *testing.T) {
	system, err := NewTellSystem("")
	if err != nil {
		t.Fatalf("NewTellSystem: %v", err)
	}
	for i := 0; i < OfflineTellLimitPerSender; i++ {
		body := fmt.Sprintf("Message %d", i)
		if _, err := system.Queue("Sender", "Recipient", body, time.Time{}); err != nil {
			t.Fatalf("Queue #%d: %v", i+1, err)
		}
	}
	if _, err := system.Queue("Sender", "Recipient", "Too many", time.Time{}); !errors.Is(err, ErrOfflineTellLimit) {
		t.Fatalf("Queue beyond limit returned %v, want ErrOfflineTellLimit", err)
	}
	if _, err := system.Queue("Another", "Recipient", "Allowed", time.Time{}); err != nil {
		t.Fatalf("Queue from different sender: %v", err)
	}
}

func TestTellSystemRetentionPrunesOnLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tells.json")
	now := time.Now().UTC()
	payload := struct {
		Queue map[string][]OfflineTell `json:"queue"`
	}{Queue: map[string][]OfflineTell{
		"Bob": {
			{Sender: "Alice", Recipient: "Bob", Body: "Too old", CreatedAt: now.Add(-2 * time.Hour)},
			{Sender: "Charlie", Recipient: "Bob", Body: "Keep 1", CreatedAt: now.Add(-30 * time.Minute)},
			{Sender: "Dana", Recipient: "Bob", Body: "Keep 2", CreatedAt: now.Add(-10 * time.Minute)},
		},
	}}
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create tells file: %v", err)
	}
	if err := json.NewEncoder(file).Encode(payload); err != nil {
		t.Fatalf("encode tells: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close tells file: %v", err)
	}

	policy := TellRetentionPolicy{MaxAge: time.Hour, MaxMessagesPerRecipient: 2}
	system, err := NewTellSystemWithRetention(path, policy)
	if err != nil {
		t.Fatalf("NewTellSystemWithRetention: %v", err)
	}
	pending := system.PendingFor("Bob")
	if len(pending) != 2 {
		t.Fatalf("PendingFor returned %d entries, want 2", len(pending))
	}
	if pending[0].Body != "Keep 1" || pending[1].Body != "Keep 2" {
		t.Fatalf("PendingFor returned unexpected messages: %#v", pending)
	}
}

func TestTellSystemRetentionPrunesOnQueue(t *testing.T) {
	policy := TellRetentionPolicy{MaxAge: 10 * time.Millisecond, MaxMessagesPerRecipient: 2}
	system, err := NewTellSystemWithRetention("", policy)
	if err != nil {
		t.Fatalf("NewTellSystemWithRetention: %v", err)
	}
	if _, err := system.Queue("Alice", "Bob", "First", time.Time{}); err != nil {
		t.Fatalf("Queue first: %v", err)
	}
	// Ensure the first entry exceeds the retention age.
	time.Sleep(20 * time.Millisecond)
	if _, err := system.Queue("Charlie", "Bob", "Second", time.Time{}); err != nil {
		t.Fatalf("Queue second: %v", err)
	}
	pending := system.PendingFor("Bob")
	if len(pending) != 1 || pending[0].Body != "Second" {
		t.Fatalf("PendingFor after second queue incorrect: %#v", pending)
	}

	time.Sleep(20 * time.Millisecond)
	if _, err := system.Queue("Dana", "Bob", "Third", time.Time{}); err != nil {
		t.Fatalf("Queue third: %v", err)
	}
	pending = system.PendingFor("Bob")
	if len(pending) != 1 || pending[0].Body != "Third" {
		t.Fatalf("PendingFor after third queue incorrect: %#v", pending)
	}
}

func TestTellSystemRetentionPersistsPrunedEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tells.json")
	policy := TellRetentionPolicy{MaxMessagesPerRecipient: 2}
	system, err := NewTellSystemWithRetention(path, policy)
	if err != nil {
		t.Fatalf("NewTellSystemWithRetention: %v", err)
	}
	base := time.Date(2025, time.January, 1, 12, 0, 0, 0, time.UTC)
	if _, err := system.Queue("Alice", "Bob", "First", base); err != nil {
		t.Fatalf("Queue first: %v", err)
	}
	if _, err := system.Queue("Charlie", "Bob", "Second", base.Add(time.Minute)); err != nil {
		t.Fatalf("Queue second: %v", err)
	}
	if _, err := system.Queue("Dana", "Bob", "Third", base.Add(2*time.Minute)); err != nil {
		t.Fatalf("Queue third: %v", err)
	}

	reloaded, err := NewTellSystemWithRetention(path, policy)
	if err != nil {
		t.Fatalf("Reload TellSystemWithRetention: %v", err)
	}
	pending := reloaded.PendingFor("Bob")
	if len(pending) != 2 {
		t.Fatalf("Reloaded pending count = %d, want 2", len(pending))
	}
	if pending[0].Body != "Second" || pending[1].Body != "Third" {
		t.Fatalf("Reloaded pending unexpected: %#v", pending)
	}
}
