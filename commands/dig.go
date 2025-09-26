package commands

import (
	"fmt"
	"strings"

	"LumenClay/internal/game"
)

var Dig = Define(Definition{
	Name:        "dig",
	Usage:       "dig <id> [title]",
	Description: "create a new room (builders/admins only)",
	Group:       GroupBuilder,
}, func(ctx *Context) bool {
	if !ctx.Player.IsAdmin && !ctx.Player.IsBuilder {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nOnly builders or admins may use dig.", game.AnsiYellow))
		return false
	}
	args := strings.TrimSpace(ctx.Arg)
	if args == "" {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nUsage: dig <id> [title]", game.AnsiYellow))
		return false
	}
	parts := strings.Fields(args)
	id := parts[0]
	title := strings.TrimSpace(strings.TrimPrefix(args, id))
	room, err := ctx.World.CreateRoom(game.RoomID(id), title, ctx.Player.Name)
	if err != nil {
		ctx.Player.Output <- game.Ansi(game.Style("\r\n"+err.Error(), game.AnsiYellow))
		return false
	}
	ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\nCreated room %s (%s).", room.ID, room.Title))
	return false
})
