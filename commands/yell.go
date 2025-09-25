package commands

import (
	"fmt"

	"LumenClay/internal/game"
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
	if ctx.World.ChannelMuted(ctx.Player, game.ChannelYell) {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nYou are muted on YELL.", game.AnsiYellow))
		return false
	}
	broadcast := game.Ansi(fmt.Sprintf("\r\n%s yells: %s", game.HighlightName(ctx.Player.Name), msg))
	ctx.World.BroadcastToAllChannel(broadcast, ctx.Player, game.ChannelYell)
	self := game.Ansi(fmt.Sprintf("\r\n%s %s", game.Style("You yell:", game.AnsiBold, game.AnsiYellow), msg))
	ctx.Player.Output <- self
	ctx.World.RecordPlayerChannelMessage(ctx.Player, game.ChannelYell, self)
	return false
})
