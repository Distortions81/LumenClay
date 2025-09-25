package commands

import (
	"fmt"

	"LumenClay/internal/game"
)

var Say = Define(Definition{
	Name:        "say",
	Usage:       "say <message>",
	Description: "chat to the room",
}, func(ctx *Context) bool {
	msg := ctx.Arg
	if msg == "" {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nSay what?", game.AnsiYellow))
		return false
	}
	if ctx.World.ChannelMuted(ctx.Player, game.ChannelSay) {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nYou are muted on SAY.", game.AnsiYellow))
		return false
	}
	broadcast := game.Ansi(fmt.Sprintf("\r\n%s says: %s", game.HighlightName(ctx.Player.Name), msg))
	ctx.World.BroadcastToRoomChannel(ctx.Player.Room, broadcast, ctx.Player, game.ChannelSay)
	self := game.Ansi(fmt.Sprintf("\r\n%s %s", game.Style("You say:", game.AnsiBold, game.AnsiYellow), msg))
	ctx.Player.Output <- self
	ctx.World.RecordPlayerChannelMessage(ctx.Player, game.ChannelSay, self)
	return false
})
