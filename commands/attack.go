package commands

import (
	"fmt"
	"strings"

	"LumenClay/internal/game"
)

var Attack = Define(Definition{
	Name:        "attack",
	Usage:       "attack <target>",
	Description: "engage a nearby foe in combat",
}, func(ctx *Context) bool {
	target := strings.TrimSpace(ctx.Arg)
	if target == "" {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nUsage: attack <target>", game.AnsiYellow))
		return false
	}

	if err := ctx.World.StartCombat(ctx.Player, target); err != nil {
		ctx.Player.Output <- game.Ansi(game.Style(fmt.Sprintf("\r\n%s", err.Error()), game.AnsiYellow))
		ctx.Player.Output <- game.Prompt(ctx.Player)
		return false
	}

	ctx.Player.Output <- game.Prompt(ctx.Player)
	return false
})
