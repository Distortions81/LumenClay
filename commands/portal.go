package commands

import (
	"fmt"
	"strings"
	"time"

	"LumenClay/internal/game"
)

var Portal = Define(Definition{
	Name:        "portal",
	Usage:       "portal [notes|builder|moderator|admin]",
	Description: "generate a secure one-use web portal link",
	Group:       GroupGeneral,
}, func(ctx *Context) bool {
	provider := ctx.World.Portal()
	if provider == nil {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nThe web portal is not configured. Ask an admin to enable TLS (default Certbot fullchain.pem/privkey.pem) or supply --web-addr with a port.", game.AnsiYellow))
		return false
	}

	requested := strings.ToLower(strings.TrimSpace(ctx.Arg))
	role, ok := selectPortalRole(ctx.Player, requested)
	if !ok {
		if requested != "" {
			ctx.Player.Output <- game.Ansi(game.Style("\r\nYou are not permitted to request that portal.", game.AnsiYellow))
		} else {
			ctx.Player.Output <- game.Ansi(game.Style("\r\nRequest a specific portal with notes, builder, moderator, or admin.", game.AnsiYellow))
		}
		return false
	}

	link, err := provider.GenerateLink(role, ctx.Player.Name)
	if err != nil {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nFailed to generate portal link: "+err.Error(), game.AnsiYellow))
		return false
	}

	ttl := time.Until(link.Expires)
	if ttl < 0 {
		ttl = 0
	}
	ttlText := formatPortalDuration(ttl)
	label := portalRoleLabel(role)
	hyperlink := game.Hyperlink(link.URL, "Open portal")
	message := fmt.Sprintf("\r\n%s portal link (expires in %s): %s\r\n  %s", label, ttlText, hyperlink, link.URL)
	ctx.Player.Output <- game.Ansi(message)
	ctx.Player.Output <- game.Ansi(game.Style("\r\nThe link may be used once. Request a new one if it expires.", game.AnsiYellow))
	return false
})

func selectPortalRole(player *game.Player, requested string) (game.PortalRole, bool) {
	switch requested {
	case "notes", "player", "note":
		return game.PortalRolePlayer, true
	case "builder":
		if player.IsBuilder || player.IsAdmin || player.IsModerator {
			return game.PortalRoleBuilder, true
		}
		return "", false
	case "moderator":
		if player.IsAdmin || player.IsModerator {
			return game.PortalRoleModerator, true
		}
		return "", false
	case "admin":
		if player.IsAdmin {
			return game.PortalRoleAdmin, true
		}
		return "", false
	case "":
		switch {
		case player.IsAdmin:
			return game.PortalRoleAdmin, true
		case player.IsModerator:
			return game.PortalRoleModerator, true
		case player.IsBuilder:
			return game.PortalRoleBuilder, true
		default:
			return game.PortalRolePlayer, true
		}
	default:
		return "", false
	}
}

func portalRoleLabel(role game.PortalRole) string {
	switch role {
	case game.PortalRoleAdmin:
		return "Administration"
	case game.PortalRoleModerator:
		return "Moderation"
	case game.PortalRolePlayer:
		return "Notes"
	default:
		return "Builder"
	}
}

func formatPortalDuration(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}
	d = d.Round(time.Second)
	hours := d / time.Hour
	d -= hours * time.Hour
	minutes := d / time.Minute
	d -= minutes * time.Minute
	seconds := d / time.Second
	parts := make([]string, 0, 3)
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if minutes > 0 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}
	if seconds > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%ds", seconds))
	}
	return strings.Join(parts, " ")
}
