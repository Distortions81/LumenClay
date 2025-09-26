package commands

import (
	"fmt"
	"strings"

	"LumenClay/internal/game"
)

var Teleport = Define(Definition{
	Name:        "teleport",
	Usage:       "teleport <room|player>",
	Description: "teleport to a room or player (builders/admins only)",
	Group:       GroupBuilder,
}, func(ctx *Context) bool {
	if !ctx.Player.IsAdmin && !ctx.Player.IsBuilder {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nOnly builders or admins may use teleport.", game.AnsiYellow))
		return false
	}
	target := strings.TrimSpace(ctx.Arg)
	if target == "" {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nUsage: teleport <room|player>", game.AnsiYellow))
		return false
	}

	destination := game.RoomID("")
	arrival := ""
	if player, ok := ctx.World.FindPlayer(target); ok {
		destination = player.Room
		arrival = fmt.Sprintf("\r\n%s appears in a shimmer of light next to %s.", game.HighlightName(ctx.Player.Name), game.HighlightName(player.Name))
	} else {
		destination = game.RoomID(target)
		if _, ok := ctx.World.GetRoom(destination); !ok {
			ctx.Player.Output <- game.Ansi(game.Style("\r\nNo such room or player.", game.AnsiYellow))
			return false
		}
		arrival = fmt.Sprintf("\r\n%s appears in a shimmer of light.", game.HighlightName(ctx.Player.Name))
	}

	previous := ctx.Player.Room
	if previous == destination {
		game.EnterRoom(ctx.World, ctx.Player, "")
		return false
	}
	if err := ctx.World.MoveToRoom(ctx.Player, destination); err != nil {
		ctx.Player.Output <- game.Ansi(game.Style("\r\n"+err.Error(), game.AnsiYellow))
		return false
	}
	vanish := fmt.Sprintf("\r\n%s vanishes in a shimmer of light.", game.HighlightName(ctx.Player.Name))
	ctx.World.BroadcastToRoom(previous, game.Ansi(vanish), ctx.Player)
	ctx.World.BroadcastToRoom(destination, game.Ansi(arrival), ctx.Player)
	game.EnterRoom(ctx.World, ctx.Player, "")
	return false
})
