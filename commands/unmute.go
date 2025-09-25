package commands

import (
	"fmt"
	"strings"

	"LumenClay/internal/game"
)

var Unmute = Define(Definition{
	Name:        "unmute",
	Usage:       "unmute <player> <channel>",
	Description: "restore a player's access to a channel (admin only)",
	Group:       GroupAdmin,
}, func(ctx *Context) bool {
	if !ctx.Player.IsAdmin {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nOnly admins may unmute players.", game.AnsiYellow))
		return false
	}
	fields := strings.Fields(ctx.Arg)
	if len(fields) != 2 {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nUsage: unmute <player> <channel>", game.AnsiYellow))
		return false
	}
	target, ok := ctx.World.FindPlayer(fields[0])
	if !ok {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nThey are not online.", game.AnsiYellow))
		return false
	}
	channel, ok := game.ChannelFromString(fields[1])
	if !ok {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nUnknown channel.", game.AnsiYellow))
		return false
	}
	if !ctx.World.ChannelMuted(target, channel) {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nThey are not muted on that channel.", game.AnsiYellow))
		return false
	}
	ctx.World.SetChannelMute(target, channel, false)
	target.Output <- game.Ansi(fmt.Sprintf("\r\nYou are no longer muted on the %s channel.", strings.ToUpper(fields[1])))
	ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\nYou unmute %s on the %s channel.", game.HighlightName(target.Name), strings.ToUpper(fields[1])))
	return false
})
