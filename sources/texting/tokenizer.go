package texting

import (
	"ximanager/sources/tracing"

	"github.com/pkoukk/tiktoken-go"
)

func Tokens(log *tracing.Logger, text string) int {
	tkm, err := tiktoken.GetEncoding("o200k_base")
	if err != nil {
		tokens := len(text) / 4
		log.E("An error occurred while getting tiktoken encoding, fallback tokens used", tracing.AiTokens, tokens, tracing.InnerError, err)
		return tokens
	}

	return tracing.ReportExecutionForRIn(log,
		func() int { return len(tkm.Encode(text, nil, nil)) },
		func(l *tracing.Logger, tokens int) { l.I("Tokens counted", tracing.AiTokens, tokens) },
	)
}

const (
	tokenCalculationBias   = 50
	minimumAvailableTokens = 1000
)

func TokensInfer(log *tracing.Logger, prompt, req, persona string, mt int) int {
	tokens := Tokens(log, prompt)
	reqTokens := Tokens(log, req)

	availableTokens := mt - tokens - reqTokens - tokenCalculationBias
	if availableTokens < minimumAvailableTokens {
		return minimumAvailableTokens
	}
	return availableTokens
}
