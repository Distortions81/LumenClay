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

// Default retention configuration for stored offline tells.
const (
	// DefaultTellMaxAge defines how long tells are retained before they expire.
	// A zero duration disables age-based expiration.
	DefaultTellMaxAge = time.Duration(0)
	// DefaultTellMaxMessagesPerRecipient caps the number of stored tells per recipient.
	// A zero value disables the per-recipient cap.
	DefaultTellMaxMessagesPerRecipient = 0
)

// TellRetentionPolicy defines the retention rules applied to stored offline tells.
type TellRetentionPolicy struct {
	MaxAge                  time.Duration
	MaxMessagesPerRecipient int
}

func (p TellRetentionPolicy) normalized() TellRetentionPolicy {
	if p.MaxAge < 0 {
		p.MaxAge = 0
	}
	if p.MaxMessagesPerRecipient < 0 {
		p.MaxMessagesPerRecipient = 0
	}
	return p
}

// TellSystem persists offline tells for delivery when players return.
type TellSystem struct {
	mu     sync.RWMutex
	path   string
	queue  map[string][]OfflineTell
	policy TellRetentionPolicy
}

// NewTellSystem constructs an offline tell manager backed by the provided file path
// using the default retention policy. When path is empty the system operates purely
// in-memory without persistence.
func NewTellSystem(path string) (*TellSystem, error) {
	return NewTellSystemWithRetention(path, TellRetentionPolicy{
		MaxAge:                  DefaultTellMaxAge,
		MaxMessagesPerRecipient: DefaultTellMaxMessagesPerRecipient,
	})
}

// NewTellSystemWithRetention constructs an offline tell manager with the provided
// retention policy. When path is empty the system operates purely in-memory without
// persistence.
func NewTellSystemWithRetention(path string, policy TellRetentionPolicy) (*TellSystem, error) {
	normalized := policy.normalized()
	system := &TellSystem{
		path:   path,
		queue:  make(map[string][]OfflineTell),
		policy: normalized,
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
		pruned := system.applyRetention(sanitized, now)
		if len(pruned) == 0 {
			continue
		}
		system.queue[normalized] = pruned
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
	now := time.Now().UTC()
	existing := t.applyRetention(t.queue[key], now)
	if len(existing) == 0 {
		delete(t.queue, key)
	} else {
		t.queue[key] = existing
	}
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
	if when.IsZero() {
		tell.CreatedAt = now
	}
	cloned = append(cloned, tell)
	retained := t.applyRetention(cloned, now)
	if len(retained) == 0 {
		delete(t.queue, key)
	} else {
		t.queue[key] = retained
	}
	if err := t.persistLocked(); err != nil {
		if len(existing) == 0 {
			delete(t.queue, key)
		} else {
			t.queue[key] = existing
		}
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
	now := time.Now().UTC()
	for key, list := range t.queue {
		retained := t.applyRetention(list, now)
		if len(retained) == 0 {
			continue
		}
		copied := make([]OfflineTell, len(retained))
		copy(copied, retained)
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

func (t *TellSystem) applyRetention(list []OfflineTell, now time.Time) []OfflineTell {
	if len(list) == 0 {
		return nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	pruned := make([]OfflineTell, 0, len(list))
	cutoff := time.Time{}
	if t.policy.MaxAge > 0 {
		cutoff = now.Add(-t.policy.MaxAge)
	}
	for _, entry := range list {
		current := entry
		if current.CreatedAt.IsZero() {
			current.CreatedAt = now
		}
		if !cutoff.IsZero() && current.CreatedAt.Before(cutoff) {
			continue
		}
		pruned = append(pruned, current)
	}
	if len(pruned) == 0 {
		return nil
	}
	sort.SliceStable(pruned, func(i, j int) bool {
		return pruned[i].CreatedAt.Before(pruned[j].CreatedAt)
	})
	if t.policy.MaxMessagesPerRecipient > 0 && len(pruned) > t.policy.MaxMessagesPerRecipient {
		pruned = pruned[len(pruned)-t.policy.MaxMessagesPerRecipient:]
	}
	sanitized := make([]OfflineTell, len(pruned))
	copy(sanitized, pruned)
	return sanitized
}
