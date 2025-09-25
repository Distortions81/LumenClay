package commands

import (
	"fmt"
	"strings"

	"LumenClay/internal/game"
)

const mailTimeLayout = "2006-01-02 15:04"

var Mail = Define(Definition{
	Name:        "mail",
	Usage:       "mail boards | mail board <name> | mail write <board> [recipients] = <message>",
	Description: "read and write public board posts",
}, func(ctx *Context) bool {
	mail := ctx.World.MailSystem()
	if mail == nil {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nThe public boards are currently unavailable.", game.AnsiYellow))
		return false
	}
	arg := strings.TrimSpace(ctx.Arg)
	if arg == "" || strings.EqualFold(arg, "help") {
		sendMailHelp(ctx.Player)
		return false
	}
	fields := strings.Fields(arg)
	if len(fields) == 0 {
		sendMailHelp(ctx.Player)
		return false
	}
	switch strings.ToLower(fields[0]) {
	case "boards":
		sendMailBoards(ctx.Player, mail, ctx.Player.Name)
	case "board":
		handleMailBoard(ctx, mail, fields)
	case "write":
		handleMailWrite(ctx, mail, arg, fields)
	default:
		// Treat the first token as a board name for convenience.
		handleMailBoard(ctx, mail, append([]string{"board"}, fields...))
	}
	return false
})

func sendMailHelp(player *game.Player) {
	var builder strings.Builder
	builder.WriteString("\r\nMail commands:\r\n")
	builder.WriteString("  mail boards - List boards and personal posts.\r\n")
	builder.WriteString("  mail board <name> - Show posts on a board.\r\n")
	builder.WriteString("  mail write <board> [recipients] = <message> - Post to a board; recipients are comma-separated player names.\r\n")
	player.Output <- game.Ansi(builder.String())
}

func sendMailBoards(player *game.Player, mail *game.MailSystem, self string) {
	boards := mail.Boards()
	if len(boards) == 0 {
		player.Output <- game.Ansi("\r\nNo boards have any posts yet.")
		return
	}
	var builder strings.Builder
	builder.WriteString("\r\nBoards:\r\n")
	for _, board := range boards {
		messages := mail.Messages(board)
		personal := 0
		for _, msg := range messages {
			if msg.AddressedTo(self) && len(msg.Recipients) > 0 {
				personal++
			}
		}
		line := fmt.Sprintf("  %-12s %3d posts", board, len(messages))
		if personal > 0 {
			line += fmt.Sprintf(" (%d for you)", personal)
		}
		builder.WriteString(line + "\r\n")
	}
	player.Output <- game.Ansi(builder.String())
}

func handleMailBoard(ctx *Context, mail *game.MailSystem, fields []string) {
	if len(fields) < 2 {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nWhich board?", game.AnsiYellow))
		return
	}
	board := fields[1]
	messages := mail.Messages(board)
	if len(messages) == 0 {
		ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\nThere are no posts on %s yet.", board))
		return
	}
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("\r\nBoard %s:\r\n", game.Style(strings.ToUpper(board), game.AnsiCyan, game.AnsiBold)))
	for _, msg := range messages {
		builder.WriteString(formatMailMessage(msg, ctx.Player.Name))
	}
	ctx.Player.Output <- game.Ansi(builder.String())
}

func formatMailMessage(msg game.MailMessage, viewer string) string {
	var builder strings.Builder
	marker := ""
	if len(msg.Recipients) > 0 && msg.AddressedTo(viewer) {
		marker = " " + game.Style("(for you)", game.AnsiGreen, game.AnsiBold)
	}
	builder.WriteString(fmt.Sprintf("  [%d] %s -> %s%s\r\n", msg.ID, game.HighlightName(msg.Author), msg.RecipientSummary(), marker))
	builder.WriteString(fmt.Sprintf("       %s\r\n", msg.CreatedAt.Format(mailTimeLayout)))
	for _, line := range strings.Split(msg.Body, "\n") {
		builder.WriteString("       " + line + "\r\n")
	}
	return builder.String()
}

func handleMailWrite(ctx *Context, mail *game.MailSystem, arg string, fields []string) {
	if len(fields) < 2 {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nWhich board should receive the post?", game.AnsiYellow))
		return
	}
	board := fields[1]
	rest := strings.TrimSpace(arg[len(fields[0]):])
	rest = strings.TrimSpace(rest[len(board):])
	if rest == "" {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nProvide recipients (optional) and a message separated by '='.", game.AnsiYellow))
		return
	}
	parts := strings.SplitN(rest, "=", 2)
	if len(parts) != 2 {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nUse '=' to separate recipients from the message body.", game.AnsiYellow))
		return
	}
	recipients := parseRecipients(parts[0])
	body := strings.TrimSpace(parts[1])
	if body == "" {
		ctx.Player.Output <- game.Ansi(game.Style("\r\nYour message is empty.", game.AnsiYellow))
		return
	}
	msg, err := mail.Write(board, ctx.Player.Name, recipients, body)
	if err != nil {
		ctx.Player.Output <- game.Ansi(game.Style("\r\n"+err.Error(), game.AnsiYellow))
		return
	}
	summary := msg.RecipientSummary()
	ctx.Player.Output <- game.Ansi(fmt.Sprintf("\r\nYou post to %s for %s.\r\n", game.Style(strings.ToUpper(board), game.AnsiCyan, game.AnsiBold), summary))
}

func parseRecipients(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	pieces := strings.Split(raw, ",")
	cleaned := make([]string, 0, len(pieces))
	seen := make(map[string]struct{}, len(pieces))
	for _, piece := range pieces {
		name := strings.TrimSpace(strings.TrimPrefix(piece, "@"))
		if name == "" {
			continue
		}
		lower := strings.ToLower(name)
		if _, exists := seen[lower]; exists {
			continue
		}
		seen[lower] = struct{}{}
		cleaned = append(cleaned, name)
	}
	if len(cleaned) == 0 {
		return nil
	}
	return cleaned
}
