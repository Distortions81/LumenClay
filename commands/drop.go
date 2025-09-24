package commands

import (
	"errors"
	"fmt"
	"strings"

	"LumenClay/internal/game"
)

var Drop = Define(Definition{
	Name:        "drop",
	Usage:       "drop <item>",
	Description: "place a carried item in the room",
}, func(ctx *Context) bool {
	target := strings.TrimSpace(ctx.Arg)
	if target == "" {
		ctx.Player.Output <- game.Ansi("\r\nDrop what?")
		return false
	}
	item, err := ctx.World.DropItem(ctx.Player, target)
	switch {
	case err == nil:
		ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\nYou drop %s.", game.HighlightItemName(item.Name)))
		ctx.World.BroadcastToRoom(ctx.Player.Room, game.Ansi(fmt.Sprintf("\r\n%s drops %s.", game.HighlightName(ctx.Player.Name), game.HighlightItemName(item.Name))), ctx.Player)
	case errors.Is(err, game.ErrItemNotCarried):
		ctx.Player.Output <- game.Ansi("\r\nYou aren't carrying that.")
	default:
		ctx.Player.Output <- game.Ansi("\r\n" + err.Error())
	}
	return false
})
