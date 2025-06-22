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
		if strings.HasPrefix(line, `**\>`) {
			lines[i] = strings.Replace(line, `**\>`, "**>", 1)
			continue
		}
		
		// Проверяем обычные цитаты: \\> -> >
		if strings.HasPrefix(line, `\>`) {
			lines[i] = strings.Replace(line, `\>`, ">", 1)
			continue
		}
	}
	
	// Убираем экраны со ссылок
	result := strings.Join(lines, "\n")
	result = unescapeLinks(result)
	result = sanitizeAlerts(result)
	
	return result
}

func unescapeLinks(input string) string {
	result := input
	
	// Ищем полные ссылки в формате \[text\]\(url\) и заменяем их на [text](url)
	for {
		// Ищем начало ссылки \[
		startBracket := strings.Index(result, `\[`)
		if startBracket == -1 {
			break
		}
		
		// Ищем конец текста ссылки \]
		endBracket := strings.Index(result[startBracket:], `\]`)
		if endBracket == -1 {
			break
		}
		endBracket += startBracket
		
		// Проверяем что сразу после \] идет \(
		if endBracket+2 >= len(result) || !strings.HasPrefix(result[endBracket+2:], `\(`) {
			// Это не ссылка, пропускаем
			result = result[:startBracket] + result[startBracket+1:] // убираем \ но оставляем [
			continue
		}
		
		startParen := endBracket + 2
		
		// Ищем конец URL \)
		endParen := strings.Index(result[startParen:], `\)`)
		if endParen == -1 {
			break
		}
		endParen += startParen
		
		// Извлекаем части ссылки
		linkText := result[startBracket+2:endBracket] // текст между \[ и \]
		linkUrl := result[startParen+2:endParen]     // URL между \( и \)
		
		// Заменяем экранированную ссылку на нормальную
		oldLink := result[startBracket:endParen+2]
		newLink := "[" + linkText + "](" + linkUrl + ")"
		
		result = strings.Replace(result, oldLink, newLink, 1)
	}
	
	return result
}



func sanitizeAlerts(input string) string {
	if strings.HasSuffix(input, "||**") {
		return strings.TrimSuffix(input, "**")
	}
	return input
}