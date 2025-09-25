package commands

import "LumenClay/internal/game"

var Reboot = Define(Definition{
	Name:        "reboot",
	Usage:       "reboot",
	Description: "reload the world (admin only)",
	Group:       GroupAdmin,
}, func(ctx *Context) bool {
	if !ctx.Player.IsAdmin {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nOnly admins may reboot the world.", game.AnsiYellow))
		return false
	}
	if ctx.World.CriticalOperationsLocked() {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nWorld reboot is temporarily disabled.", game.AnsiYellow))
		return false
	}
	ctx.Player.Output <- game.Ansi(game.Style("\r\nRebooting the world...", game.AnsiMagenta, game.AnsiBold))
	players, err := ctx.World.Reboot()
	if err != nil {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nWorld reload failed: "+err.Error(), game.AnsiYellow))
		return false
	}
	for _, target := range players {
		target.Output <- game.Ansi(game.Style("\r\nReality shimmers as the world is rebooted.", game.AnsiMagenta))
		game.EnterRoom(ctx.World, target, "")
	}
	return false
})
