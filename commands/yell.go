package commands

import (
	"fmt"

	"aiMud/internal/game"
)

var Yell = Define(Definition{
	Name:        "yell",
	Usage:       "yell <message>",
	Description: "yell to everyone",
}, func(ctx *Context) bool {
	msg := ctx.Arg
	if msg == "" {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nYell what?", game.AnsiYellow))
		return false
	}
	ctx.World.BroadcastToAllChannel(game.Ansi(fmt.Sprintf("\r\n%s yells: %s", game.HighlightName(ctx.Player.Name), msg)), ctx.Player, game.ChannelYell)
	ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\n%s %s", game.Style("You yell:", game.AnsiBold, game.AnsiYellow), msg))
	return false
})
