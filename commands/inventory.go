package commands

import (
	"fmt"
	"strings"

	"LumenClay/internal/game"
)

var Inventory = Define(Definition{
	Name:        "inventory",
	Aliases:     []string{"inv", "i"},
	Usage:       "inventory",
	Description: "list items you are carrying",
}, func(ctx *Context) bool {
	items := ctx.World.PlayerInventory(ctx.Player)
	if len(items) == 0 {
		ctx.Player.Output <- game.Ansi("\r\nYou aren't carrying anything.")
		return false
	}
	names := make([]string, len(items))
	for i, item := range items {
		names[i] = game.HighlightItemName(item.Name)
	}
	ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\nYou are carrying: %s", strings.Join(names, ", ")))
	return false
})
