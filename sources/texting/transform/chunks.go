package transform

func Chunks(text string, cs int) []string {
	runes := []rune(text)
	var chunks []string
	for i := 0; i < len(runes); i += cs {
		end := i + cs
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[i:end]))
	}
	return chunks
}