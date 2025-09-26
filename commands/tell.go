package commands

import (
	"errors"
	"fmt"
	"strings"

	"LumenClay/internal/game"
)

var Tell = Define(Definition{
	Name:        "tell",
	Usage:       "tell <player> <message>",
	Description: "send a private message to a player, queueing it if they're offline",
}, func(ctx *Context) bool {
	fields := strings.Fields(ctx.Arg)
	if len(fields) < 2 {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nUsage: tell <player> <message>", game.AnsiYellow))
		return false
	}
	targetToken := fields[0]
	message := strings.TrimSpace(strings.TrimPrefix(ctx.Arg, targetToken))
	if message == "" {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nWhat do you want to say?", game.AnsiYellow))
		return false
	}

	if target, ok := ctx.World.FindPlayer(targetToken); ok {
		received := game.Ansi(fmt.Sprintf("\r\n%s tells you: %s", game.HighlightName(ctx.Player.Name), message))
		target.Output <- received
		ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\nYou tell %s: %s", game.HighlightName(target.Name), message))
		return false
	}

	tell, canonical, err := ctx.World.QueueOfflineTell(ctx.Player, targetToken, message)
	if err != nil {
		if errors.Is(err, game.ErrOfflineTellLimit) {
			ctx.Player.Output <- game.Ansi(game.Style(fmt.Sprintf("\r\nYou already have %d offline tells queued for %s.", game.OfflineTellLimitPerSender, game.HighlightName(canonical)), game.AnsiYellow))
			return false
		}
		ctx.Player.Output <- game.Ansi(game.Style("\r\n"+err.Error(), game.AnsiYellow))
		return false
	}
	ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\nYou queue an offline tell for %s: %s", game.HighlightName(tell.Recipient), tell.Body))
	return false
})
