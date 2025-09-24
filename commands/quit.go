package commands

import "aiMud/internal/game"

var Quit = Define(Definition{
	Name:        "quit",
	Aliases:     []string{"q"},
	Usage:       "quit",
	Description: "disconnect",
}, func(ctx *Context) bool {
	ctx.Player.Output <- game.Ansi("\r\nGoodbye.\r\n")
	return true
})
