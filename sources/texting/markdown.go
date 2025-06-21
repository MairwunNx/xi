package texting

import (
	"strings"
)

const escapable = "*_[]()~>#+-=|{}.!"
func EscapeMarkdown(input string) string {
	var str strings.Builder
	for _, char := range input {
		if strings.ContainsRune(escapable, char) {
			str.WriteRune('\\')
		}
		str.WriteRune(char)
	}
	return str.String()
}
