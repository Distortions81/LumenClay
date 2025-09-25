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
	others := game.FilterOut(names, ctx.Player.Name)
	if len(others) == 0 {
		ctx.Player.Output <- game.Ansi("\r\nYou are the only adventurer online.")
		return false
	}
	ctx.Player.Output <- game.Ansi("\r\nOther adventurers online: " + strings.Join(game.HighlightNames(others), ", "))
	return false
})
