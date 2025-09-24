package commands

import (
	"fmt"
	"strings"

	"aiMud/internal/game"
)

var Builder = Define(Definition{
	Name:        "builder",
	Usage:       "builder <player> <on|off>",
	Description: "grant or revoke builder rights (admin only)",
}, func(ctx *Context) bool {
	if !ctx.Player.IsAdmin {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nOnly admins may manage builders.", game.AnsiYellow))
		return false
	}
	parts := strings.Fields(ctx.Arg)
	if len(parts) != 2 {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nUsage: builder <player> <on|off>", game.AnsiYellow))
		return false
	}
	targetName := parts[0]
	toggle := strings.ToLower(parts[1])
	var enable bool
	switch toggle {
	case "on", "enable", "enabled", "true", "grant":
		enable = true
	case "off", "disable", "disabled", "false", "revoke":
		enable = false
	default:
		ctx.Player.Output <- game.Ansi(game.Style("\r\nUsage: builder <player> <on|off>", game.AnsiYellow))
		return false
	}
	target, err := ctx.World.SetBuilder(targetName, enable)
	if err != nil {
		ctx.Player.Output <- game.Ansi(game.Style("\r\n"+err.Error(), game.AnsiYellow))
		return false
	}
	state := "no longer"
	if enable {
		state = "now"
	}
	ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\n%s is %s a builder.", game.HighlightName(target.Name), state))
	notice := "\r\nYou are now a builder."
	if !enable {
		notice = "\r\nYou are no longer a builder."
	}
	target.Output <- game.Ansi(notice)
	return false
})
