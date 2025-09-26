package game

import (
	"fmt"
	"strings"
)

const (
	AnsiReset     = "\x1b[0m"
	AnsiBold      = "\x1b[1m"
	AnsiDim       = "\x1b[2m"
	AnsiItalic    = "\x1b[3m"
	AnsiUnderline = "\x1b[4m"
	AnsiCyan      = "\x1b[36m"
	AnsiYellow    = "\x1b[33m"
	AnsiGreen     = "\x1b[32m"
	AnsiMagenta   = "\x1b[35m"
	AnsiBlue      = "\x1b[34m"
)

// Style wraps text with the provided ANSI attributes.
func Style(text string, attrs ...string) string {
	if len(attrs) == 0 {
		return text
	}
	return strings.Join(attrs, "") + text + AnsiReset
}

// HighlightName formats player names consistently.
func HighlightName(name string) string {
	return Style(name, AnsiBold, AnsiCyan)
}

// HighlightNames formats each name in the slice.
func HighlightNames(list []string) []string {
	out := make([]string, len(list))
	for i, name := range list {
		out[i] = HighlightName(name)
	}
	return out
}

// HighlightNPCName formats NPC names consistently.
func HighlightNPCName(name string) string {
	return Style(name, AnsiBold, AnsiMagenta)
}

// HighlightItemName formats item names consistently.
func HighlightItemName(name string) string {
	return Style(name, AnsiBold, AnsiYellow)
}

// Trim normalises a telnet input line.
func Trim(s string) string {
	cleaned := sanitizeInput(s)
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" {
		return ""
	}
	// Normalise any sequences of whitespace introduced during sanitisation to single spaces
	// to avoid leaking unexpected spacing into command handling.
	fields := strings.Fields(cleaned)
	if len(fields) == 0 {
		return ""
	}
	return strings.Join(fields, " ")
}

// Ansi ensures output strings end with a reset sequence.
func Ansi(c string) string {
	if strings.Contains(c, "\x1b[") && !strings.HasSuffix(c, AnsiReset) {
		return c + AnsiReset
	}
	return c
}

// Prompt renders the standard player prompt.
func Prompt(p *Player) string {
	if p != nil {
		p.EnsureStats()
	}
	if p == nil {
		return Ansi(Style("\r\n> ", AnsiBold, AnsiYellow))
	}
	summary := fmt.Sprintf("\r\n[L%02d HP %d/%d MP %d/%d] > ", p.Level, p.Health, p.MaxHealth, p.Mana, p.MaxMana)
	return Ansi(Style(summary, AnsiBold, AnsiYellow))
}
