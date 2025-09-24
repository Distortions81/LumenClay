package game

import "strings"

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

// Trim normalises a telnet input line.
func Trim(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, "\r", ""))
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
	return Ansi(Style("\r\n> ", AnsiBold, AnsiYellow))
}
