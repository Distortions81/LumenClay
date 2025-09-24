package commands

import (
	"fmt"
	"strings"

	"aiMud/internal/game"
)

var Look = Define(Definition{
	Name:        "look",
	Aliases:     []string{"l"},
	Usage:       "look",
	Description: "describe your room",
}, func(ctx *Context) bool {
	room, ok := ctx.World.GetRoom(ctx.Player.Room)
	if !ok {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nYou see only void.", game.AnsiYellow))
		return false
	}
	title := game.Style(room.Title, game.AnsiBold, game.AnsiCyan)
	desc := game.Style(room.Description, game.AnsiItalic, game.AnsiDim)
	exits := game.Style(game.ExitList(room), game.AnsiGreen)
	ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\n%s\r\n%s\r\nExits: %s", title, desc, exits))
	others := ctx.World.ListPlayers(true, ctx.Player.Room)
	if len(others) > 1 {
		seen := game.FilterOut(others, ctx.Player.Name)
		colored := game.HighlightNames(seen)
		ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\nYou see: %s", strings.Join(colored, ", ")))
	}
	return false
})
