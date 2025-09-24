package commands

import (
	"errors"
	"fmt"
	"strings"

	"LumenClay/internal/game"
)

var Get = Define(Definition{
	Name:        "get",
	Aliases:     []string{"take", "pickup"},
	Usage:       "get <item>",
	Description: "pick up an item in the room",
}, func(ctx *Context) bool {
	target := strings.TrimSpace(ctx.Arg)
	if target == "" {
		ctx.Player.Output <- game.Ansi("\r\nGet what?")
		return false
	}
	item, err := ctx.World.TakeItem(ctx.Player, target)
	switch {
	case err == nil:
		ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\nYou pick up %s.", game.HighlightItemName(item.Name)))
		ctx.World.BroadcastToRoom(ctx.Player.Room, game.Ansi(fmt.Sprintf("\r\n%s picks up %s.", game.HighlightName(ctx.Player.Name), game.HighlightItemName(item.Name))), ctx.Player)
	case errors.Is(err, game.ErrItemNotFound):
		ctx.Player.Output <- game.Ansi("\r\nYou don't see that here.")
	default:
		ctx.Player.Output <- game.Ansi("\r\n" + err.Error())
	}
	return false
})
