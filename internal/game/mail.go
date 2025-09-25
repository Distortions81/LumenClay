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

// MailMessage represents a single entry on a public board.
type MailMessage struct {
	ID         int       `json:"id"`
	Board      string    `json:"board"`
	Author     string    `json:"author"`
	Recipients []string  `json:"recipients,omitempty"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"created_at"`
}

// MailSystem manages persistent public board messages.
type MailSystem struct {
	mu     sync.RWMutex
	path   string
	nextID int
	boards map[string][]MailMessage
}

// NewMailSystem constructs a mail system backed by the provided file path.
// When path is empty the system operates purely in-memory without persistence.
func NewMailSystem(path string) (*MailSystem, error) {
	ms := &MailSystem{
		path:   path,
		nextID: 1,
		boards: make(map[string][]MailMessage),
	}
	if strings.TrimSpace(path) == "" {
		return ms, nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return ms, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read mail file: %w", err)
	}
	if len(data) == 0 {
		return ms, nil
	}
	var record struct {
		NextID int                      `json:"next_id"`
		Boards map[string][]MailMessage `json:"boards"`
	}
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("decode mail file: %w", err)
	}
	if record.Boards != nil {
		for name, messages := range record.Boards {
			board := normalizeBoard(name)
			if board == "" {
				continue
			}
			copied := make([]MailMessage, len(messages))
			for i, msg := range messages {
				copied[i] = sanitizeLoadedMessage(board, msg)
			}
			ms.boards[board] = copied
		}
	}
	if record.NextID > 0 {
		ms.nextID = record.NextID
	} else {
		ms.nextID = ms.computeNextID()
	}
	return ms, nil
}

func sanitizeLoadedMessage(board string, msg MailMessage) MailMessage {
	msg.Board = board
	msg.Recipients = normalizeRecipients(msg.Recipients)
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now().UTC()
	}
	if msg.ID == 0 {
		// ID will be recomputed by computeNextID if necessary.
	}
	msg.Body = strings.TrimSpace(msg.Body)
	return msg
}

func (m *MailSystem) computeNextID() int {
	next := 1
	for _, list := range m.boards {
		for _, msg := range list {
			if msg.ID >= next {
				next = msg.ID + 1
			}
		}
	}
	if next <= 0 {
		next = 1
	}
	return next
}

func normalizeBoard(name string) string {
	return strings.TrimSpace(strings.ToLower(name))
}

func normalizeRecipients(recipients []string) []string {
	if len(recipients) == 0 {
		return nil
	}
	out := make([]string, 0, len(recipients))
	seen := make(map[string]struct{}, len(recipients))
	for _, raw := range recipients {
		trimmed := strings.TrimSpace(raw)
		trimmed = strings.TrimPrefix(trimmed, "@")
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		if _, exists := seen[lower]; exists {
			continue
		}
		seen[lower] = struct{}{}
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// Boards returns the set of known board names sorted alphabetically.
func (m *MailSystem) Boards() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	boards := make([]string, 0, len(m.boards))
	for board := range m.boards {
		boards = append(boards, board)
	}
	sort.Strings(boards)
	return boards
}

// Messages returns a snapshot of the messages posted to the specified board.
func (m *MailSystem) Messages(board string) []MailMessage {
	key := normalizeBoard(board)
	m.mu.RLock()
	defer m.mu.RUnlock()
	list := m.boards[key]
	if len(list) == 0 {
		return nil
	}
	out := make([]MailMessage, len(list))
	copy(out, list)
	return out
}

// MessagesForPlayer returns only the messages from the board that mention the provided player.
// Messages without explicit recipients are considered visible to everyone.
func (m *MailSystem) MessagesForPlayer(board, player string) []MailMessage {
	messages := m.Messages(board)
	if len(messages) == 0 {
		return nil
	}
	if strings.TrimSpace(player) == "" {
		return messages
	}
	out := make([]MailMessage, 0, len(messages))
	for _, msg := range messages {
		if len(msg.Recipients) == 0 {
			out = append(out, msg)
			continue
		}
		for _, target := range msg.Recipients {
			if strings.EqualFold(target, player) {
				out = append(out, msg)
				break
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// Write stores a new message on the specified board.
func (m *MailSystem) Write(board, author string, recipients []string, body string) (MailMessage, error) {
	key := normalizeBoard(board)
	if key == "" {
		return MailMessage{}, fmt.Errorf("board name is required")
	}
	body = strings.TrimSpace(body)
	if body == "" {
		return MailMessage{}, fmt.Errorf("message body is required")
	}
	author = strings.TrimSpace(author)
	m.mu.Lock()
	defer m.mu.Unlock()
	msg := MailMessage{
		ID:         m.nextID,
		Board:      key,
		Author:     author,
		Recipients: normalizeRecipients(recipients),
		Body:       body,
		CreatedAt:  time.Now().UTC(),
	}
	if msg.ID <= 0 {
		msg.ID = m.computeNextID()
	}
	m.boards[key] = append(m.boards[key], msg)
	m.nextID = msg.ID + 1
	if err := m.saveLocked(); err != nil {
		// Revert the append when persistence fails.
		list := m.boards[key]
		m.boards[key] = list[:len(list)-1]
		m.nextID = msg.ID
		return MailMessage{}, err
	}
	return msg, nil
}

func (m *MailSystem) saveLocked() error {
	if strings.TrimSpace(m.path) == "" {
		return nil
	}
	dir := filepath.Dir(m.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create mail directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, "mail-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp mail file: %w", err)
	}
	record := struct {
		NextID int                      `json:"next_id"`
		Boards map[string][]MailMessage `json:"boards"`
	}{
		NextID: m.nextID,
		Boards: m.boards,
	}
	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(record); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return fmt.Errorf("write mail file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("close temp mail file: %w", err)
	}
	if err := os.Rename(tmp.Name(), m.path); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("replace mail file: %w", err)
	}
	return nil
}

// RecipientSummary returns a descriptive string for the message recipients.
func (msg MailMessage) RecipientSummary() string {
	if len(msg.Recipients) == 0 {
		return "everyone"
	}
	return strings.Join(msg.Recipients, ", ")
}

// AddressedTo returns true when the provided player is listed as a recipient.
func (msg MailMessage) AddressedTo(player string) bool {
	if len(msg.Recipients) == 0 {
		return true
	}
	for _, name := range msg.Recipients {
		if strings.EqualFold(name, player) {
			return true
		}
	}
	return false
}
