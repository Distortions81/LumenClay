package game

import "strings"

// WrapText inserts soft line breaks into the provided text so that each line
// fits within the supplied column width. Existing paragraph breaks are
// preserved and a minimum width is enforced to avoid over-wrapping when the
// client reports extremely small windows.
func WrapText(text string, width int) string {
	if width <= 0 {
		return text
	}
	if width < 20 {
		width = 20
	}
	lines := strings.Split(text, "\n")
	wrapped := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			wrapped = append(wrapped, "")
			continue
		}
		wrapped = append(wrapped, wrapLine(trimmed, width))
	}
	return strings.Join(wrapped, "\n")
}

func wrapLine(line string, width int) string {
	words := strings.Fields(line)
	if len(words) == 0 {
		return ""
	}
	var builder strings.Builder
	current := 0
	for _, word := range words {
		runes := []rune(word)
		for len(runes) > 0 {
			if len(runes) > width {
				if current != 0 {
					builder.WriteString("\n")
				}
				builder.WriteString(string(runes[:width]))
				runes = runes[width:]
				current = width
				if len(runes) > 0 {
					builder.WriteString("\n")
					current = 0
				}
				continue
			}
			wordLen := len(runes)
			if current == 0 {
				builder.WriteString(string(runes))
				current = wordLen
			} else if current+1+wordLen > width {
				builder.WriteString("\n")
				builder.WriteString(string(runes))
				current = wordLen
			} else {
				builder.WriteByte(' ')
				builder.WriteString(string(runes))
				current += 1 + wordLen
			}
			runes = runes[:0]
		}
	}
	return builder.String()
}
