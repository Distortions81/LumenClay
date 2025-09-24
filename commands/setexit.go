package commands

import (
	"strings"

	"aiMud/internal/game"
)

var SetExit = Define(Definition{
	Name:        "setexit",
	Usage:       "setexit <direction> <room|none>",
	Description: "connect the current room to another (builders/admins only)",
}, func(ctx *Context) bool {
	if !ctx.Player.IsAdmin && !ctx.Player.IsBuilder {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nOnly builders or admins may use setexit.", game.AnsiYellow))
		return false
	}
	parts := strings.Fields(ctx.Arg)
	if len(parts) != 2 {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nUsage: setexit <direction> <room|none>", game.AnsiYellow))
		return false
	}
	dir := parts[0]
	target := parts[1]
	if strings.EqualFold(target, "none") || strings.EqualFold(target, "remove") || strings.EqualFold(target, "clear") {
		if err := ctx.World.ClearExit(ctx.Player.Room, dir); err != nil {
			ctx.Player.Output <- game.Ansi(game.Style("\r\n"+err.Error(), game.AnsiYellow))
			return false
		}
		ctx.Player.Output <- game.Ansi("\r\nExit removed.")
		return false
	}
	if err := ctx.World.SetExit(ctx.Player.Room, dir, game.RoomID(target)); err != nil {
		ctx.Player.Output <- game.Ansi(game.Style("\r\n"+err.Error(), game.AnsiYellow))
		return false
	}
	ctx.Player.Output <- game.Ansi("\r\nExit updated.")
	return false
})
