package commands

import (
	"fmt"
	"strings"

	"LumenClay/internal/game"
)

var List = Define(Definition{
	Name:        "list",
	Usage:       "list",
	Description: "list revision history for the current room (builders/admins only)",
	Group:       GroupBuilder,
}, func(ctx *Context) bool {
	if !ctx.Player.IsAdmin && !ctx.Player.IsBuilder {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nOnly builders or admins may review revisions.", game.AnsiYellow))
		return false
	}
	revisions, err := ctx.World.RoomRevisions(ctx.Player.Room)
	if err != nil {
		ctx.Player.Output <- game.Ansi(game.Style("\r\n"+err.Error(), game.AnsiYellow))
		return false
	}
	if len(revisions) == 0 {
		ctx.Player.Output <- game.Ansi("\r\nNo revisions recorded for this room.")
		return false
	}
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("\r\nRoom revisions for %s:\r\n", ctx.Player.Room))
	for i := len(revisions) - 1; i >= 0; i-- {
		rev := revisions[i]
		editor := strings.TrimSpace(rev.Editor)
		if editor == "" {
			editor = "system"
		} else {
			editor = game.HighlightName(editor)
		}
		builder.WriteString(fmt.Sprintf("  #%d by %s — title: %q, desc: %d chars\r\n", rev.Number, editor, rev.Title, len(rev.Description)))
	}
	ctx.Player.Output <- game.Ansi(builder.String())
	return false
})
