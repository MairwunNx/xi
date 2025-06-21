package texting

import (
	"strings"
)

var shouldBeEscaped = "_*[]()~`>#+-=|{}.!"

func EscapeMarkdown(input string) string {
	var result []rune
	var escaped bool
	for _, r := range input {
		if r == '\\' {
			escaped = !escaped
			result = append(result, r)
			continue
		}
		if strings.ContainsRune(shouldBeEscaped, r) && !escaped {
			result = append(result, '\\')
		}
		escaped = false
		result = append(result, r)
	}
	return string(result)
}