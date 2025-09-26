package commands

import (
	"fmt"
	"strings"

	"LumenClay/internal/game"
)

var dreamCommand = Define(Definition{
	Name:        "dream",
	Usage:       "dream",
	Description: "slip into a personalised dreamscape",
	Group:       GroupGeneral,
}, func(ctx *Context) bool {
	ctx.Player.Output <- game.Ansi(renderDreamscape(ctx.Player.Name))
	return false
})

var dreamScenes = []func(name, highlight string) string{
	func(name, highlight string) string {
		return fmt.Sprintf(
			"%s wanders the %s, where every book is bound in comet tails and hums with %s.\r\n"+
				"A librarian woven from auroras presses a volume into your hands, its pages\r\n"+
				"promising %s a new chapter you have not yet dared to write.",
			highlight,
			game.Style("astral archive", game.AnsiBold, game.AnsiMagenta),
			game.Style("possibility", game.AnsiGreen),
			game.Style(name, game.AnsiBold, game.AnsiYellow),
		)
	},
	func(name, highlight string) string {
		return fmt.Sprintf(
			"%s sails a tide of %s that spills across the midnight dunes.\r\n"+
				"Constellations rearrange themselves to sketch the choices you treasure most,\r\n"+
				"while distant waves whisper a rhythm that matches your heartbeat, %s.",
			highlight,
			game.Style("liquid starlight", game.AnsiBold, game.AnsiCyan),
			game.Style(name, game.AnsiBold, game.AnsiBlue),
		)
	},
	func(name, highlight string) string {
		return fmt.Sprintf(
			"%s tends a garden where memories bloom as %s.\r\n"+
				"Each petal glows warmer as %s breathes courage into the stories still to come,\r\n"+
				"and fireflies gather to map a path only you can follow.",
			highlight,
			game.Style("luminescent flowers", game.AnsiYellow, game.AnsiBold),
			game.Style(name, game.AnsiBold, game.AnsiMagenta),
		)
	},
	func(name, highlight string) string {
		melody := game.Style("celestial melody", game.AnsiBold, game.AnsiBlue)
		return fmt.Sprintf(
			"%s balances upon a bridge of moonbeams suspended above a sleeping city.\r\n"+
				"Beneath you, windows kindle awake as you hum a %s,\r\n"+
				"inviting those below to chase the dream alongside %s.",
			highlight,
			melody,
			game.Style(name, game.AnsiBold, game.AnsiGreen),
		)
	},
}

func renderDreamscape(name string) string {
	highlight := game.HighlightName(name)
	idx := dreamIndex(name)
	scene := dreamScenes[idx](name, highlight)

	var builder strings.Builder
	builder.WriteString("\r\n")
	builder.WriteString(game.Style("You close your eyes and drift into a dreamscape...\r\n\r\n", game.AnsiItalic))
	builder.WriteString(scene)
	builder.WriteString("\r\n\r\n")
	builder.WriteString(game.Style("The vision lingers like stardust on your fingertips.", game.AnsiDim))
	return builder.String()
}

func dreamIndex(name string) int {
	if len(dreamScenes) == 0 {
		return 0
	}
	sum := 0
	for _, r := range strings.ToLower(name) {
		sum += int(r)
	}
	if sum < 0 {
		sum = -sum
	}
	return sum % len(dreamScenes)
}
