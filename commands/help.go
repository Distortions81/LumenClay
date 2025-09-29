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
	general := generalHelpCommands()
	message := helpMessage("Commands:", general)
	if ctx.Player.IsBuilder || ctx.Player.IsAdmin {
		message += "\r\nType 'buildhelp' for building commands."
	}
	if ctx.Player.IsAdmin {
		message += "\r\nType 'wizhelp' for admin commands."
	}
	if ctx.Player.IsModerator {
		message += "\r\nModerators may type 'portal' to request a moderation portal link."
	}
	ctx.Player.Output <- game.Ansi(message)
	return false
})

func helpMessage(title string, commands []*Command) string {
	var builder strings.Builder
	builder.WriteString(game.Style("\r\n"+title+"\r\n", game.AnsiBold, game.AnsiUnderline))
	for _, cmd := range commands {
		usage := cmd.Usage
		if strings.TrimSpace(usage) == "" {
			usage = cmd.Name
		}
		builder.WriteString(fmt.Sprintf("  %-18s - %s\r\n", usage, cmd.Description))
	}
	return builder.String()
}

func commandsForGroup(group CommandGroup) []*Command {
	all := All()
	filtered := make([]*Command, 0, len(all))
	for _, cmd := range all {
		if cmd.Group == group {
			filtered = append(filtered, cmd)
		}
	}
	return filtered
}

func generalHelpCommands() []*Command {
	all := All()
	filtered := make([]*Command, 0, len(all))
	for _, cmd := range all {
		if cmd.Group == GroupGeneral {
			filtered = append(filtered, cmd)
		}
	}
	return filtered
}
