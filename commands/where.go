package commands

import (
	"fmt"
	"strings"

	"aiMud/internal/game"
)

var Where = Define(Definition{
	Name:        "where",
	Usage:       "where",
	Description: "show player locations (builders/admins only)",
	Group:       GroupBuilder,
}, func(ctx *Context) bool {
	if !ctx.Player.IsAdmin && !ctx.Player.IsBuilder {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nOnly builders or admins may use where.", game.AnsiYellow))
		return false
	}
	locations := ctx.World.PlayerLocations()
	if len(locations) == 0 {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nNo players are currently connected.", game.AnsiYellow))
		return false
	}
	var builder strings.Builder
	builder.WriteString(game.Style("\r\nPlayer locations:\r\n", game.AnsiBold, game.AnsiUnderline))
	for _, loc := range locations {
		roomName := "Unknown"
		if room, ok := ctx.World.GetRoom(loc.Room); ok {
			roomName = room.Title
		}
		builder.WriteString(fmt.Sprintf("  %-18s - %s [%s]\r\n", game.HighlightName(loc.Name), roomName, loc.Room))
	}
	ctx.Player.Output <- game.Ansi(builder.String())
	return false
})
