package texting

import (
	"strings"
)

func ParseCmdArgs(args string) []string {
	var result []string
	var current strings.Builder
	inQuotes := false
	escaped := false

	for i := 0; i < len(args); i++ {
		ch := args[i]

		if escaped {
			if ch == '\'' || ch == '\\' {
				current.WriteByte(ch)
			} else {
				current.WriteByte('\\')
				current.WriteByte(ch)
			}
			escaped = false
			continue
		}

		if ch == '\\' {
			escaped = true
			continue
		}

		if ch == '\'' {
			inQuotes = !inQuotes
			continue
		}

		if ch == ' ' && !inQuotes {
			if current.Len() > 0 {
				result = append(result, current.String())
				current.Reset()
			}
			continue
		}

		current.WriteByte(ch)
	}

	if current.Len() > 0 {
		result = append(result, current.String())
	}

	var filteredResult []string
	for _, arg := range result {
		if strings.TrimSpace(arg) != "" {
			filteredResult = append(filteredResult, arg)
		}
	}

	return filteredResult
}