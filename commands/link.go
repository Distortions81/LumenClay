package commands

import (
	"fmt"
	"strings"

	"LumenClay/internal/game"
)

var Link = Define(Definition{
	Name:        "link",
	Usage:       "link <direction> <room> [return-direction]",
	Description: "create exits between rooms (builders/admins only)",
	Group:       GroupBuilder,
}, func(ctx *Context) bool {
	if !ctx.Player.IsAdmin && !ctx.Player.IsBuilder {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nOnly builders or admins may use link.", game.AnsiYellow))
		return false
	}
	parts := strings.Fields(ctx.Arg)
	if len(parts) < 2 {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nUsage: link <direction> <room> [return-direction]", game.AnsiYellow))
		return false
	}
	dir := parts[0]
	target := game.RoomID(parts[1])
	reverse := ""
	if len(parts) >= 3 {
		reverse = parts[2]
	}
	if err := ctx.World.LinkRooms(ctx.Player.Room, dir, target, reverse); err != nil {
		ctx.Player.Output <- game.Ansi(game.Style("\r\n"+err.Error(), game.AnsiYellow))
		return false
	}
	if reverse != "" {
		ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\nLinked %s to %s and %s back to %s.", dir, target, reverse, ctx.Player.Room))
	} else {
		ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\nLinked %s to %s.", dir, target))
	}
	return false
})
