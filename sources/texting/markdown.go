package texting

import (
	"bytes"
	"log/slog"
	"strings"

	tgmd "github.com/Mad-Pixels/goldmark-tgmd"
)

// EscapeMarkdown преобразует обычный markdown в Telegram MarkdownV2 формат
func EscapeMarkdown(input string) string {
	// Используем профессиональную библиотеку для конвертации в Telegram MarkdownV2
	md := tgmd.TGMD()
	var buf bytes.Buffer
	
	if err := md.Convert([]byte(input), &buf); err != nil {
		slog.Error("Error converting markdown to Telegram MarkdownV2", "inner_error", err)
		return escapeBasic(input)
	}

	processed := buf.String()
	
	processed = RemoveRestrictedMarkdown(processed)
	processed = sanitizeAlerts(processed)

	return processed
}

// escapeBasic - запасной вариант экранирования если goldmark-tgmd не сработал
func escapeBasic(input string) string {
	const escapable = "_*[]()~`>#+-=|{}.!"
	
	var result strings.Builder
	for _, char := range input {
		if strings.ContainsRune(escapable, char) {
			result.WriteRune('\\')
		}
		result.WriteRune(char)
	}
	
	return result.String()
}

func RemoveRestrictedMarkdown(input string) string {
	lines := strings.Split(input, "\n")
	
	// Обрабатываем заголовки
	for i, line := range lines {
		if len(line) == 0 {
			continue
		}
		
		// Проверяем начинается ли строка с экранированными решетками \#
		j := 0
		for j < len(line)-1 && line[j] == '\\' {
			j += 2 // пропускаем \#
		}
		
		// Если найдены экранированные #, удаляем их и следующий пробел
		if j > 0 {
			// Пропускаем пробел после последней \#
			if j < len(line) && line[j] == ' ' {
				j++
			}
			lines[i] = line[j:]
		}
	}
	
	// Убираем лишние ньюлайны после кодовых блоков
	for i := 0; i < len(lines); i++ {
		trimmedLine := strings.TrimSpace(lines[i])
		// Проверяем что строка заканчивается на ``` (может быть с языком в начале)
		if strings.HasSuffix(trimmedLine, "```") || trimmedLine == "```" {
			// Убираем все пустые строки после кодового блока
			j := i + 1
			for j < len(lines) && strings.TrimSpace(lines[j]) == "" {
				j++
			}
			if j > i + 1 {
				// Удаляем лишние пустые строки
				lines = append(lines[:i+1], lines[j:]...)
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

func sanitizeAlerts(input string) string {
	if strings.HasSuffix(input, "||**") {
		return strings.TrimSuffix(input, "**")
	}
	return input
}