package commands

import (
	"fmt"
	"strings"

	"LumenClay/internal/game"
)

var Look = Define(Definition{
	Name:        "look",
	Aliases:     []string{"l"},
	Usage:       "look [target]",
	Description: "describe your surroundings or inspect a target",
}, func(ctx *Context) bool {
	room, ok := ctx.World.GetRoom(ctx.Player.Room)
	if !ok {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nYou see only void.", game.AnsiYellow))
		return false
	}

	width, _ := ctx.Player.WindowSize()

	target := strings.TrimSpace(ctx.Arg)
	if target != "" {
		if npc, found := ctx.World.FindRoomNPC(ctx.Player.Room, target); found {
			line := fmt.Sprintf("\r\n%s stands here.", game.HighlightNPCName(npc.Name))
			if greet := strings.TrimSpace(npc.AutoGreet); greet != "" {
				line = fmt.Sprintf("%s They say, \"%s\"", line, greet)
			}
			ctx.Player.Output <- game.Ansi(line)
			if offered := ctx.World.QuestsByNPC(npc.Name); len(offered) > 0 {
				if available := ctx.World.AvailableQuests(ctx.Player); len(available) > 0 {
					eligible := make(map[string]struct{}, len(available))
					for _, quest := range available {
						eligible[strings.ToLower(quest.ID)] = struct{}{}
					}
					var names []string
					for _, quest := range offered {
						if _, ok := eligible[strings.ToLower(quest.ID)]; !ok {
							continue
						}
						names = append(names, game.HighlightQuestName(quest.Name))
					}
					if len(names) > 0 {
						ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\nThey seem ready to offer: %s", strings.Join(names, ", ")))
						ctx.Player.Output <- game.Ansi("\r\nUse 'quests accept <id>' to begin.")
					}
				}
			}
			return false
		}
		if item, found := ctx.World.FindRoomItem(ctx.Player.Room, target); found {
			desc := strings.TrimSpace(item.Description)
			if desc == "" {
				desc = "You see nothing special."
			}
			ctx.Player.Output <- game.Ansi(fmt.Sprintf(
				"\r\nYou study %s. %s",
				game.HighlightItemName(item.Name),
				game.WrapText(desc, width),
			))
			return false
		}
		if dir, dest, found := ctx.World.ResolveExit(ctx.Player.Room, target); found {
			message := fmt.Sprintf("\r\nLooking %s you glimpse a passage.", dir)
			if next, ok := ctx.World.GetRoom(dest); ok {
				title := game.Style(next.Title, game.AnsiBold, game.AnsiCyan)
				desc := strings.TrimSpace(next.Description)
				if desc != "" {
					message = fmt.Sprintf(
						"\r\nLooking %s you glimpse %s. %s",
						dir,
						title,
						game.WrapText(desc, width),
					)
				} else {
					message = fmt.Sprintf("\r\nLooking %s you glimpse %s.", dir, title)
				}
			}
			ctx.Player.Output <- game.Ansi(message)
			return false
		}
		ctx.Player.Output <- game.Ansi("\r\nYou don't see that here.")
		return false
	}

	title := game.Style(room.Title, game.AnsiBold, game.AnsiCyan)
	desc := game.Style(game.WrapText(room.Description, width), game.AnsiItalic, game.AnsiDim)
	exits := game.Style(game.ExitList(room), game.AnsiGreen)
	ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\n%s\r\n%s\r\nExits: %s", title, desc, exits))

	others := ctx.World.ListPlayers(true, ctx.Player.Room)
	if len(others) > 1 {
		seen := game.FilterOut(others, ctx.Player.Name)
		colored := game.HighlightNames(seen)
		ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\nYou see: %s", strings.Join(colored, ", ")))
	}

	if npcs := ctx.World.RoomNPCs(ctx.Player.Room); len(npcs) > 0 {
		names := make([]string, len(npcs))
		for i, npc := range npcs {
			names[i] = game.HighlightNPCName(npc.Name)
		}
		ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\nYou notice: %s", strings.Join(names, ", ")))
	}

	if items := ctx.World.RoomItems(ctx.Player.Room); len(items) > 0 {
		names := make([]string, len(items))
		for i, item := range items {
			names[i] = game.HighlightItemName(item.Name)
		}
		ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\nOn the ground: %s", strings.Join(names, ", ")))
	}
	return false
})
