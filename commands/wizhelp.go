package commands

import "LumenClay/internal/game"

var WizHelp = Define(Definition{
	Name:        "wizhelp",
	Usage:       "wizhelp",
	Description: "list administrative commands",
	Group:       GroupAdmin,
}, func(ctx *Context) bool {
	if !ctx.Player.IsAdmin {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nOnly admins may view wizard commands.", game.AnsiYellow))
		return false
	}
	cmds := commandsForGroup(GroupAdmin)
	ctx.Player.Output <- game.Ansi(helpMessage("Admin Commands:", cmds))
	return false
})
