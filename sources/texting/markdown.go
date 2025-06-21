package texting

import (
	"strings"
)

func EscapeMarkdown(input string) string {
	var str strings.Builder
	escapable := "_*[]()~`>#+-=|{}.!\\"
	for _, char := range input {
		if strings.ContainsRune(escapable, char) {
			str.WriteRune('\\')
		}
		str.WriteRune(char)
	}
	return str.String()
}