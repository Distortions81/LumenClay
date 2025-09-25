package commands

import (
	"fmt"
	"strconv"
	"strings"

	"LumenClay/internal/game"
)

var History = Define(Definition{
	Name:        "history",
	Usage:       "history <channel> [count]",
	Description: "show recent channel messages",
}, func(ctx *Context) bool {
	fields := strings.Fields(ctx.Arg)
	if len(fields) == 0 {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nUsage: history <channel> [count]", game.AnsiYellow))
		return false
	}
	channelToken := fields[0]
	channel, ok := ctx.World.ResolveChannelToken(ctx.Player, channelToken)
	if !ok {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nUnknown channel.", game.AnsiYellow))
		return false
	}
	limit := game.ChannelHistoryDefault
	if len(fields) > 1 {
		count, err := strconv.Atoi(fields[1])
		if err != nil || count <= 0 {
			ctx.Player.Output <- game.Ansi(game.Style("\r\nHistory count must be a positive number.", game.AnsiYellow))
			return false
		}
		if count > game.ChannelHistoryLimit {
			count = game.ChannelHistoryLimit
		}
		limit = count
	}
	entries := ctx.World.ChannelHistory(ctx.Player, channel, limit)
	if len(entries) == 0 {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nNo messages recorded for that channel yet.", game.AnsiYellow))
		return false
	}
	label := strings.ToUpper(string(channel))
	if alias := ctx.World.ChannelAlias(ctx.Player, channel); alias != "" {
		label = fmt.Sprintf("%s (%s)", label, strings.ToUpper(alias))
	}
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("\r\nRecent %s messages:\r\n", label))
	for _, entry := range entries {
		stamp := entry.Timestamp.Format("15:04:05")
		clean := strings.TrimPrefix(entry.Message, "\r\n")
		clean = strings.TrimSuffix(clean, "\r\n")
		builder.WriteString(fmt.Sprintf("  [%s] %s\r\n", stamp, clean))
	}
	ctx.Player.Output <- game.Ansi(builder.String())
	return false
})
