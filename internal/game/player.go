package game

import (
	"strings"
	"sync"
	"time"
)

// Player represents a connected adventurer in the world.
type Player struct {
	Name             string
	Account          string
	Session          *TelnetSession
	Room             RoomID
	Home             RoomID
	Output           chan string
	Alive            bool
	IsAdmin          bool
	IsBuilder        bool
	Channels         map[Channel]bool
	ChannelAliases   map[Channel]string
	Inventory        []Item
	JoinedAt         time.Time
	Level            int
	Experience       int
	Health           int
	MaxHealth        int
	Mana             int
	MaxMana          int
	history          []time.Time
	channelHistory   map[Channel][]ChannelLogEntry
	channelHistoryMu sync.Mutex
	MutedChannels    map[Channel]bool
}

// PlayerProfile captures persistent player state and preferences.
type PlayerProfile struct {
	Room     RoomID
	Home     RoomID
	Channels map[Channel]bool
	Aliases  map[Channel]string
}

const (
	commandLimit  = 5
	commandWindow = time.Second
)

const (
	// ChannelHistoryDefault is the default number of entries displayed by the history command.
	ChannelHistoryDefault = 10
	// ChannelHistoryLimit caps the number of retained messages per channel.
	ChannelHistoryLimit = 50
)

func (p *Player) allowCommand(now time.Time) bool {
	cutoff := now.Add(-commandWindow)
	filtered := p.history[:0]
	for _, t := range p.history {
		if t.After(cutoff) {
			filtered = append(filtered, t)
		}
	}
	p.history = filtered
	if len(p.history) >= commandLimit {
		return false
	}
	p.history = append(p.history, now)
	return true
}

func (p *Player) channelEnabled(channel Channel) bool {
	if p.Channels == nil {
		return true
	}
	enabled, ok := p.Channels[channel]
	if !ok {
		return true
	}
	return enabled
}

func (p *Player) channelAlias(channel Channel) string {
	if p.ChannelAliases == nil {
		return ""
	}
	return p.ChannelAliases[channel]
}

// EnsureStats normalizes the player's level, health, and mana pools.
func (p *Player) EnsureStats() {
	if p == nil {
		return
	}
	if p.Level < 1 {
		p.Level = 1
	}
	if p.MaxHealth <= 0 {
		p.MaxHealth = 50 + (p.Level-1)*10
	}
	if p.Health <= 0 || p.Health > p.MaxHealth {
		p.Health = p.MaxHealth
	}
	if p.MaxMana < 0 {
		p.MaxMana = 25 + (p.Level-1)*5
	}
	if p.Mana < 0 || p.Mana > p.MaxMana {
		p.Mana = p.MaxMana
	}
}

// AttackDamage estimates the base damage dealt by the player in melee combat.
func (p *Player) AttackDamage() int {
	p.EnsureStats()
	base := 5 + p.Level*2
	if base < 1 {
		return 1
	}
	return base
}

// GainExperience awards experience points and handles level progression.
// It returns the number of levels gained.
func (p *Player) GainExperience(amount int) int {
	if p == nil || amount <= 0 {
		return 0
	}
	p.EnsureStats()
	p.Experience += amount
	levelsGained := 0
	for {
		threshold := experienceForLevel(p.Level + 1)
		if p.Experience < threshold {
			break
		}
		p.Level++
		levelsGained++
		p.MaxHealth += 10
		p.MaxMana += 5
		p.Health = p.MaxHealth
		p.Mana = p.MaxMana
	}
	return levelsGained
}

func experienceForLevel(level int) int {
	if level <= 1 {
		return 0
	}
	return (level - 1) * 100
}

func (p *Player) setChannelAlias(channel Channel, alias string) {
	trimmed := strings.TrimSpace(alias)
	if p.ChannelAliases == nil {
		if trimmed == "" {
			return
		}
		p.ChannelAliases = make(map[Channel]string)
	}
	if trimmed == "" {
		delete(p.ChannelAliases, channel)
		return
	}
	p.ChannelAliases[channel] = trimmed
}

func (p *Player) rememberChannelMessage(channel Channel, message string, when time.Time) {
	if when.IsZero() {
		when = time.Now()
	}
	p.channelHistoryMu.Lock()
	if p.channelHistory == nil {
		p.channelHistory = make(map[Channel][]ChannelLogEntry)
	}
	entries := append(p.channelHistory[channel], ChannelLogEntry{Timestamp: when, Message: message, Channel: channel})
	if excess := len(entries) - ChannelHistoryLimit; excess > 0 {
		entries = append([]ChannelLogEntry(nil), entries[excess:]...)
	}
	p.channelHistory[channel] = entries
	p.channelHistoryMu.Unlock()
}

func (p *Player) snapshotChannelHistory(channel Channel, limit int) []ChannelLogEntry {
	if limit <= 0 || limit > ChannelHistoryLimit {
		limit = ChannelHistoryLimit
	}
	p.channelHistoryMu.Lock()
	defer p.channelHistoryMu.Unlock()
	entries := p.channelHistory[channel]
	if len(entries) == 0 {
		return nil
	}
	if len(entries) > limit {
		entries = entries[len(entries)-limit:]
	}
	out := make([]ChannelLogEntry, len(entries))
	copy(out, entries)
	return out
}

func (p *Player) muted(channel Channel) bool {
	if p.MutedChannels == nil {
		return false
	}
	return p.MutedChannels[channel]
}

// WindowSize returns the most recent NAWS dimensions reported by the player's
// telnet session, falling back to the standard 80x24 layout when unavailable.
func (p *Player) WindowSize() (int, int) {
	const (
		defaultWidth  = 80
		defaultHeight = 24
	)
	if p == nil || p.Session == nil {
		return defaultWidth, defaultHeight
	}
	width, height := p.Session.Size()
	if width <= 0 {
		width = defaultWidth
	}
	if height <= 0 {
		height = defaultHeight
	}
	return width, height
}

// ChannelLogEntry records a single message delivered via a chat channel.
type ChannelLogEntry struct {
	Timestamp time.Time
	Message   string
	Channel   Channel
}
