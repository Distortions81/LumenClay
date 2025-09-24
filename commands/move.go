package commands

import (
	"strings"

	"aiMud/internal/game"
)

var Move = Define(Definition{
	Name:        "go",
	Aliases:     []string{"n", "s", "e", "w", "u", "d", "up", "down"},
	Shortcut:    "g",
	Usage:       "go <direction>",
	Description: "move (n/s/e/w/u/d and more)",
}, func(ctx *Context) bool {
	dir := ""
	switch strings.ToLower(ctx.Input) {
	case "n", "s", "e", "w", "u", "d":
		dir = strings.ToLower(ctx.Input)
	case "up":
		dir = "u"
	case "down":
		dir = "d"
	default:
		dir = strings.ToLower(strings.TrimSpace(ctx.Arg))
	}
	if dir == "" {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nUsage: go <direction>", game.AnsiYellow))
		return false
	}
	return move(ctx.World, ctx.Player, dir)
})
