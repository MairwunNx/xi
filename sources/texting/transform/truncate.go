package transform

import "unicode"

func SmartTruncate(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}

	truncated := text[:maxLen]

	for i := len(truncated) - 1; i >= 0; i-- {
		if unicode.IsSpace(rune(truncated[i])) {
			return truncated[:i]
		}
	}

	return truncated
}