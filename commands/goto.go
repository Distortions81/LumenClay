package commands

import (
	"fmt"
	"strings"

	"aiMud/internal/game"
)

var Goto = Define(Definition{
	Name:        "goto",
	Usage:       "goto <room>",
	Description: "teleport to a room (builders/admins only)",
	Group:       GroupBuilder,
}, func(ctx *Context) bool {
	if !ctx.Player.IsAdmin && !ctx.Player.IsBuilder {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nOnly builders or admins may use goto.", game.AnsiYellow))
		return false
	}
	target := strings.TrimSpace(ctx.Arg)
	if target == "" {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nUsage: goto <room>", game.AnsiYellow))
		return false
	}
	roomID := game.RoomID(target)
	if _, ok := ctx.World.GetRoom(roomID); !ok {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nNo such room.", game.AnsiYellow))
		return false
	}
	prev := ctx.Player.Room
	if prev == roomID {
		game.EnterRoom(ctx.World, ctx.Player, "")
		return false
	}
	if err := ctx.World.MoveToRoom(ctx.Player, roomID); err != nil {
		ctx.Player.Output <- game.Ansi(game.Style("\r\n"+err.Error(), game.AnsiYellow))
		return false
	}
	ctx.World.BroadcastToRoom(prev, game.Ansi(fmt.Sprintf("\r\n%s vanishes in a shimmer of light.", game.HighlightName(ctx.Player.Name))), ctx.Player)
	ctx.World.BroadcastToRoom(roomID, game.Ansi(fmt.Sprintf("\r\n%s appears in a shimmer of light.", game.HighlightName(ctx.Player.Name))), ctx.Player)
	game.EnterRoom(ctx.World, ctx.Player, "")
	return false
})
