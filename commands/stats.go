package commands

import (
	"fmt"
	"strings"
	"time"

	"LumenClay/internal/game"
)

var Stats = Define(Definition{
	Name:        "stats",
	Usage:       "stats",
	Description: "review your account details",
}, func(ctx *Context) bool {
	stats, ok := ctx.World.AccountStats(ctx.Player.Account)
	if !ok {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nAccount details are unavailable.", game.AnsiYellow))
		return false
	}

	ctx.Player.EnsureStats()
	var builder strings.Builder
	builder.WriteString(game.Style("\r\nAccount overview\r\n", game.AnsiBold, game.AnsiUnderline))
	builder.WriteString(fmt.Sprintf("  Name: %s\r\n", game.HighlightName(ctx.Player.Name)))
	builder.WriteString(fmt.Sprintf("  Account: %s\r\n", game.Style(ctx.Player.Account, game.AnsiCyan)))
	builder.WriteString(fmt.Sprintf("  Roles: %s\r\n", formatRoles(ctx.Player)))
	builder.WriteString(fmt.Sprintf("  Home: %s\r\n", describeRoom(ctx.World, ctx.Player.Home)))
	builder.WriteString(fmt.Sprintf("  Location: %s\r\n", describeRoom(ctx.World, ctx.Player.Room)))
	builder.WriteString(fmt.Sprintf("  Level: %s\r\n", game.Style(fmt.Sprintf("%d", ctx.Player.Level), game.AnsiGreen, game.AnsiBold)))
	builder.WriteString(fmt.Sprintf("  Experience: %s\r\n", game.Style(fmt.Sprintf("%d", ctx.Player.Experience), game.AnsiBlue)))
	builder.WriteString(fmt.Sprintf("  Health: %s\r\n", game.Style(fmt.Sprintf("%d/%d", ctx.Player.Health, ctx.Player.MaxHealth), game.AnsiGreen)))
	builder.WriteString(fmt.Sprintf("  Mana: %s\r\n", game.Style(fmt.Sprintf("%d/%d", ctx.Player.Mana, ctx.Player.MaxMana), game.AnsiMagenta)))

	now := time.Now().UTC()
	builder.WriteString(fmt.Sprintf("  Created: %s\r\n", formatTimestamp(stats.CreatedAt, now)))
	builder.WriteString(fmt.Sprintf("  Last login: %s\r\n", formatTimestamp(stats.LastLogin, now)))
	builder.WriteString(fmt.Sprintf("  Total logins: %s\r\n", game.Style(fmt.Sprintf("%d", stats.TotalLogins), game.AnsiGreen, game.AnsiBold)))
	builder.WriteString(fmt.Sprintf("  Channels: %s\r\n", formatChannelStatuses(ctx.World, ctx.Player)))

	ctx.Player.Output <- game.Ansi(builder.String())
	return false
})

func describeRoom(world *game.World, id game.RoomID) string {
	if id == "" {
		return game.Style("unknown", game.AnsiYellow)
	}
	if room, ok := world.GetRoom(id); ok {
		title := strings.TrimSpace(room.Title)
		if title == "" {
			title = string(id)
		}
		styledTitle := game.Style(title, game.AnsiBold, game.AnsiCyan)
		return fmt.Sprintf("%s %s", styledTitle, game.Style(fmt.Sprintf("(%s)", id), game.AnsiDim))
	}
	return game.Style(string(id), game.AnsiYellow)
}

func formatTimestamp(ts, now time.Time) string {
	if ts.IsZero() {
		return game.Style("never", game.AnsiYellow)
	}
	ts = ts.UTC()
	absolute := game.Style(ts.Format("2006-01-02 15:04 MST"), game.AnsiGreen)
	relative := describeRelative(ts, now)
	return fmt.Sprintf("%s %s", absolute, game.Style(fmt.Sprintf("(%s)", relative), game.AnsiDim))
}

func describeRelative(when, now time.Time) string {
	if when.IsZero() {
		return "never"
	}
	if now.Before(when) {
		now = when
	}
	diff := now.Sub(when)
	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		mins := int(diff / time.Minute)
		if mins <= 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	case diff < 24*time.Hour:
		hours := int(diff / time.Hour)
		if hours <= 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case diff < 30*24*time.Hour:
		days := int(diff / (24 * time.Hour))
		if days <= 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	case diff < 365*24*time.Hour:
		months := int(diff / (30 * 24 * time.Hour))
		if months <= 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	default:
		years := int(diff / (365 * 24 * time.Hour))
		if years <= 1 {
			return "1 year ago"
		}
		return fmt.Sprintf("%d years ago", years)
	}
}

func formatChannelStatuses(world *game.World, player *game.Player) string {
	statuses := world.ChannelStatuses(player)
	enabled := make([]string, 0, len(statuses))
	disabled := make([]string, 0)
	for _, channel := range game.AllChannels() {
		label := strings.ToUpper(string(channel))
		if statuses[channel] {
			enabled = append(enabled, label)
		} else {
			disabled = append(disabled, label)
		}
	}
	var parts []string
	if len(enabled) > 0 {
		parts = append(parts, game.Style(strings.Join(enabled, ", "), game.AnsiGreen, game.AnsiBold))
	} else {
		parts = append(parts, game.Style("none", game.AnsiYellow))
	}
	if len(disabled) > 0 {
		parts = append(parts, game.Style("off: "+strings.Join(disabled, ", "), game.AnsiDim))
	}
	return strings.Join(parts, " ")
}

func formatRoles(player *game.Player) string {
	roles := []string{"Player"}
	if player.IsBuilder {
		roles = append(roles, "Builder")
	}
	if player.IsAdmin {
		roles = append(roles, "Admin")
	}
	styled := make([]string, len(roles))
	for i, role := range roles {
		switch role {
		case "Admin":
			styled[i] = game.Style(role, game.AnsiBold, game.AnsiMagenta)
		case "Builder":
			styled[i] = game.Style(role, game.AnsiCyan)
		default:
			styled[i] = game.Style(role, game.AnsiGreen)
		}
	}
	return strings.Join(styled, game.Style(", ", game.AnsiDim))
}
