package commands

import (
	"fmt"
	"strings"

	"LumenClay/internal/game"
)

var Channel = Define(Definition{
	Name:        "channel",
	Usage:       "channel <name> <on|off> | channel alias <name> <alias|clear>",
	Description: "manage channel filters and aliases",
}, func(ctx *Context) bool {
	trimmed := strings.TrimSpace(ctx.Arg)
	if trimmed == "" {
		sendChannelStatus(ctx.World, ctx.Player)
		return false
	}
	tokens := strings.Fields(ctx.Arg)
	if len(tokens) == 0 {
		sendChannelStatus(ctx.World, ctx.Player)
		return false
	}
	if strings.EqualFold(tokens[0], "alias") {
		return handleChannelAlias(ctx, ctx.Arg)
	}
	fields := make([]string, len(tokens))
	for i, token := range tokens {
		fields[i] = strings.ToLower(token)
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

func handleChannelAlias(ctx *Context, raw string) bool {
	fields := strings.Fields(raw)
	if len(fields) < 2 {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nUsage: channel alias <name> <alias|clear>", game.AnsiYellow))
		return false
	}
	if !strings.EqualFold(fields[0], "alias") {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nUsage: channel alias <name> <alias|clear>", game.AnsiYellow))
		return false
	}
	channel, ok := game.ChannelFromString(fields[1])
	if !ok {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nUnknown channel.", game.AnsiYellow))
		return false
	}
	if len(fields) == 2 {
		current := ctx.World.ChannelAlias(ctx.Player, channel)
		if current == "" {
			ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\n%s channel has no alias set.", strings.ToUpper(fields[1])))
			return false
		}
		ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\n%s channel alias is %s.", strings.ToUpper(fields[1]), game.Style(strings.ToUpper(current), game.AnsiCyan, game.AnsiBold)))
		return false
	}
	if len(fields) == 3 && strings.EqualFold(fields[2], "clear") {
		ctx.World.SetChannelAlias(ctx.Player, channel, "")
		ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\n%s channel alias cleared.", strings.ToUpper(fields[1])))
		return false
	}
	if len(fields) != 3 {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nAliases must be a single word without spaces.", game.AnsiYellow))
		return false
	}
	alias := fields[2]
	if len(alias) > 16 {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nAliases are limited to 16 characters.", game.AnsiYellow))
		return false
	}
	ctx.World.SetChannelAlias(ctx.Player, channel, alias)
	ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\n%s channel alias set to %s.", strings.ToUpper(fields[1]), game.Style(strings.ToUpper(alias), game.AnsiCyan, game.AnsiBold)))
	return false
}
