package texting

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

const escapable = "_[]()~>#+-={}.!`\\"
var headerRegex = regexp.MustCompile(`(^|\n)#{1,6}\s?[^\n]*`)
var quoteRegex = regexp.MustCompile(`(^|\n)(\*\*)?>\s`)

// Типы контекстов для парсинга
type ContextType int

const (
	ContextNormal ContextType = iota
	ContextInlineCode
	ContextPreCode
	ContextLinkText
	ContextLinkURL
	ContextEmojiURL
	ContextBlockQuote
)

// Состояние парсера
type ParserState struct {
	context    ContextType
	stack      []rune // стек открытых тегов
	inEscape   bool   // следующий символ экранирован
	codeBlocks int    // счетчик ``` блоков
}

func EscapeNecessary(input string) string {
	runes := []rune(input)
	result := strings.Builder{}
	state := &ParserState{context: ContextNormal}
	
	for i := 0; i < len(runes); i++ {
		char := runes[i]
		
		// Обработка экранированных символов
		if state.inEscape {
			result.WriteRune(char)
			state.inEscape = false
			continue
		}
		
		if char == '\\' {
			state.inEscape = true
			result.WriteRune(char)
			continue
		}
		
		// Обработка в зависимости от контекста
		switch state.context {
		case ContextInlineCode:
			if char == '`' {
				state.context = ContextNormal
				result.WriteRune(char)
			} else {
				if char == '\\' || char == '`' {
					result.WriteRune('\\')
				}
				result.WriteRune(char)
			}
			continue
			
		case ContextPreCode:
			if char == '`' && i+2 < len(runes) && runes[i+1] == '`' && runes[i+2] == '`' {
				state.codeBlocks--
				if state.codeBlocks == 0 {
					state.context = ContextNormal
				}
				result.WriteString("```")
				i += 2
			} else {
				if char == '\\' || char == '`' {
					result.WriteRune('\\')
				}
				result.WriteRune(char)
			}
			continue
			
		case ContextLinkURL, ContextEmojiURL:
			if char == ')' {
				state.context = ContextNormal
				result.WriteRune(char)
			} else {
				if char == ')' || char == '\\' {
					result.WriteRune('\\')
				}
				result.WriteRune(char)
			}
			continue
		}
		
		// Основная логика для нормального контекста
		switch char {
		case '`':
			if i+2 < len(runes) && runes[i+1] == '`' && runes[i+2] == '`' {
				// Начало блока кода
				state.context = ContextPreCode
				state.codeBlocks++
				result.WriteString("```")
				i += 2
			} else {
				// Inline code
				state.context = ContextInlineCode
				result.WriteRune(char)
			}
			
		case '*':
			if shouldEscapeFormatting(runes, i, '*', state) {
				result.WriteRune('\\')
			}
			result.WriteRune(char)
			
		case '_':
			if shouldEscapeFormatting(runes, i, '_', state) {
				result.WriteRune('\\')
			}
			result.WriteRune(char)
			
		case '~':
			if shouldEscapeFormatting(runes, i, '~', state) {
				result.WriteRune('\\')
			}
			result.WriteRune(char)
			
		case '|':
			if shouldEscapePipes(runes, i, state) {
				result.WriteRune('\\')
			}
			result.WriteRune(char)
			
		case '[':
			if isLinkStart(runes, i) {
				state.context = ContextLinkText
				result.WriteRune(char)
			} else {
				result.WriteRune('\\')
				result.WriteRune(char)
			}
			
		case ']':
			if state.context == ContextLinkText {
				if i+1 < len(runes) && runes[i+1] == '(' {
					state.context = ContextLinkURL
					result.WriteRune(char)
				} else {
					state.context = ContextNormal
					result.WriteRune(char)
				}
			} else {
				result.WriteRune('\\')
				result.WriteRune(char)
			}
			
		case '!':
			if i+1 < len(runes) && runes[i+1] == '[' && isEmojiStart(runes, i) {
				result.WriteRune(char)
			} else {
				result.WriteRune('\\')
				result.WriteRune(char)
			}
			
		case '>':
			if isBlockQuoteStart(runes, i) {
				state.context = ContextBlockQuote
				result.WriteRune(char)
			} else {
				result.WriteRune('\\')
				result.WriteRune(char)
			}
			
		case '\n':
			if state.context == ContextBlockQuote {
				state.context = ContextNormal
			}
			result.WriteRune(char)
			
		default:
			if strings.ContainsRune(escapable, char) {
				result.WriteRune('\\')
			}
			result.WriteRune(char)
		}
	}
	
	return result.String()
}

// Определяет, нужно ли экранировать символ форматирования
func shouldEscapeFormatting(runes []rune, pos int, char rune, state *ParserState) bool {
	// Проверяем, может ли это быть началом или концом форматирования
	if pos == 0 || pos == len(runes)-1 {
		return !isValidFormatBoundary(runes, pos, char)
	}
	
	// Проверяем соседние символы
	prevChar := runes[pos-1]
	nextChar := runes[pos+1]
	
	// Если окружен пробелами с обеих сторон - скорее всего не форматирование
	if unicode.IsSpace(prevChar) && unicode.IsSpace(nextChar) {
		return true
	}
	
	// Дополнительные проверки для конкретных символов
	switch char {
	case '*':
		return !isValidAsteriskFormat(runes, pos)
	case '_':
		return !isValidUnderscoreFormat(runes, pos)
	case '~':
		return !isValidTildeFormat(runes, pos)
	}
	
	return false
}

// Проверяет валидность границы форматирования
func isValidFormatBoundary(runes []rune, pos int, char rune) bool {
	if pos == 0 {
		return pos+1 < len(runes) && !unicode.IsSpace(runes[pos+1])
	}
	if pos == len(runes)-1 {
		return pos > 0 && !unicode.IsSpace(runes[pos-1])
	}
	return true
}

// Проверяет, является ли это валидным форматированием звездочкой
func isValidAsteriskFormat(runes []rune, pos int) bool {
	// Проверяем на двойные звездочки для курсива
	if pos+1 < len(runes) && runes[pos+1] == '*' {
		return true // **italic**
	}
	if pos > 0 && runes[pos-1] == '*' {
		return true // **italic**
	}
	
	// Проверяем на одинарные звездочки для жирного
	if pos > 0 && pos+1 < len(runes) {
		prevNotSpace := !unicode.IsSpace(runes[pos-1])
		nextNotSpace := !unicode.IsSpace(runes[pos+1])
		return prevNotSpace || nextNotSpace
	}
	
	return false
}

// Проверяет, является ли это валидным форматированием подчеркиванием
func isValidUnderscoreFormat(runes []rune, pos int) bool {
	// Проверяем на двойные подчеркивания
	if pos+1 < len(runes) && runes[pos+1] == '_' {
		return true // __underline__
	}
	if pos > 0 && runes[pos-1] == '_' {
		return true // __underline__
	}
	
	// Одинарные подчеркивания для курсива
	if pos > 0 && pos+1 < len(runes) {
		prevNotSpace := !unicode.IsSpace(runes[pos-1])
		nextNotSpace := !unicode.IsSpace(runes[pos+1])
		return prevNotSpace || nextNotSpace
	}
	
	return false
}

// Проверяет, является ли это валидным зачеркиванием
func isValidTildeFormat(runes []rune, pos int) bool {
	if pos > 0 && pos+1 < len(runes) {
		prevNotSpace := !unicode.IsSpace(runes[pos-1])
		nextNotSpace := !unicode.IsSpace(runes[pos+1])
		return prevNotSpace || nextNotSpace
	}
	return false
}

// Проверяет, нужно ли экранировать символы труб (спойлеры)
func shouldEscapePipes(runes []rune, pos int, state *ParserState) bool {
	if pos+1 < len(runes) && runes[pos+1] == '|' {
		return false // ||spoiler||
	}
	if pos > 0 && runes[pos-1] == '|' {
		return false // ||spoiler||
	}
	return true
}

// Проверяет, является ли это началом ссылки
func isLinkStart(runes []rune, pos int) bool {
	// Ищем соответствующую закрывающую скобку и следующую за ней открывающую круглую
	depth := 1
	for i := pos + 1; i < len(runes); i++ {
		switch runes[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return i+1 < len(runes) && runes[i+1] == '('
			}
		case '\n':
			return false // ссылки не могут быть многострочными
		}
	}
	return false
}

// Проверяет, является ли это началом emoji
func isEmojiStart(runes []rune, pos int) bool {
	if pos+1 >= len(runes) || runes[pos+1] != '[' {
		return false
	}
	
	// Ищем паттерн ![emoji](tg://emoji?id=...)
	for i := pos + 2; i < len(runes); i++ {
		if runes[i] == ']' {
			if i+1 < len(runes) && runes[i+1] == '(' {
				// Проверяем, что после ( идет tg://emoji
				remaining := string(runes[i+2:])
				return strings.HasPrefix(remaining, "tg://emoji")
			}
			break
		}
		if runes[i] == '\n' {
			break
		}
	}
	return false
}

// Проверяет, является ли это началом блочной цитаты
func isBlockQuoteStart(runes []rune, pos int) bool {
	// > должен быть в начале строки или после переноса
	if pos == 0 {
		return true
	}
	if runes[pos-1] == '\n' {
		return true
	}
	// Проверяем на expandable quote **>
	if pos >= 2 && runes[pos-1] == '*' && runes[pos-2] == '*' {
		return true
	}
	return false
}

func EscapeMarkdown(input string) string {
	return EscapeNecessary2(TrimSpecials(input))
}

// EscapeMarkdown2 - альтернативная реализация для тестирования
func EscapeMarkdown2(input string) string {
	return EscapeNecessary2(TrimSpecials(input))
}

func TrimSpecials(input string) string {
	result := headerRegex.ReplaceAllString(input, "$1")
	result = quoteRegex.ReplaceAllString(result, "$1$2>")
	return result
}

// EscapeNecessary2 - новая реализация основанная на telegramify-markdown
func EscapeNecessary2(input string) string {
	if input == "" {
		return ""
	}
	
	// Этап 1: Заменяем ** на * для жирного текста (как в telegramify-markdown)
	input = convertBoldMarkdown(input)
	
	// Этап 2: Первый проход - экранируем все специальные символы
	firstPass := escapeAllSpecials(input)
	
	// Этап 3: Второй проход - убираем двойное экранирование в контекстах разметки
	finalResult := removeDoubleEscaping(firstPass)
	
	return finalResult
}

// Конвертирует ** в * для жирного текста, но только в контексте разметки
func convertBoldMarkdown(input string) string {
	// Умно заменяем **text** на *text*, но НЕ трогаем **> (expandable quotes)
	boldRegex := regexp.MustCompile(`\*\*([^\*\n\r>]+?)\*\*`)
	return boldRegex.ReplaceAllString(input, "*$1*")
}

// Первый проход - экранирует все специальные символы
func escapeAllSpecials(input string) string {
	// Экранируем все специальные символы (как в telegramify-markdown)
	specialChars := regexp.MustCompile(`([_*\\[\]()~` + "`>#\\+\\-=|{}.!\\\\])")
	return specialChars.ReplaceAllString(input, "\\$1")
}

// Второй проход - убирает двойное экранирование в допустимых контекстах
func removeDoubleEscaping(input string) string {
	result := input
	
	// Убираем двойное экранирование для специальных символов в контекстах разметки
	doubleEscapeRegex := regexp.MustCompile(`\\\\([_*\\[\]()~` + "`>#\\+\\-=|{}.!\\\\])")
	result = doubleEscapeRegex.ReplaceAllString(result, "\\$1")
	
	// Обработка цитат - убираем экранирование > в начале строки
	result = unescapeQuotes(result)
	
	// Обработка ссылок - убираем экранирование внутри [text](url)
	result = unescapeLinks(result)
	
	// Обработка кода - убираем экранирование внутри `code` и ```code```
	result = unescapeCode(result)
	
	// Обработка форматирования - убираем экранирование внутри *bold*, _italic_, ~strike~, ||spoiler||
	result = unescapeFormatting(result)
	
	return result
}

// Убирает экранирование > в цитатах
func unescapeQuotes(input string) string {
	result := input
	
	// Обычные цитаты: \> в начале строки → >
	quoteStartRegex := regexp.MustCompile(`(^|\n)\\>`)
	result = quoteStartRegex.ReplaceAllString(result, "$1>")
	
	// Expandable цитаты: **\> → **> (ищем именно две звездочки!)
	expandableQuoteRegex := regexp.MustCompile(`\*\*\\>`)
	result = expandableQuoteRegex.ReplaceAllString(result, "**>")
	
	return result
}

// Убирает экранирование внутри ссылок [text](url)
func unescapeLinks(input string) string {
	// Находим все ссылки и убираем экранирование внутри них
	linkRegex := regexp.MustCompile(`\[([^\]]*)\]\(([^)]*)\)`)
	
	return linkRegex.ReplaceAllStringFunc(input, func(match string) string {
		parts := linkRegex.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}
		
		text := parts[1]
		url := parts[2]
		
		// Убираем экранирование всего кроме ] и ) внутри ссылки
		text = strings.ReplaceAll(text, "\\[", "[")
		text = strings.ReplaceAll(text, "\\*", "*")
		text = strings.ReplaceAll(text, "\\_", "_")
		text = strings.ReplaceAll(text, "\\~", "~")
		text = strings.ReplaceAll(text, "\\|", "|")
		
		// В URL убираем экранирование всего кроме ) и \
		url = strings.ReplaceAll(url, "\\[", "[")
		url = strings.ReplaceAll(url, "\\]", "]")
		url = strings.ReplaceAll(url, "\\*", "*")
		url = strings.ReplaceAll(url, "\\_", "_")
		url = strings.ReplaceAll(url, "\\~", "~")
		url = strings.ReplaceAll(url, "\\|", "|")
		url = strings.ReplaceAll(url, "\\#", "#")
		url = strings.ReplaceAll(url, "\\+", "+")
		url = strings.ReplaceAll(url, "\\-", "-")
		url = strings.ReplaceAll(url, "\\=", "=")
		url = strings.ReplaceAll(url, "\\{", "{")
		url = strings.ReplaceAll(url, "\\}", "}")
		url = strings.ReplaceAll(url, "\\.", ".")
		url = strings.ReplaceAll(url, "\\!", "!")
		
		return fmt.Sprintf("[%s](%s)", text, url)
	})
}

// Убирает экранирование внутри кода
func unescapeCode(input string) string {
	result := input
	
	// Inline code: `code`
	inlineCodeRegex := regexp.MustCompile("(`+)([^`]*?)(`+)")
	result = inlineCodeRegex.ReplaceAllStringFunc(result, func(match string) string {
		parts := inlineCodeRegex.FindStringSubmatch(match)
		if len(parts) != 4 {
			return match
		}
		
		openTicks := parts[1]
		code := parts[2]
		closeTicks := parts[3]
		
		// Внутри кода убираем экранирование всего кроме ` и \
		unescapedCode := code
		for _, char := range "_*[]()~>#+-=|{}.!" {
			unescapedCode = strings.ReplaceAll(unescapedCode, "\\"+string(char), string(char))
		}
		
		return openTicks + unescapedCode + closeTicks
	})
	
	// Block code: ```code```
	blockCodeRegex := regexp.MustCompile("(?s)(```[^\\n]*\\n)(.*?)(```)")
	result = blockCodeRegex.ReplaceAllStringFunc(result, func(match string) string {
		parts := blockCodeRegex.FindStringSubmatch(match)
		if len(parts) != 4 {
			return match
		}
		
		openBlock := parts[1]
		code := parts[2]
		closeBlock := parts[3]
		
		// Внутри блока кода убираем экранирование всего кроме ` и \
		unescapedCode := code
		for _, char := range "_*[]()~>#+-=|{}.!" {
			unescapedCode = strings.ReplaceAll(unescapedCode, "\\"+string(char), string(char))
		}
		
		return openBlock + unescapedCode + closeBlock
	})
	
	return result
}

// Убирает экранирование внутри форматирования
func unescapeFormatting(input string) string {
	result := input
	
	// *bold* - убираем экранирование внутри жирного текста
	boldRegex := regexp.MustCompile(`\*([^*\n]+?)\*`)
	result = boldRegex.ReplaceAllStringFunc(result, func(match string) string {
		parts := boldRegex.FindStringSubmatch(match)
		if len(parts) != 2 {
			return match
		}
		content := parts[1]
		// Убираем экранирование символов внутри жирного текста
		content = strings.ReplaceAll(content, "\\@", "@")
		content = strings.ReplaceAll(content, "\\#", "#")
		content = strings.ReplaceAll(content, "\\.", ".")
		return "*" + content + "*"
	})
	
	// _italic_ - убираем экранирование внутри курсива
	italicRegex := regexp.MustCompile(`_([^_\n]+?)_`)
	result = italicRegex.ReplaceAllStringFunc(result, func(match string) string {
		parts := italicRegex.FindStringSubmatch(match)
		if len(parts) != 2 {
			return match
		}
		content := parts[1]
		content = strings.ReplaceAll(content, "\\@", "@")
		content = strings.ReplaceAll(content, "\\#", "#")
		content = strings.ReplaceAll(content, "\\.", ".")
		return "_" + content + "_"
	})
	
	// __underline__ - убираем экранирование внутри подчеркивания
	underlineRegex := regexp.MustCompile(`__([^_\n]+?)__`)
	result = underlineRegex.ReplaceAllStringFunc(result, func(match string) string {
		parts := underlineRegex.FindStringSubmatch(match)
		if len(parts) != 2 {
			return match
		}
		content := parts[1]
		content = strings.ReplaceAll(content, "\\@", "@")
		content = strings.ReplaceAll(content, "\\#", "#")
		content = strings.ReplaceAll(content, "\\.", ".")
		return "__" + content + "__"
	})
	
	// ~strikethrough~ - убираем экранирование внутри зачеркивания
	strikeRegex := regexp.MustCompile(`~([^~\n]+?)~`)
	result = strikeRegex.ReplaceAllStringFunc(result, func(match string) string {
		parts := strikeRegex.FindStringSubmatch(match)
		if len(parts) != 2 {
			return match
		}
		content := parts[1]
		content = strings.ReplaceAll(content, "\\@", "@")
		content = strings.ReplaceAll(content, "\\#", "#")
		content = strings.ReplaceAll(content, "\\.", ".")
		return "~" + content + "~"
	})
	
	// ||spoiler|| - убираем экранирование внутри спойлеров
	spoilerRegex := regexp.MustCompile(`\|\|([^|\n]+?)\|\|`)
	result = spoilerRegex.ReplaceAllStringFunc(result, func(match string) string {
		parts := spoilerRegex.FindStringSubmatch(match)
		if len(parts) != 2 {
			return match
		}
		content := parts[1]
		content = strings.ReplaceAll(content, "\\@", "@")
		content = strings.ReplaceAll(content, "\\#", "#")
		content = strings.ReplaceAll(content, "\\.", ".")
		return "||" + content + "||"
	})
	
	return result
}
