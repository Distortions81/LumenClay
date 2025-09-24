package commands

import "LumenClay/internal/game"

var BuildHelp = Define(Definition{
	Name:        "buildhelp",
	Usage:       "buildhelp",
	Description: "list building commands",
	Group:       GroupBuilder,
}, func(ctx *Context) bool {
	if !ctx.Player.IsBuilder && !ctx.Player.IsAdmin {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nOnly builders or admins may view building commands.", game.AnsiYellow))
		return false
	}
	cmds := commandsForGroup(GroupBuilder)
	ctx.Player.Output <- game.Ansi(helpMessage("Building Commands:", cmds))
	return false
})
