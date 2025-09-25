package commands

import (
	"fmt"

	"LumenClay/internal/game"
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
	if ctx.World.ChannelMuted(ctx.Player, game.ChannelOOC) {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nYou are muted on OOC.", game.AnsiYellow))
		return false
	}
	tag := game.Style("[OOC]", game.AnsiMagenta, game.AnsiBold)
	broadcast := game.Ansi(fmt.Sprintf("\r\n%s %s: %s", tag, game.HighlightName(ctx.Player.Name), msg))
	ctx.World.BroadcastToAllChannel(broadcast, ctx.Player, game.ChannelOOC)
	self := game.Ansi(fmt.Sprintf("\r\n%s %s", game.Style("You (OOC):", game.AnsiBold, game.AnsiYellow), msg))
	ctx.Player.Output <- self
	ctx.World.RecordPlayerChannelMessage(ctx.Player, game.ChannelOOC, self)
	return false
})
