package texting

import (
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
	return TrimEndingBold(EscapeNecessary(TrimSpecials(input)))
}

func TrimSpecials(input string) string {
	result := headerRegex.ReplaceAllString(input, "$1")
	result = quoteRegex.ReplaceAllString(result, "$1$2>")
	return result
}

func TrimEndingBold(input string) string {
	return strings.TrimSuffix(input, "**") + "||"
}
