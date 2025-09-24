package commands

import (
	"strings"

	"LumenClay/internal/game"
)

var Clone = Define(Definition{
	Name:        "clone",
	Usage:       "clone <room id>",
	Description: "copy NPCs, items, and resets from another room (builders/admins only)",
	Group:       GroupBuilder,
}, func(ctx *Context) bool {
	if !ctx.Player.IsAdmin && !ctx.Player.IsBuilder {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nOnly builders or admins may clone rooms.", game.AnsiYellow))
		return false
	}
	target := strings.TrimSpace(ctx.Arg)
	if target == "" {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nUsage: clone <room id>", game.AnsiYellow))
		return false
	}
	if err := ctx.World.CloneRoomPopulation(game.RoomID(target), ctx.Player.Room); err != nil {
		ctx.Player.Output <- game.Ansi(game.Style("\r\n"+err.Error(), game.AnsiYellow))
		return false
	}
	ctx.Player.Output <- game.Ansi("\r\nRoom population cloned.")
	return false
})
