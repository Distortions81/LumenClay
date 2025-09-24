package commands

import (
	"fmt"
	"strings"

	"aiMud/internal/game"
)

var Summon = Define(Definition{
	Name:        "summon",
	Usage:       "summon <player>",
	Description: "summon a player to you (admin only)",
	Group:       GroupAdmin,
}, func(ctx *Context) bool {
	if !ctx.Player.IsAdmin {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nOnly admins may summon players.", game.AnsiYellow))
		return false
	}
	targetName := strings.TrimSpace(ctx.Arg)
	if targetName == "" {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nUsage: summon <player>", game.AnsiYellow))
		return false
	}
	target, ok := ctx.World.FindPlayer(targetName)
	if !ok {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nThey are not online.", game.AnsiYellow))
		return false
	}
	if target == ctx.Player {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nYou cannot summon yourself.", game.AnsiYellow))
		return false
	}
	if target.Room == ctx.Player.Room {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nThey are already here.", game.AnsiYellow))
		return false
	}
	previous := target.Room
	if err := ctx.World.MoveToRoom(target, ctx.Player.Room); err != nil {
		ctx.Player.Output <- game.Ansi(game.Style("\r\n"+err.Error(), game.AnsiYellow))
		return false
	}
	ctx.World.BroadcastToRoom(previous, game.Ansi(fmt.Sprintf("\r\n%s is yanked away by unseen forces.", game.HighlightName(target.Name))), target)
	ctx.World.BroadcastToRoom(ctx.Player.Room, game.Ansi(fmt.Sprintf("\r\n%s is summoned by %s.", game.HighlightName(target.Name), game.HighlightName(ctx.Player.Name))), target)
	target.Output <- game.Ansi(fmt.Sprintf("\r\nYou are summoned by %s.", game.HighlightName(ctx.Player.Name)))
	game.EnterRoom(ctx.World, target, "")
	ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\nYou summon %s to your side.", game.HighlightName(target.Name)))
	return false
})
