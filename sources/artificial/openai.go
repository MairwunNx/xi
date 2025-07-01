package artificial

import (
	"net/http"

	"github.com/sashabaranov/go-openai"
)

func NewOpenAIClient(client *http.Client, config *AIConfig) *openai.Client {
	openaiConfig := openai.DefaultConfig(config.OpenAIToken)
	openaiConfig.HTTPClient = client
	return openai.NewClientWithConfig(openaiConfig)
}