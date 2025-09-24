package commands

import (
	"strings"

	"LumenClay/internal/game"
)

var Who = Define(Definition{
	Name:        "who",
	Usage:       "who",
	Description: "list connected players",
}, func(ctx *Context) bool {
	names := ctx.World.ListPlayers(false, "")
	ctx.Player.Output <- game.Ansi("\r\nPlayers: " + strings.Join(game.HighlightNames(names), ", "))
	return false
})
