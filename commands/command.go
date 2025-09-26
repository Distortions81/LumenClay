package commands

import (
	"fmt"
	"strings"

	"LumenClay/internal/game"
)

var CommandToggle = Define(Definition{
	Name:        "command",
	Usage:       "command <name> <on|off>",
	Description: "enable or disable a command (admin only)",
	Group:       GroupAdmin,
}, func(ctx *Context) bool {
	if !ctx.Player.IsAdmin {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nOnly admins may manage commands.", game.AnsiYellow))
		return false
	}
	parts := strings.Fields(ctx.Arg)
	if len(parts) != 2 {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nUsage: command <name> <on|off>", game.AnsiYellow))
		return false
	}
	targetName := parts[0]
	toggle := strings.ToLower(parts[1])
	var enable bool
	switch toggle {
	case "on", "enable", "enabled", "true":
		enable = true
	case "off", "disable", "disabled", "false":
		enable = false
	default:
		ctx.Player.Output <- game.Ansi(game.Style("\r\nUsage: command <name> <on|off>", game.AnsiYellow))
		return false
	}

	target, ok := Find(targetName)
	if !ok || target == nil {
		ctx.Player.Output <- game.Ansi(game.Style(fmt.Sprintf("\r\nUnknown command: %s", targetName), game.AnsiYellow))
		return false
	}
	if strings.EqualFold(target.Name, "command") {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nThe command toggle cannot disable itself.", game.AnsiYellow))
		return false
	}

	disabled := ctx.World.CommandDisabled(target.Name)
	if enable {
		if !disabled {
			ctx.Player.Output <- game.Ansi(game.Style(fmt.Sprintf("\r\nCommand %s is already enabled.", target.Name), game.AnsiYellow))
			return false
		}
		ctx.World.SetCommandDisabled(target.Name, false)
		ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\nCommand %s is now enabled.", game.Style(target.Name, game.AnsiCyan)))
		return false
	}
	if disabled {
		ctx.Player.Output <- game.Ansi(game.Style(fmt.Sprintf("\r\nCommand %s is already disabled.", target.Name), game.AnsiYellow))
		return false
	}
	ctx.World.SetCommandDisabled(target.Name, true)
	ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\nCommand %s is now disabled.", game.Style(target.Name, game.AnsiYellow)))
	return false
})
