package commands

import (
	"fmt"

	"LumenClay/internal/game"
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
	if ctx.World.ChannelMuted(ctx.Player, game.ChannelWhisper) {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nYou are muted on WHISPER.", game.AnsiYellow))
		return false
	}
	broadcast := game.Ansi(fmt.Sprintf("\r\n%s whispers: %s", game.HighlightName(ctx.Player.Name), msg))
	ctx.World.BroadcastToRoomChannel(ctx.Player.Room, broadcast, ctx.Player, game.ChannelWhisper)
	nearby := ctx.World.AdjacentRooms(ctx.Player.Room)
	if len(nearby) > 0 {
		echo := game.Ansi(fmt.Sprintf("\r\nYou hear %s whisper from nearby: %s", game.HighlightName(ctx.Player.Name), msg))
		ctx.World.BroadcastToRoomsChannel(nearby, echo, ctx.Player, game.ChannelWhisper)
	}
	self := game.Ansi(fmt.Sprintf("\r\n%s %s", game.Style("You whisper:", game.AnsiBold, game.AnsiYellow), msg))
	ctx.Player.Output <- self
	ctx.World.RecordPlayerChannelMessage(ctx.Player, game.ChannelWhisper, self)
	return false
})
