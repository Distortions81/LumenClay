package commands

import (
	"fmt"

	"aiMud/internal/game"
)

var OOC = Define(Definition{
	Name:        "ooc",
	Usage:       "ooc <message>",
	Description: "out-of-character chat",
}, func(ctx *Context) bool {
	msg := ctx.Arg
	if msg == "" {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nOOC what?", game.AnsiYellow))
		return false
	}
	tag := game.Style("[OOC]", game.AnsiMagenta, game.AnsiBold)
	ctx.World.BroadcastToAllChannel(game.Ansi(fmt.Sprintf("\r\n%s %s: %s", tag, game.HighlightName(ctx.Player.Name), msg)), ctx.Player, game.ChannelOOC)
	ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\n%s %s", game.Style("You (OOC):", game.AnsiBold, game.AnsiYellow), msg))
	return false
})
