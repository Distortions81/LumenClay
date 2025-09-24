package commands

import (
	"fmt"
	"strings"

	"aiMud/internal/game"
)

var Channel = Define(Definition{
	Name:        "channel",
	Usage:       "channel <name> <on|off>",
	Description: "toggle channel filters",
}, func(ctx *Context) bool {
	fields := strings.Fields(strings.ToLower(ctx.Arg))
	if len(fields) == 0 {
		sendChannelStatus(ctx.World, ctx.Player)
		return false
	}
	if len(fields) != 2 {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nUsage: channel <name> <on|off>", game.AnsiYellow))
		return false
	}
	channel, ok := game.ChannelFromString(fields[0])
	if !ok {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nUnknown channel.", game.AnsiYellow))
		return false
	}
	switch fields[1] {
	case "on", "enable", "enabled":
		ctx.World.SetChannel(ctx.Player, channel, true)
		ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\n%s channel %s.", strings.ToUpper(fields[0]), game.Style("ON", game.AnsiGreen, game.AnsiBold)))
	case "off", "disable", "disabled":
		ctx.World.SetChannel(ctx.Player, channel, false)
		ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\n%s channel %s.", strings.ToUpper(fields[0]), game.Style("OFF", game.AnsiYellow)))
	default:
		ctx.Player.Output <- game.Ansi(game.Style("\r\nUsage: channel <name> <on|off>", game.AnsiYellow))
	}
	return false
})
