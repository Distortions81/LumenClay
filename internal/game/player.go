package game

import "time"

// Player represents a connected adventurer in the world.
type Player struct {
	Name      string
	Account   string
	Session   *TelnetSession
	Room      RoomID
	Home      RoomID
	Output    chan string
	Alive     bool
	IsAdmin   bool
	IsBuilder bool
	Channels  map[Channel]bool
	Inventory []Item
	JoinedAt  time.Time
	history   []time.Time
}

// PlayerProfile captures persistent player state and preferences.
type PlayerProfile struct {
	Room     RoomID
	Home     RoomID
	Channels map[Channel]bool
}

const (
	commandLimit  = 5
	commandWindow = time.Second
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
