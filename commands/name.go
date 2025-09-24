package commands

import (
	"fmt"
	"strings"

	"LumenClay/internal/game"
)

var Name = Define(Definition{
	Name:        "name",
	Usage:       "name <newname>",
	Description: "change your display name",
}, func(ctx *Context) bool {
	newName := strings.TrimSpace(ctx.Arg)
	if newName == "" {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nUsage: name <newname>", game.AnsiYellow))
		return false
	}
	if strings.ContainsAny(newName, " \t\r\n") || len(newName) > 24 {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nInvalid name.", game.AnsiYellow))
		return false
	}
	old := ctx.Player.Name
	if err := ctx.World.RenamePlayer(ctx.Player, newName); err != nil {
		ctx.Player.Output <- game.Ansi(game.Style("\r\n"+err.Error(), game.AnsiYellow))
		return false
	}
	ctx.World.BroadcastToRoom(ctx.Player.Room, game.Ansi(fmt.Sprintf("\r\n%s is now known as %s.", game.HighlightName(old), game.HighlightName(newName))), ctx.Player)
	ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\nYou are now known as %s.", game.HighlightName(newName)))
	return false
})
