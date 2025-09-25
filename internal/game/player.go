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

// ChannelLogEntry records a single message delivered via a chat channel.
type ChannelLogEntry struct {
	Timestamp time.Time
	Message   string
	Channel   Channel
}
