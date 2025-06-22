package texting

import (
	"strings"
)

const escapable = "_[]()~>#+-=|{}.!"

func EscapeMarkdown(input string) string {
	var str strings.Builder
	for _, char := range input {
		if strings.ContainsRune(escapable, char) {
			str.WriteRune('\\')
		}
		str.WriteRune(char)
	}
	
	return UnescapeSpecials(str.String())
}

func UnescapeSpecials(input string) string {
	result := input
	
	// Убираем экранирование для символа > после жирного текста
	result = strings.ReplaceAll(result, "**\\>", "**>")
	
	// Убираем экранирование звездочек внутри жирного текста **text \* text**
	result = strings.ReplaceAll(result, "**\\*", "***")
	result = strings.ReplaceAll(result, "\\***", "***")
	
	// Убираем экранирование подчеркиваний внутри подчеркнутого текста __text \_ text__
	result = strings.ReplaceAll(result, "__\\_", "__")
	result = strings.ReplaceAll(result, "\\___", "___")
	
	// Убираем экранирование тильд внутри зачеркнутого текста ~text \~ text~
	result = strings.ReplaceAll(result, "~\\~", "~~")
	
	// Убираем экранирование вертикальных черт внутри спойлеров ||text \| text||
	result = strings.ReplaceAll(result, "||\\|", "|||")
	result = strings.ReplaceAll(result, "\\|||", "|||")
	
	// Убираем экранирование @ внутри markdown конструкций (например **@username**)
	result = strings.ReplaceAll(result, "**\\@", "**@")
	result = strings.ReplaceAll(result, "_\\@", "_@")
	result = strings.ReplaceAll(result, "__\\@", "__@")
	result = strings.ReplaceAll(result, "~\\@", "~@")
	result = strings.ReplaceAll(result, "||\\@", "||@")
	
	return result
}
