package commands

import (
	"fmt"
	"strings"

	"LumenClay/internal/game"
)

var Quests = Define(Definition{
	Name:        "quests",
	Aliases:     []string{"quest"},
	Usage:       "quests [available|active|accept <id>|turnin <id>]",
	Description: "review active quests or interact with quest givers",
}, func(ctx *Context) bool {
	width, _ := ctx.Player.WindowSize()
	parts := strings.Fields(ctx.Arg)
	if len(parts) == 0 {
		return showActiveQuests(ctx, width)
	}

	sub := strings.ToLower(parts[0])
	switch sub {
	case "active", "log":
		return showActiveQuests(ctx, width)
	case "available":
		return showAvailableQuests(ctx, width)
	case "accept":
		if len(parts) < 2 {
			ctx.Player.Output <- game.Ansi(game.Style("\r\nUsage: quests accept <id>", game.AnsiYellow))
			return false
		}
		questID := strings.ToLower(parts[1])
		quest, err := ctx.World.AcceptQuest(ctx.Player, questID)
		if err != nil {
			ctx.Player.Output <- game.Ansi(game.Style("\r\n"+err.Error(), game.AnsiYellow))
			return false
		}
		ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\nYou accept %s.", game.HighlightQuestName(quest.Name)))
		if desc := strings.TrimSpace(quest.Description); desc != "" {
			ctx.Player.Output <- game.Ansi("\r\n" + game.WrapText(desc, width))
		}
		return false
	case "turnin", "complete":
		if len(parts) < 2 {
			ctx.Player.Output <- game.Ansi(game.Style("\r\nUsage: quests turnin <id>", game.AnsiYellow))
			return false
		}
		questID := strings.ToLower(parts[1])
		result, err := ctx.World.CompleteQuest(ctx.Player, questID)
		if err != nil {
			ctx.Player.Output <- game.Ansi(game.Style("\r\n"+err.Error(), game.AnsiYellow))
			return false
		}
		ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\nYou complete %s!", game.HighlightQuestName(result.Quest.Name)))
		if strings.TrimSpace(result.CompletionMsg) != "" {
			ctx.Player.Output <- game.Ansi("\r\n" + game.WrapText(result.CompletionMsg, width))
		}
		if result.RewardXP > 0 {
			ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\nYou gain %d experience.", result.RewardXP))
			if result.LevelsGained > 0 {
				ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\nYou advance to level %d!", ctx.Player.Level))
			}
		}
		if len(result.RewardItems) > 0 {
			names := make([]string, len(result.RewardItems))
			for i, item := range result.RewardItems {
				names[i] = game.HighlightItemName(item.Name)
			}
			ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\nRewards: %s", strings.Join(names, ", ")))
		}
		return false
	default:
		ctx.Player.Output <- game.Ansi(game.Style("\r\nUnrecognised quests subcommand.", game.AnsiYellow))
		return false
	}
})

func showActiveQuests(ctx *Context, width int) bool {
	snapshots := ctx.World.SnapshotQuestLog(ctx.Player)
	if len(snapshots) == 0 {
		ctx.Player.Output <- game.Ansi("\r\nYou have no active quests.")
		return false
	}
	inventory := ctx.World.PlayerInventory(ctx.Player)
	itemCounts := make(map[string]int)
	for _, item := range inventory {
		itemCounts[strings.ToLower(item.Name)]++
	}
	for _, snap := range snapshots {
		status := "in progress"
		if snap.Completed {
			status = "completed"
		}
		header := fmt.Sprintf("\r\n%s (%s)", game.HighlightQuestName(snap.Quest.Name), status)
		ctx.Player.Output <- game.Ansi(header)
		if desc := strings.TrimSpace(snap.Quest.Description); desc != "" {
			ctx.Player.Output <- game.Ansi("\r\n  " + game.WrapText(desc, width))
		}
		for _, prog := range snap.KillProgress {
			ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\n  - Defeat %s (%d/%d)",
				game.HighlightNPCName(prog.NPC),
				prog.Current,
				prog.Required,
			))
		}
		for _, req := range snap.Quest.RequiredItems {
			have := itemCounts[strings.ToLower(req.Item)]
			ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\n  - Deliver %s (%d/%d)",
				game.HighlightItemName(req.Item),
				have,
				req.Count,
			))
		}
	}
	return false
}

func showAvailableQuests(ctx *Context, width int) bool {
	quests := ctx.World.AvailableQuests(ctx.Player)
	if len(quests) == 0 {
		ctx.Player.Output <- game.Ansi("\r\nNo quests are available here.")
		return false
	}
	for _, quest := range quests {
		ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\n[%s] %s", strings.ToLower(quest.ID), game.HighlightQuestName(quest.Name)))
		if desc := strings.TrimSpace(quest.Description); desc != "" {
			ctx.Player.Output <- game.Ansi("\r\n  " + game.WrapText(desc, width))
		}
		if len(quest.RequiredKills) > 0 {
			for _, req := range quest.RequiredKills {
				ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\n  - Defeat %s (%d)",
					game.HighlightNPCName(req.NPC),
					req.Count,
				))
			}
		}
		if len(quest.RequiredItems) > 0 {
			for _, req := range quest.RequiredItems {
				ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\n  - Deliver %s (%d)",
					game.HighlightItemName(req.Item),
					req.Count,
				))
			}
		}
		ctx.Player.Output <- game.Ansi("\r\n")
	}
	return false
}
