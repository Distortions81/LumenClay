package commands

import (
	"strings"

	"aiMud/internal/game"
)

var Describe = Define(Definition{
	Name:        "describe",
	Usage:       "describe <text>",
	Description: "update the current room description (builders/admins only)",
}, func(ctx *Context) bool {
	if !ctx.Player.IsAdmin && !ctx.Player.IsBuilder {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nOnly builders or admins may use describe.", game.AnsiYellow))
		return false
	}
	desc := strings.TrimSpace(ctx.Arg)
	if desc == "" {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nUsage: describe <text>", game.AnsiYellow))
		return false
	}
	if _, err := ctx.World.UpdateRoomDescription(ctx.Player.Room, desc); err != nil {
		ctx.Player.Output <- game.Ansi(game.Style("\r\n"+err.Error(), game.AnsiYellow))
		return false
	}
	ctx.Player.Output <- game.Ansi("\r\nRoom description updated.")
	return false
})
