package commands

import (
	"fmt"
	"strings"

	"LumenClay/internal/game"
)

var Attack = Define(Definition{
	Name:        "attack",
	Usage:       "attack <target>",
	Description: "strike a nearby foe with a melee attack",
}, func(ctx *Context) bool {
	target := strings.TrimSpace(ctx.Arg)
	if target == "" {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nUsage: attack <target>", game.AnsiYellow))
		return false
	}

	ctx.Player.EnsureStats()
	damage := ctx.Player.AttackDamage()

	if result, err := ctx.World.ApplyDamageToNPC(ctx.Player.Room, target, damage); err == nil {
		npcName := game.HighlightNPCName(result.NPC.Name)
		ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\nYou strike %s for %d damage. (%d/%d HP)", npcName, result.Damage, result.NPC.Health, result.NPC.MaxHealth))
		broadcast := fmt.Sprintf("\r\n%s strikes %s for %d damage.", game.HighlightName(ctx.Player.Name), npcName, result.Damage)
		ctx.World.BroadcastToRoom(ctx.Player.Room, game.Ansi(broadcast), ctx.Player)
		if result.Defeated {
			ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\nYou defeat %s!", npcName))
			ctx.World.BroadcastToRoom(ctx.Player.Room, game.Ansi(fmt.Sprintf("\r\n%s defeats %s!", game.HighlightName(ctx.Player.Name), npcName)), ctx.Player)
			xp := result.NPC.Experience
			if xp < 1 {
				xp = result.NPC.Level * 25
			}
			levels := ctx.World.AwardExperience(ctx.Player, xp)
			ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\nYou gain %d experience.", xp))
			if levels > 0 {
				ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\nYou advance to level %d!", ctx.Player.Level))
			}
			if len(result.Loot) > 0 {
				names := make([]string, len(result.Loot))
				for i, item := range result.Loot {
					names[i] = game.HighlightItemName(item.Name)
				}
				lootLine := fmt.Sprintf("\r\n%s drops %s.", npcName, strings.Join(names, ", "))
				ctx.Player.Output <- game.Ansi(lootLine)
				ctx.World.BroadcastToRoom(ctx.Player.Room, game.Ansi(fmt.Sprintf("\r\n%s leaves behind %s.", npcName, strings.Join(names, ", "))), ctx.Player)
			}
			if updates := ctx.World.RecordNPCKill(ctx.Player, result.NPC); len(updates) > 0 {
				for _, update := range updates {
					for _, prog := range update.KillProgress {
						ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\n[Quest] %s: %s (%d/%d)",
							game.HighlightQuestName(update.Quest.Name),
							game.HighlightNPCName(prog.NPC),
							prog.Current,
							prog.Required,
						))
					}
					if update.KillsCompleted {
						turnIn := update.Quest.TurnIn
						if strings.TrimSpace(turnIn) == "" {
							turnIn = update.Quest.Giver
						}
						if turnIn != "" {
							ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\n[Quest] %s objectives complete. Visit %s to turn in.",
								game.HighlightQuestName(update.Quest.Name),
								game.HighlightNPCName(turnIn),
							))
						} else {
							ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\n[Quest] %s objectives complete.",
								game.HighlightQuestName(update.Quest.Name)))
						}
					}
				}
			}
		}
		ctx.Player.Output <- game.Prompt(ctx.Player)
		return false
	}

	result, err := ctx.World.ApplyDamageToPlayer(ctx.Player, target, damage)
	if err != nil {
		ctx.Player.Output <- game.Ansi(game.Style(fmt.Sprintf("\r\n%s", err.Error()), game.AnsiYellow))
		ctx.Player.Output <- game.Prompt(ctx.Player)
		return false
	}

	targetName := game.HighlightName(result.Target.Name)
	broadcast := fmt.Sprintf("\r\n%s strikes %s for %d damage.", game.HighlightName(ctx.Player.Name), targetName, result.Damage)
	ctx.World.BroadcastToRoom(result.PreviousRoom, game.Ansi(broadcast), ctx.Player)

	if result.Defeated {
		ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\nYou defeat %s!", targetName))
		ctx.World.BroadcastToRoom(result.PreviousRoom, game.Ansi(fmt.Sprintf("\r\n%s collapses in defeat!", targetName)), ctx.Player)
		if result.Target.Output != nil {
			result.Target.Output <- game.Ansi(fmt.Sprintf("\r\nYou have been defeated by %s!", game.HighlightName(ctx.Player.Name)))
			game.EnterRoom(ctx.World, result.Target, "defeat")
		}
	} else {
		ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\nYou strike %s for %d damage. (%d/%d HP)", targetName, result.Damage, result.Remaining, result.Target.MaxHealth))
		if result.Target.Output != nil {
			result.Target.Output <- game.Ansi(fmt.Sprintf("\r\n%s strikes you for %d damage. (%d/%d HP)", game.HighlightName(ctx.Player.Name), result.Damage, result.Remaining, result.Target.MaxHealth))
			result.Target.Output <- game.Prompt(result.Target)
		}
	}

	ctx.Player.Output <- game.Prompt(ctx.Player)
	return false
})
