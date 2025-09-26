package game

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// OfflineTellLimitPerSender caps the number of queued messages from a single sender to a recipient.
const OfflineTellLimitPerSender = 5

// ErrOfflineTellLimit is returned when a sender has reached the queue limit for a recipient.
var ErrOfflineTellLimit = errors.New("offline tell limit reached")

// OfflineTell represents a private message stored while the recipient was offline.
type OfflineTell struct {
	Sender    string    `json:"sender"`
	Recipient string    `json:"recipient"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

// TellSystem persists offline tells for delivery when players return.
type TellSystem struct {
	mu    sync.RWMutex
	path  string
	queue map[string][]OfflineTell
}

// NewTellSystem constructs an offline tell manager backed by the provided file path.
// When path is empty the system operates purely in-memory without persistence.
func NewTellSystem(path string) (*TellSystem, error) {
	system := &TellSystem{
		path:  path,
		queue: make(map[string][]OfflineTell),
	}
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return system, nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return system, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read offline tells: %w", err)
	}
	if len(data) == 0 {
		return system, nil
	}
	var file struct {
		Queue map[string][]OfflineTell `json:"queue"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("decode offline tells: %w", err)
	}
	now := time.Now().UTC()
	for key, list := range file.Queue {
		normalized := normalizeTellKey(key)
		if normalized == "" {
			continue
		}
		sanitized := make([]OfflineTell, 0, len(list))
		for _, entry := range list {
			body := strings.TrimSpace(entry.Body)
			if body == "" {
				continue
			}
			sender := strings.TrimSpace(entry.Sender)
			recipient := strings.TrimSpace(entry.Recipient)
			if recipient == "" {
				recipient = key
			}
			created := entry.CreatedAt
			if created.IsZero() {
				created = now
			}
			sanitized = append(sanitized, OfflineTell{
				Sender:    sender,
				Recipient: recipient,
				Body:      body,
				CreatedAt: created.UTC(),
			})
		}
		if len(sanitized) == 0 {
			continue
		}
		sort.SliceStable(sanitized, func(i, j int) bool {
			return sanitized[i].CreatedAt.Before(sanitized[j].CreatedAt)
		})
		system.queue[normalized] = sanitized
	}
	return system, nil
}

// PendingFor returns a snapshot of queued tells for the specified recipient without removing them.
func (t *TellSystem) PendingFor(recipient string) []OfflineTell {
	key := normalizeTellKey(recipient)
	if key == "" {
		return nil
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	list := t.queue[key]
	if len(list) == 0 {
		return nil
	}
	out := make([]OfflineTell, len(list))
	copy(out, list)
	return out
}

// ConsumeFor retrieves and clears all queued tells for the specified recipient.
func (t *TellSystem) ConsumeFor(recipient string) []OfflineTell {
	key := normalizeTellKey(recipient)
	if key == "" {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	list := t.queue[key]
	if len(list) == 0 {
		return nil
	}
	snapshot := make([]OfflineTell, len(list))
	copy(snapshot, list)
	delete(t.queue, key)
	if err := t.persistLocked(); err != nil {
		t.queue[key] = list
		return nil
	}
	return snapshot
}

// Queue stores a new offline tell for the specified recipient.
func (t *TellSystem) Queue(sender, recipient, body string, when time.Time) (OfflineTell, error) {
	key := normalizeTellKey(recipient)
	if key == "" {
		return OfflineTell{}, fmt.Errorf("recipient is required")
	}
	trimmedBody := strings.TrimSpace(body)
	if trimmedBody == "" {
		return OfflineTell{}, fmt.Errorf("message cannot be empty")
	}
	trimmedSender := strings.TrimSpace(sender)
	trimmedRecipient := strings.TrimSpace(recipient)
	if when.IsZero() {
		when = time.Now().UTC()
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	existing := t.queue[key]
	count := 0
	for _, entry := range existing {
		if strings.EqualFold(entry.Sender, trimmedSender) {
			count++
		}
	}
	if count >= OfflineTellLimitPerSender {
		return OfflineTell{}, ErrOfflineTellLimit
	}
	cloned := make([]OfflineTell, len(existing))
	copy(cloned, existing)
	tell := OfflineTell{
		Sender:    trimmedSender,
		Recipient: trimmedRecipient,
		Body:      trimmedBody,
		CreatedAt: when.UTC(),
	}
	cloned = append(cloned, tell)
	t.queue[key] = cloned
	if err := t.persistLocked(); err != nil {
		t.queue[key] = existing
		return OfflineTell{}, err
	}
	return tell, nil
}

func (t *TellSystem) persistLocked() error {
	if t.queue == nil {
		t.queue = make(map[string][]OfflineTell)
	}
	if strings.TrimSpace(t.path) == "" {
		return nil
	}
	active := make(map[string][]OfflineTell, len(t.queue))
	for key, list := range t.queue {
		if len(list) == 0 {
			continue
		}
		copied := make([]OfflineTell, len(list))
		copy(copied, list)
		sort.SliceStable(copied, func(i, j int) bool {
			return copied[i].CreatedAt.Before(copied[j].CreatedAt)
		})
		active[key] = copied
	}
	if len(active) == 0 {
		if err := os.Remove(t.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove offline tells: %w", err)
		}
		return nil
	}
	dir := filepath.Dir(t.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create offline tells directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, "tells-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp offline tells file: %w", err)
	}
	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(struct {
		Queue map[string][]OfflineTell `json:"queue"`
	}{Queue: active}); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return fmt.Errorf("write offline tells: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("close offline tells file: %w", err)
	}
	if err := os.Rename(tmp.Name(), t.path); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("replace offline tells file: %w", err)
	}
	return nil
}

func normalizeTellKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
