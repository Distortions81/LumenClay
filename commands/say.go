package commands

import (
	"fmt"

	"aiMud/internal/game"
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
	ctx.World.BroadcastToRoomChannel(ctx.Player.Room, game.Ansi(fmt.Sprintf("\r\n%s says: %s", game.HighlightName(ctx.Player.Name), msg)), ctx.Player, game.ChannelSay)
	ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\n%s %s", game.Style("You say:", game.AnsiBold, game.AnsiYellow), msg))
	return false
})
