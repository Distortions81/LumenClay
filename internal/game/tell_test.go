package game

import (
	"errors"
	"fmt"
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
