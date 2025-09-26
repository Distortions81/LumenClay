package commands

import (
	"fmt"
	"strconv"
	"strings"

	"LumenClay/internal/game"
)

var Revnum = Define(Definition{
	Name:        "revnum",
	Usage:       "revnum <number>",
	Description: "revert the current room to a previous revision (builders/admins only)",
	Group:       GroupBuilder,
}, func(ctx *Context) bool {
	if !ctx.Player.IsAdmin && !ctx.Player.IsBuilder {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nOnly builders or admins may revert rooms.", game.AnsiYellow))
		return false
	}
	arg := strings.TrimSpace(ctx.Arg)
	if arg == "" {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nUsage: revnum <number>", game.AnsiYellow))
		return false
	}
	number, err := strconv.Atoi(arg)
	if err != nil || number <= 0 {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nRevision numbers must be positive integers.", game.AnsiYellow))
		return false
	}
	revisions, err := ctx.World.RoomRevisions(ctx.Player.Room)
	if err != nil {
		ctx.Player.Output <- game.Ansi(game.Style("\r\n"+err.Error(), game.AnsiYellow))
		return false
	}
	var latest *game.RoomRevision
	var target *game.RoomRevision
	for i := range revisions {
		rev := &revisions[i]
		if latest == nil || rev.Number > latest.Number {
			latest = rev
		}
		if rev.Number == number {
			target = rev
		}
	}
	if target == nil {
		ctx.Player.Output <- game.Ansi(game.Style(fmt.Sprintf("\r\nUnknown revision: %d", number), game.AnsiYellow))
		return false
	}
	if latest != nil && latest.Number == target.Number {
		if strings.TrimSpace(latest.Title) == strings.TrimSpace(target.Title) && strings.TrimSpace(latest.Description) == strings.TrimSpace(target.Description) {
			ctx.Player.Output <- game.Ansi(game.Style("\r\nThe room already matches that revision.", game.AnsiYellow))
			return false
		}
	}
	if _, err := ctx.World.RevertRoomToRevision(ctx.Player.Room, number, ctx.Player.Name); err != nil {
		ctx.Player.Output <- game.Ansi(game.Style("\r\n"+err.Error(), game.AnsiYellow))
		return false
	}
	ctx.World.BroadcastToRoom(ctx.Player.Room, game.Ansi(fmt.Sprintf("\r\n%s restores the room to revision #%d.", game.HighlightName(ctx.Player.Name), number)), ctx.Player)
	ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\nRoom reverted to revision #%d.", number))
	return false
})
