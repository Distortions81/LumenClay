package commands

import (
	"fmt"

	"aiMud/internal/game"
)

var Emote = Define(Definition{
	Name:        "emote",
	Aliases:     []string{":"},
	Usage:       "emote <action>",
	Description: "emote to the room",
}, func(ctx *Context) bool {
	action := ctx.Arg
	if action == "" {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nEmote what?", game.AnsiYellow))
		return false
	}
	ctx.World.BroadcastToRoom(ctx.Player.Room, game.Ansi(fmt.Sprintf("\r\n%s %s", game.HighlightName(ctx.Player.Name), action)), ctx.Player)
	ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\n%s %s", game.Style("You", game.AnsiBold, game.AnsiYellow), action))
	return false
})
