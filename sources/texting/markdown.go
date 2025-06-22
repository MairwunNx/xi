package texting

import (
	"strings"
)

const escapable = "[]()>#+-={}.!"

func EscapeMarkdown(input string) string {
	var str strings.Builder
	for _, char := range input {
		if strings.ContainsRune(escapable, char) {
			str.WriteRune('\\')
		}
		str.WriteRune(char)
	}
	
	result := str.String()
	result = RemoveRestrictedMarkdown(result)
	result = RemoveEscapedMarkdown(result)
	return result
}

func RemoveRestrictedMarkdown(input string) string {
	lines := strings.Split(input, "\n")
	
	// Обрабатываем заголовки
	for i, line := range lines {
		if len(line) == 0 {
			continue
		}
		
		// Проверяем начинается ли строка с экранированными решетками \\#
		j := 0
		for j < len(line)-2 && line[j] == '\\' && line[j+1] == '\\' && line[j+2] == '#' {
			j += 3 // пропускаем \\#
		}
		
		// Если найдены экранированные #, удаляем их и следующий пробел
		if j > 0 {
			// Пропускаем пробел после последней \\#
			if j < len(line) && line[j] == ' ' {
				j++
			}
			lines[i] = line[j:]
		}
	}
	
	// Убираем лишние ньюлайны после кодовых блоков
	for i := 0; i < len(lines); i++ {
		trimmedLine := strings.TrimSpace(lines[i])
		if trimmedLine == "```" {
			// Убираем все пустые строки после кодового блока
			j := i + 1
			for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
				j++
			}
			if j > i + 1 {
				// Удаляем пустые строки, оставляем только одну
				lines = append(lines[:i+1], append([]string{""}, lines[j:]...)...)
			}
		}
	}
	
	// Убираем сепараторы ---
	filteredLines := make([]string, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "---" {
			// Проверяем контекст: если это сепаратор между параграфами
			if i > 0 && i < len(lines)-1 {
				// Пропускаем сепаратор и возможные пустые строки после него
				j := i + 1
				for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
					j++
				}
				// Добавляем один ньюлайн вместо сепаратора
				if len(filteredLines) > 0 && filteredLines[len(filteredLines)-1] != "" {
					filteredLines = append(filteredLines, "")
				}
				i = j - 1 // -1 потому что цикл сделает i++
				continue
			}
		}
		filteredLines = append(filteredLines, lines[i])
	}
	
	return strings.Join(filteredLines, "\n")
}

func RemoveEscapedMarkdown(input string) string {
	lines := strings.Split(input, "\n")
	
	for i, line := range lines {
		if len(line) == 0 {
			continue
		}
		
		// Проверяем коллапсируемые цитаты: **\\> -> **>
		if strings.HasPrefix(line, "**\\\\>") {
			lines[i] = strings.Replace(line, "**\\\\>", "**>", 1)
			continue
		}
		
		// Проверяем обычные цитаты: \\> -> >
		if strings.HasPrefix(line, "\\\\>") {
			lines[i] = strings.Replace(line, "\\\\>", ">", 1)
			continue
		}
	}
	
	// Убираем экраны со ссылок
	result := strings.Join(lines, "\n")
	result = unescapeLinks(result)
	
	return result
}

func unescapeLinks(input string) string {
	var result strings.Builder
	runes := []rune(input)
	
	for i := 0; i < len(runes); i++ {
		if i < len(runes)-2 && runes[i] == '\\' && runes[i+1] == '\\' {
			// Проверяем на экранированные символы ссылок
			next := runes[i+2]
			if next == '[' || next == ']' || next == '(' || next == ')' {
				// Проверяем что это может быть частью ссылки
				if isPartOfLink(runes, i+1) {
					// Пропускаем двойной экран, записываем только символ
					result.WriteRune(next)
					i += 2 // пропускаем следующие два символа
					continue
				}
			}
		}
		result.WriteRune(runes[i])
	}
	
	return result.String()
}

func isPartOfLink(runes []rune, pos int) bool {
	if pos >= len(runes)-1 {
		return false
	}
	
	char := runes[pos+1]
	
	// Простая проверка: ищем паттерн \\[text\\]\\(url\\)
	if char == '[' {
		// Ищем вперед \\]
		i := pos + 2
		foundClosingBracket := false
		for i < len(runes)-2 {
			if runes[i] == '\\' && runes[i+1] == '\\' && i+2 < len(runes) && runes[i+2] == ']' {
				foundClosingBracket = true
				i += 3
				break
			}
			i++
		}
		// Проверяем что после \\] идет \\(
		if foundClosingBracket && i < len(runes)-2 && runes[i] == '\\' && runes[i+1] == '\\' && runes[i+2] == '(' {
			return true
		}
	} else if char == ']' {
		// Проверяем что после текущей позиции идет \\(
		if pos+4 < len(runes) && runes[pos+2] == '\\' && runes[pos+3] == '\\' && runes[pos+4] == '(' {
			// Ищем назад \\[
			for i := pos - 2; i >= 2; i-- {
				if runes[i-2] == '\\' && runes[i-1] == '\\' && runes[i] == '[' {
					return true
				}
			}
		}
	} else if char == '(' {
		// Ищем назад \\]
		for i := pos - 2; i >= 2; i-- {
			if runes[i-2] == '\\' && runes[i-1] == '\\' && runes[i] == ']' {
				return true
			}
		}
	} else if char == ')' {
		// Ищем назад \\(
		for i := pos - 2; i >= 2; i-- {
			if runes[i-2] == '\\' && runes[i-1] == '\\' && runes[i] == '(' {
				return true
			}
		}
	}
	
	return false
}
