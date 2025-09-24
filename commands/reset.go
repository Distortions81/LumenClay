package commands

import (
	"fmt"
	"strings"

	"LumenClay/internal/game"
)

var Reset = Define(Definition{
	Name:        "reset",
	Usage:       "reset <add|remove|list|apply> ...",
	Description: "manage room population resets (builders/admins only)",
	Group:       GroupBuilder,
}, func(ctx *Context) bool {
	if !ctx.Player.IsAdmin && !ctx.Player.IsBuilder {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nOnly builders or admins may manage resets.", game.AnsiYellow))
		return false
	}
	arg := strings.TrimSpace(ctx.Arg)
	if arg == "" {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nUsage: reset <add|remove|list|apply> ...", game.AnsiYellow))
		return false
	}
	word := func(input string) (string, string) {
		trimmed := strings.TrimSpace(input)
		if trimmed == "" {
			return "", ""
		}
		if idx := strings.IndexAny(trimmed, " \t"); idx >= 0 {
			return trimmed[:idx], strings.TrimSpace(trimmed[idx+1:])
		}
		return trimmed, ""
	}
	nameAndValue := func(input string) (string, string) {
		trimmed := strings.TrimSpace(input)
		if trimmed == "" {
			return "", ""
		}
		if before, after, ok := strings.Cut(trimmed, "="); ok {
			return strings.TrimSpace(before), strings.TrimSpace(after)
		}
		return trimmed, ""
	}

	action, rest := word(arg)
	action = strings.ToLower(action)
	switch action {
	case "add":
		kind, remainder := word(rest)
		kind = strings.ToLower(kind)
		switch kind {
		case "npc":
			name, greet := nameAndValue(remainder)
			if strings.TrimSpace(name) == "" {
				ctx.Player.Output <- game.Ansi(game.Style("\r\nUsage: reset add npc <name> [= auto greet]", game.AnsiYellow))
				return false
			}
			if _, err := ctx.World.UpsertRoomNPC(ctx.Player.Room, name, greet); err != nil {
				ctx.Player.Output <- game.Ansi(game.Style("\r\n"+err.Error(), game.AnsiYellow))
				return false
			}
			msg := fmt.Sprintf("\r\nNPC %s defined.", game.HighlightNPCName(strings.TrimSpace(name)))
			ctx.Player.Output <- game.Ansi(msg)
			return false
		case "item":
			name, desc := nameAndValue(remainder)
			if strings.TrimSpace(name) == "" {
				ctx.Player.Output <- game.Ansi(game.Style("\r\nUsage: reset add item <name> [= description]", game.AnsiYellow))
				return false
			}
			if _, err := ctx.World.UpsertRoomItemReset(ctx.Player.Room, name, desc); err != nil {
				ctx.Player.Output <- game.Ansi(game.Style("\r\n"+err.Error(), game.AnsiYellow))
				return false
			}
			msg := fmt.Sprintf("\r\nItem spawner %s defined.", game.HighlightItemName(strings.TrimSpace(name)))
			ctx.Player.Output <- game.Ansi(msg)
			return false
		default:
			ctx.Player.Output <- game.Ansi(game.Style("\r\nUsage: reset add <npc|item> ...", game.AnsiYellow))
			return false
		}
	case "remove":
		kind, remainder := word(rest)
		kind = strings.ToLower(kind)
		name := strings.TrimSpace(remainder)
		if name == "" {
			ctx.Player.Output <- game.Ansi(game.Style("\r\nUsage: reset remove <npc|item> <name>", game.AnsiYellow))
			return false
		}
		switch kind {
		case "npc":
			if err := ctx.World.RemoveRoomNPC(ctx.Player.Room, name); err != nil {
				ctx.Player.Output <- game.Ansi(game.Style("\r\n"+err.Error(), game.AnsiYellow))
				return false
			}
			msg := fmt.Sprintf("\r\nRemoved NPC %s.", game.HighlightNPCName(name))
			ctx.Player.Output <- game.Ansi(msg)
			return false
		case "item":
			if err := ctx.World.RemoveRoomItemReset(ctx.Player.Room, name); err != nil {
				ctx.Player.Output <- game.Ansi(game.Style("\r\n"+err.Error(), game.AnsiYellow))
				return false
			}
			msg := fmt.Sprintf("\r\nRemoved item spawner %s.", game.HighlightItemName(name))
			ctx.Player.Output <- game.Ansi(msg)
			return false
		default:
			ctx.Player.Output <- game.Ansi(game.Style("\r\nUsage: reset remove <npc|item> <name>", game.AnsiYellow))
			return false
		}
	case "list":
		resets := ctx.World.RoomResets(ctx.Player.Room)
		if len(resets) == 0 {
			ctx.Player.Output <- game.Ansi("\r\nNo resets defined for this room.")
			return false
		}
		lines := make([]string, 0, len(resets))
		for _, reset := range resets {
			switch reset.Kind {
			case game.ResetKindNPC:
				entry := fmt.Sprintf("NPC %s", game.HighlightNPCName(reset.Name))
				if strings.TrimSpace(reset.AutoGreet) != "" {
					entry = fmt.Sprintf("%s — \"%s\"", entry, reset.AutoGreet)
				}
				lines = append(lines, entry)
			case game.ResetKindItem:
				entry := fmt.Sprintf("Item %s", game.HighlightItemName(reset.Name))
				if reset.Count > 1 {
					entry = fmt.Sprintf("%s (x%d)", entry, reset.Count)
				}
				if strings.TrimSpace(reset.Description) != "" {
					entry = fmt.Sprintf("%s — %s", entry, reset.Description)
				}
				lines = append(lines, entry)
			}
		}
		ctx.Player.Output <- game.Ansi("\r\n" + strings.Join(lines, "\r\n"))
		return false
	case "apply":
		if err := ctx.World.ApplyRoomResets(ctx.Player.Room); err != nil {
			ctx.Player.Output <- game.Ansi(game.Style("\r\n"+err.Error(), game.AnsiYellow))
			return false
		}
		ctx.Player.Output <- game.Ansi("\r\nRoom resets applied.")
		return false
	default:
		ctx.Player.Output <- game.Ansi(game.Style("\r\nUsage: reset <add|remove|list|apply> ...", game.AnsiYellow))
		return false
	}
})
