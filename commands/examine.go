package commands

import (
	"fmt"
	"strings"

	"LumenClay/internal/game"
)

var Examine = Define(Definition{
	Name:        "examine",
	Aliases:     []string{"exa"},
	Usage:       "examine <item>",
	Description: "inspect an item you are carrying",
}, func(ctx *Context) bool {
	target := strings.TrimSpace(ctx.Arg)
	if target == "" {
		ctx.Player.Output <- game.Ansi("\r\nExamine what?")
		return false
	}
	item, ok := ctx.World.FindInventoryItem(ctx.Player, target)
	if !ok {
		ctx.Player.Output <- game.Ansi("\r\nYou aren't carrying that.")
		return false
	}
	desc := strings.TrimSpace(item.Description)
	if desc == "" {
		desc = "You see nothing special."
	}
	ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\nYou examine %s. %s", game.HighlightItemName(item.Name), desc))
	ctx.World.TriggerItemInspect(ctx.Player, ctx.Player.Room, item, "inventory")
	return false
})
