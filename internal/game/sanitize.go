package game

import (
	"strings"
	"unicode"
)

func sanitizeInput(s string) string {
	if s == "" {
		return ""
	}
	var builder strings.Builder
	builder.Grow(len(s))
	changed := false
	for _, r := range s {
		sanitized, ok := sanitizeRune(r)
		if !ok {
			if !changed {
				changed = true
			}
			continue
		}
		if sanitized != r {
			if !changed {
				changed = true
			}
		}
		builder.WriteRune(sanitized)
	}
	if !changed {
		return s
	}
	return builder.String()
}

func sanitizeRune(r rune) (rune, bool) {
	switch {
	case r == '\r':
		return 0, false
	case unicode.IsSpace(r):
		if r == ' ' {
			return r, true
		}
		return ' ', true
	case r < 0x20 || r == 0x7f:
		return 0, false
	case unicode.Is(unicode.Cf, r):
		return 0, false
	case unicode.IsControl(r):
		return 0, false
	case unicode.In(r, unicode.Zl, unicode.Zp):
		return 0, false
	case !unicode.IsPrint(r):
		return 0, false
	default:
		return r, true
	}
}
