package tokenizer

import (
	"ximanager/sources/tracing"

	"github.com/pkoukk/tiktoken-go"
)

var tkm, _ = tiktoken.GetEncoding("o200k_base")

func Tokens(log *tracing.Logger, text string) int {
  defer tracing.ProfilePoint(log, "Tokens counted", "tokenizer.tiktoken.tokens")()
	return len(tkm.Encode(text, nil, nil))
}