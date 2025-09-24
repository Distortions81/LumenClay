package commands

import (
	"fmt"

	"aiMud/internal/game"
)

var Whisper = Define(Definition{
	Name:        "whisper",
	Usage:       "whisper <message>",
	Description: "whisper to nearby rooms",
}, func(ctx *Context) bool {
	msg := ctx.Arg
	if msg == "" {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nWhisper what?", game.AnsiYellow))
		return false
	}
	ctx.World.BroadcastToRoomChannel(ctx.Player.Room, game.Ansi(fmt.Sprintf("\r\n%s whispers: %s", game.HighlightName(ctx.Player.Name), msg)), ctx.Player, game.ChannelWhisper)
	nearby := ctx.World.AdjacentRooms(ctx.Player.Room)
	if len(nearby) > 0 {
		ctx.World.BroadcastToRoomsChannel(nearby, game.Ansi(fmt.Sprintf("\r\nYou hear %s whisper from nearby: %s", game.HighlightName(ctx.Player.Name), msg)), ctx.Player, game.ChannelWhisper)
	}
	ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\n%s %s", game.Style("You whisper:", game.AnsiBold, game.AnsiYellow), msg))
	return false
})
