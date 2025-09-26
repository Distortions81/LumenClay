package commands

import (
	"fmt"
	"strings"

	"LumenClay/internal/game"
)

var Name = Define(Definition{
	Name:        "name",
	Usage:       "name <newname> | name room <title>",
	Description: "change your display name or rename the current room",
}, func(ctx *Context) bool {
	args := strings.TrimSpace(ctx.Arg)
	if args == "" {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nUsage: name <newname> | name room <title>", game.AnsiYellow))
		return false
	}

	fields := strings.Fields(args)
	if len(fields) > 0 && strings.EqualFold(fields[0], "room") {
		if !ctx.Player.IsAdmin && !ctx.Player.IsBuilder {
			ctx.Player.Output <- game.Ansi(game.Style("\r\nOnly builders or admins may rename rooms.", game.AnsiYellow))
			return false
		}
		if len(fields) == 1 {
			ctx.Player.Output <- game.Ansi(game.Style("\r\nUsage: name room <title>", game.AnsiYellow))
			return false
		}
		newTitle := strings.TrimSpace(strings.TrimPrefix(args, fields[0]))
		room, ok := ctx.World.GetRoom(ctx.Player.Room)
		if !ok {
			ctx.Player.Output <- game.Ansi(game.Style("\r\nYou are not in a valid room.", game.AnsiYellow))
			return false
		}
		if strings.TrimSpace(room.Title) == newTitle {
			ctx.Player.Output <- game.Ansi(game.Style("\r\nThe room already has that title.", game.AnsiYellow))
			return false
		}
		if _, err := ctx.World.UpdateRoomTitle(ctx.Player.Room, newTitle, ctx.Player.Name); err != nil {
			ctx.Player.Output <- game.Ansi(game.Style("\r\n"+err.Error(), game.AnsiYellow))
			return false
		}
		colored := game.Style(newTitle, game.AnsiCyan)
		ctx.World.BroadcastToRoom(ctx.Player.Room, game.Ansi(fmt.Sprintf("\r\n%s renames the room to %s.", game.HighlightName(ctx.Player.Name), colored)), ctx.Player)
		ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\nRoom name updated to %s.", colored))
		return false
	}

	if strings.ContainsAny(args, " \t\r\n") || len(args) > 24 {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nInvalid name.", game.AnsiYellow))
		return false
	}
	old := ctx.Player.Name
	if err := ctx.World.RenamePlayer(ctx.Player, args); err != nil {
		ctx.Player.Output <- game.Ansi(game.Style("\r\n"+err.Error(), game.AnsiYellow))
		return false
	}
	ctx.World.BroadcastToRoom(ctx.Player.Room, game.Ansi(fmt.Sprintf("\r\n%s is now known as %s.", game.HighlightName(old), game.HighlightName(args))), ctx.Player)
	ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\nYou are now known as %s.", game.HighlightName(args)))
	return false
})
