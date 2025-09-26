package game

import (
	"fmt"
	"sort"
	"strings"
)

// EnterRoom places the player into their current room and sends the
// appropriate descriptions and arrival notifications.
func EnterRoom(world *World, p *Player, via string) {
	r, ok := world.GetRoom(p.Room)
	if !ok {
		p.Output <- Ansi(Style("\r\nYou seem to be nowhere.", AnsiYellow))
		return
	}
	width, _ := p.WindowSize()
	if via != "" {
		world.BroadcastToRoom(p.Room, Ansi(fmt.Sprintf("\r\n%s arrives from %s.", HighlightName(p.Name), via)), p)
	}
	title := Style(r.Title, AnsiBold, AnsiCyan)
	desc := Style(WrapText(r.Description, width), AnsiItalic, AnsiDim)
	exits := Style(ExitList(r), AnsiGreen)
	p.Output <- Ansi(fmt.Sprintf("\r\n\r\n%s\r\n%s\r\nExits: %s", title, desc, exits))
	others := world.ListPlayers(true, p.Room)
	if len(others) > 1 {
		seen := FilterOut(others, p.Name)
		colored := HighlightNames(seen)
		p.Output <- Ansi(fmt.Sprintf("\r\nYou see: %s", strings.Join(colored, ", ")))
	}
	if items := world.RoomItems(p.Room); len(items) > 0 {
		names := make([]string, len(items))
		for i, item := range items {
			names[i] = HighlightItemName(item.Name)
		}
		p.Output <- Ansi(fmt.Sprintf("\r\nOn the ground: %s", strings.Join(names, ", ")))
	}
	if len(r.NPCs) > 0 {
		for _, npc := range r.NPCs {
			if strings.TrimSpace(npc.AutoGreet) == "" {
				continue
			}
			msg := fmt.Sprintf("\r\n%s says, \"%s\"", HighlightNPCName(npc.Name), npc.AutoGreet)
			p.Output <- Ansi(msg)
		}
	}
	p.Output <- Prompt(p)
}

// ExitList renders the exits for a room in a deterministic order.
func ExitList(r *Room) string {
	if len(r.Exits) == 0 {
		return "none"
	}
	keys := make([]string, 0, len(r.Exits))
	for k := range r.Exits {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, " ")
}

// FilterOut returns a copy of list without the provided name.
func FilterOut(list []string, name string) []string {
	out := make([]string, 0, len(list))
	for _, v := range list {
		if v != name {
			out = append(out, v)
		}
	}
	return out
}
