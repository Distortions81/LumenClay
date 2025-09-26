package commands

import (
	"fmt"
	"strings"

	"LumenClay/internal/game"
)

var Cast = Define(Definition{
	Name:        "cast",
	Usage:       "cast <spell> [target]",
	Description: "invoke a simple spell",
}, func(ctx *Context) bool {
	fields := strings.Fields(ctx.Arg)
	if len(fields) == 0 {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nUsage: cast <spell> [target]", game.AnsiYellow))
		return false
	}

	spell := strings.ToLower(fields[0])
	ctx.Player.EnsureStats()

	switch spell {
	case "heal":
		manaCost := 10
		if ctx.Player.Mana < manaCost {
			ctx.Player.Output <- game.Ansi(game.Style("\r\nYou lack the mana to cast heal.", game.AnsiYellow))
			return false
		}
		ctx.Player.Mana -= manaCost
		amount := 15 + ctx.Player.Level*5
		ctx.Player.Health += amount
		if ctx.Player.Health > ctx.Player.MaxHealth {
			ctx.Player.Health = ctx.Player.MaxHealth
		}
		ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\nYou channel restorative energy and recover %d health.", amount))
		ctx.World.BroadcastToRoom(ctx.Player.Room, game.Ansi(fmt.Sprintf("\r\n%s is bathed in soothing light.", game.HighlightName(ctx.Player.Name))), ctx.Player)
		ctx.Player.Output <- game.Prompt(ctx.Player)
		return false
	case "bolt":
		if len(fields) < 2 {
			ctx.Player.Output <- game.Ansi(game.Style("\r\nUsage: cast bolt <target>", game.AnsiYellow))
			return false
		}
		manaCost := 15
		if ctx.Player.Mana < manaCost {
			ctx.Player.Output <- game.Ansi(game.Style("\r\nYou lack the mana to cast bolt.", game.AnsiYellow))
			return false
		}
		target := strings.Join(fields[1:], " ")
		damage := 10 + ctx.Player.Level*3
		if result, err := ctx.World.ApplyDamageToNPC(ctx.Player.Room, target, damage); err == nil {
			ctx.Player.Mana -= manaCost
			npcName := game.HighlightNPCName(result.NPC.Name)
			ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\nArcs of energy slam into %s for %d damage. (%d/%d HP)", npcName, result.Damage, result.NPC.Health, result.NPC.MaxHealth))
			ctx.World.BroadcastToRoom(ctx.Player.Room, game.Ansi(fmt.Sprintf("\r\n%s hurls a crackling bolt at %s for %d damage!", game.HighlightName(ctx.Player.Name), npcName, result.Damage)), ctx.Player)
			if result.Defeated {
				ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\nYour magic fells %s!", npcName))
				xp := result.NPC.Experience
				if xp < 1 {
					xp = result.NPC.Level * 25
				}
				levels := ctx.World.AwardExperience(ctx.Player, xp)
				ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\nYou gain %d experience.", xp))
				if levels > 0 {
					ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\nYou advance to level %d!", ctx.Player.Level))
				}
			}
			ctx.Player.Output <- game.Prompt(ctx.Player)
			return false
		}
		if result, err := ctx.World.ApplyDamageToPlayer(ctx.Player, target, damage); err == nil {
			ctx.Player.Mana -= manaCost
			targetName := game.HighlightName(result.Target.Name)
			ctx.World.BroadcastToRoom(result.PreviousRoom, game.Ansi(fmt.Sprintf("\r\n%s unleashes a bolt at %s for %d damage!", game.HighlightName(ctx.Player.Name), targetName, result.Damage)), ctx.Player)
			if result.Defeated {
				ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\nYour bolt overwhelms %s!", targetName))
				ctx.World.BroadcastToRoom(result.PreviousRoom, game.Ansi(fmt.Sprintf("\r\n%s collapses under the magical assault!", targetName)), ctx.Player)
				if result.Target.Output != nil {
					result.Target.Output <- game.Ansi(fmt.Sprintf("\r\n%s' bolt overwhelms you!", game.HighlightName(ctx.Player.Name)))
					game.EnterRoom(ctx.World, result.Target, "defeat")
				}
			} else {
				ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\nYour bolt scorches %s for %d damage. (%d/%d HP)", targetName, result.Damage, result.Remaining, result.Target.MaxHealth))
				if result.Target.Output != nil {
					result.Target.Output <- game.Ansi(fmt.Sprintf("\r\n%s' bolt burns you for %d damage! (%d/%d HP)", game.HighlightName(ctx.Player.Name), result.Damage, result.Remaining, result.Target.MaxHealth))
					result.Target.Output <- game.Prompt(result.Target)
				}
			}
			ctx.Player.Output <- game.Prompt(ctx.Player)
			return false
		}
		ctx.Player.Output <- game.Ansi(game.Style("\r\nYour spell fails to find a target.", game.AnsiYellow))
		return false
	default:
		ctx.Player.Output <- game.Ansi(game.Style("\r\nYou do not know that spell.", game.AnsiYellow))
		return false
	}
})
