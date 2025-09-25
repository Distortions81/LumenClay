package commands

import (
	"fmt"
	"strings"

	"LumenClay/internal/game"
)

var Mute = Define(Definition{
	Name:        "mute",
	Usage:       "mute <player> <channel>",
	Description: "prevent a player from speaking on a channel (admin only)",
	Group:       GroupAdmin,
}, func(ctx *Context) bool {
	if !ctx.Player.IsAdmin {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nOnly admins may mute players.", game.AnsiYellow))
		return false
	}
	fields := strings.Fields(ctx.Arg)
	if len(fields) != 2 {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nUsage: mute <player> <channel>", game.AnsiYellow))
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
	if ctx.World.ChannelMuted(target, channel) {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nThey are already muted on that channel.", game.AnsiYellow))
		return false
	}
	ctx.World.SetChannelMute(target, channel, true)
	notice := fmt.Sprintf("\r\nYou have been muted on the %s channel by %s.", strings.ToUpper(fields[1]), game.HighlightName(ctx.Player.Name))
	target.Output <- game.Ansi(notice)
	ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\nYou mute %s on the %s channel.", game.HighlightName(target.Name), strings.ToUpper(fields[1])))
	return false
})
