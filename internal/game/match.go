package game

import "strings"

// uniqueMatch attempts to resolve the provided target string against a slice of
// candidate names. It performs a case-insensitive comparison, supports prefix
// matching, and optionally considers word-level prefixes. The function returns
// the index of the uniquely matched candidate and true. If no match or an
// ambiguous match is found, it returns -1 and false.
func uniqueMatch(target string, names []string, matchWords bool) (int, bool) {
	trimmed := strings.TrimSpace(target)
	if trimmed == "" {
		return -1, false
	}
	normalized := strings.ToLower(trimmed)

	partial := -1
	ambiguous := false
	for i, name := range names {
		candidate := strings.ToLower(strings.TrimSpace(name))
		if candidate == normalized {
			return i, true
		}

		match := strings.HasPrefix(candidate, normalized)
		if !match && matchWords {
			for _, word := range strings.Fields(candidate) {
				if strings.HasPrefix(word, normalized) {
					match = true
					break
				}
			}
		}

		if match {
			if partial != -1 {
				ambiguous = true
				continue
			}
			partial = i
		}
	}

	if partial != -1 && !ambiguous {
		return partial, true
	}
	return -1, false
}
