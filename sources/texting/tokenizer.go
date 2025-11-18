package texting

import (
	"ximanager/sources/tracing"

	"github.com/pkoukk/tiktoken-go"
)

var tkm, _ = tiktoken.GetEncoding("o200k_base")

func Tokens(log *tracing.Logger, text string) int {
	return tracing.ReportExecutionForRIn(log,
		func() int { return len(tkm.Encode(text, nil, nil)) },
		func(l *tracing.Logger, tokens int) { l.I("Tokens counted", tracing.AiTokens, tokens) },
	)
}
