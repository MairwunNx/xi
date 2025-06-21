package texting

import (
	"strings"
)

const (
	specialSymbols = "[]()>#+-=|{}.!''"
	formatSymbols = "*_~|"
)

func EscapeMarkdown(input string) string {
	return escapeString(input, true)
}

func escapeString(input string, addClosingCodeBlock bool) string {
	if addClosingCodeBlock && len(strings.Split(input, "```"))%2 == 0 {
		input += "\n```"
	}

	var result []rune
	insideCodeBlock := false
	insideInlineCode := false
	
	insideBlocks := map[string]bool{
		"*":  false,
		"**": false,
		"_":  false,
		"__": false,
		"~":  false,
		"||": false, // Спойлеры
	}

	runes := []rune(input)
	i := 0
	
	for i < len(runes) {
		if codeBlockStartAt(runes, i) {
			insideCodeBlock = !insideCodeBlock
			result = append(result, []rune("```")...)
			i += 3
			continue
		}

		if insideCodeBlock {
			i = handleInsideCodeBlock(runes, &result, i)
			i++
			continue
		}

		if insideInlineCode {
			insideInlineCode = handleInsideInlineCode(runes, &result, i)
		} else {
			var newI int
			newI, insideInlineCode, insideBlocks = handleOutsideInlineCode(runes, &result, i, insideBlocks)
			i = newI
		}

		i++
	}

	return string(result)
}

func handleInsideCodeBlock(input []rune, result *[]rune, index int) int {
	if specialSymbolAt(input, index) {
		*result = append(*result, input[index])
	} else if inlineCodeAt(input, index) {
		*result = append(*result, []rune("\\`")...)
	} else if formatSymbolAt(input, index) {
		*result = append(*result, input[index])
	} else if codeBlockStartAt(input, index) {
		*result = append(*result, []rune("\\`\\`\\`")...)
		index += 2
	} else {
		*result = append(*result, input[index])
	}
	return index
}

func handleInsideInlineCode(input []rune, result *[]rune, index int) bool {
	insideInlineCode := true
	isSpecial := specialSymbolAt(input, index)
	isFormat := formatSymbolAt(input, index)
	
	if isSpecial || isFormat {
		*result = append(*result, '\\')
		*result = append(*result, input[index])
	} else if codeBlockStartAt(input, index) {
		*result = append(*result, []rune("\\`\\`\\`")...)
		index += 2
	} else if inlineCodeAt(input, index) {
		insideInlineCode = false
		*result = append(*result, '`')
	} else {
		*result = append(*result, input[index])
	}
	return insideInlineCode
}

func handleOutsideInlineCode(input []rune, result *[]rune, index int, insideBlocks map[string]bool) (int, bool, map[string]bool) {
	insideInlineCode := false
	
	if index+1 < len(input) && string(input[index:index+2]) == "||" {
		if insideBlocks["||"] {
			*result = append(*result, []rune("||")...)
			insideBlocks["||"] = false
			index++
		} else if hasClosingSymbolInLine(input, index, "||") {
			*result = append(*result, []rune("||")...)
			insideBlocks["||"] = true
			index++
		} else {
			*result = append(*result, []rune("\\|\\|")...)
			index++
		}
	} else if index+1 < len(input) && string(input[index:index+2]) == "**" {
		if insideBlocks["**"] {
			*result = append(*result, []rune("**")...)
			insideBlocks["**"] = false
			index++
		} else if hasClosingSymbolInLine(input, index, "**") {
			*result = append(*result, []rune("**")...)
			insideBlocks["**"] = true
			index++
		} else {
			*result = append(*result, []rune("\\*\\*")...)
			index++
		}
	} else if index+1 < len(input) && string(input[index:index+2]) == "__" {
		if insideBlocks["__"] {
			*result = append(*result, []rune("__")...)
			insideBlocks["__"] = false
			index++
		} else if hasClosingSymbolInLine(input, index, "__") {
			*result = append(*result, []rune("__")...)
			insideBlocks["__"] = true
			index++
		} else {
			*result = append(*result, []rune("\\__")...)
			index++
		}
	} else if input[index] == '*' {
		if insideBlocks["*"] {
			*result = append(*result, '*')
			insideBlocks["*"] = false
		} else if hasClosingSymbolInLine(input, index, "*") {
			*result = append(*result, '*')
			insideBlocks["*"] = true
		} else {
			*result = append(*result, []rune("\\*")...)
		}
	} else if input[index] == '_' {
		if insideBlocks["_"] {
			*result = append(*result, '_')
			insideBlocks["_"] = false
		} else if hasClosingSymbolInLine(input, index, "_") {
			*result = append(*result, '_')
			insideBlocks["_"] = true
		} else {
			*result = append(*result, []rune("\\_")...)
		}
	} else if input[index] == '~' {
		if insideBlocks["~"] {
			*result = append(*result, '~')
			insideBlocks["~"] = false
		} else if hasClosingSymbolInLine(input, index, "~") {
			*result = append(*result, '~')
			insideBlocks["~"] = true
		} else {
			*result = append(*result, []rune("\\~")...)
		}
	} else if input[index] == '>' && (index == 0 || input[index-1] == '\n') {
		*result = append(*result, []rune("\\>")...)
	} else if specialSymbolAt(input, index) {
		*result = append(*result, '\\')
		*result = append(*result, input[index])
	} else if formatSymbolAt(input, index) && input[index] != '|' {
		*result = append(*result, '\\')
		*result = append(*result, input[index])
	} else if inlineCodeAt(input, index) {
		if hasClosingSymbolInLine(input, index, "`") {
			insideInlineCode = true
			*result = append(*result, '`')
		} else {
			*result = append(*result, []rune("\\`")...)
		}
	} else if codeBlockStartAt(input, index) {
		*result = append(*result, []rune("\\`\\`\\`")...)
		index += 2
	} else {
		*result = append(*result, input[index])
	}
	
	return index, insideInlineCode, insideBlocks
}

func codeBlockStartAt(input []rune, index int) bool {
	return index+2 < len(input) && 
		input[index] == '`' && input[index+1] == '`' && input[index+2] == '`'
}

func inlineCodeAt(input []rune, index int) bool {
	return input[index] == '`' && !codeBlockStartAt(input, index)
}

func specialSymbolAt(input []rune, index int) bool {
	return strings.ContainsRune(specialSymbols, input[index])
}

func formatSymbolAt(input []rune, index int) bool {
	return strings.ContainsRune(formatSymbols, input[index])
}

func hasClosingSymbolInLine(input []rune, index int, symbol string) bool {
	symbolRunes := []rune(symbol)
	searchStart := index + len(symbolRunes)
	
	if searchStart >= len(input) {
		return false
	}
	
	endOfLine := len(input)
	for i := searchStart; i < len(input); i++ {
		if input[i] == '\n' {
			endOfLine = i
			break
		}
	}
	
	inputStr := string(input)
	searchStartStr := searchStart
	possibleClosingIndex := strings.Index(inputStr[searchStartStr:], symbol)
	
	if possibleClosingIndex == -1 {
		return false
	}
	
	actualIndex := searchStartStr + possibleClosingIndex
	return actualIndex <= endOfLine && actualIndex != index+len(symbolRunes)
}