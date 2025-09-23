package main

import "strings"

const (
	ansiReset     = "\x1b[0m"
	ansiBold      = "\x1b[1m"
	ansiDim       = "\x1b[2m"
	ansiItalic    = "\x1b[3m"
	ansiUnderline = "\x1b[4m"
	ansiCyan      = "\x1b[36m"
	ansiYellow    = "\x1b[33m"
	ansiGreen     = "\x1b[32m"
	ansiMagenta   = "\x1b[35m"
)

func style(text string, attrs ...string) string {
	if len(attrs) == 0 {
		return text
	}
	return strings.Join(attrs, "") + text + ansiReset
}

func highlightName(name string) string {
	return style(name, ansiBold, ansiCyan)
}

func highlightNames(list []string) []string {
	out := make([]string, len(list))
	for i, name := range list {
		out[i] = highlightName(name)
	}
	return out
}

func trim(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, "\r", ""))
}

func ansi(c string) string {
	if strings.Contains(c, "\x1b[") && !strings.HasSuffix(c, ansiReset) {
		return c + ansiReset
	}
	return c
}

func prompt(p *Player) string {
	return ansi(style("\r\n> ", ansiBold, ansiYellow))
}
