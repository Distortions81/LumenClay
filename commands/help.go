package commands

import (
	"fmt"
	"strings"

	"LumenClay/internal/game"
)

var Help = Define(Definition{
	Name:        "help",
	Aliases:     []string{"?"},
	Usage:       "help",
	Description: "show this message",
}, func(ctx *Context) bool {
	var builder strings.Builder
	builder.WriteString(game.Style("\r\nCommands:\r\n", game.AnsiBold, game.AnsiUnderline))
	for _, cmd := range All() {
		usage := cmd.Usage
		if strings.TrimSpace(usage) == "" {
			usage = cmd.Name
		}
		builder.WriteString(fmt.Sprintf("  %-18s - %s\r\n", usage, cmd.Description))
	}
	ctx.Player.Output <- game.Ansi(builder.String())
	return false
})
