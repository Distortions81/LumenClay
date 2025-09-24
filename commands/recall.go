package commands

import (
	"fmt"

	"aiMud/internal/game"
)

var Recall = Define(Definition{
	Name:        "recall",
	Usage:       "recall",
	Description: "return to your bound home",
}, func(ctx *Context) bool {
	destination := ctx.Player.Home
	if destination == "" {
		destination = game.StartRoom
	}
	if destination == ctx.Player.Room {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nYou are already home.", game.AnsiYellow))
		return false
	}
	if _, ok := ctx.World.GetRoom(destination); !ok {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nYour home has been lost to the void.", game.AnsiYellow))
		return false
	}
	prev := ctx.Player.Room
	if err := ctx.World.MoveToRoom(ctx.Player, destination); err != nil {
		ctx.Player.Output <- game.Ansi(game.Style("\r\n"+err.Error(), game.AnsiYellow))
		return false
	}
	ctx.World.BroadcastToRoom(prev, game.Ansi(fmt.Sprintf("\r\n%s is enveloped in a flash of light and vanishes.", game.HighlightName(ctx.Player.Name))), ctx.Player)
	ctx.World.BroadcastToRoom(destination, game.Ansi(fmt.Sprintf("\r\n%s arrives in a flash of light.", game.HighlightName(ctx.Player.Name))), ctx.Player)
	ctx.Player.Output <- game.Ansi("\r\nYou answer the call of home.")
	game.EnterRoom(ctx.World, ctx.Player, "")
	return false
})
