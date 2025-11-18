package artificial

import (
	"net/http"
	"ximanager/sources/configuration"

	"github.com/sashabaranov/go-openai"
)

func NewOpenAIClient(client *http.Client, config *configuration.Config) *openai.Client {
	openaiConfig := openai.DefaultConfig(config.AI.OpenAIToken)
	openaiConfig.HTTPClient = client
	return openai.NewClientWithConfig(openaiConfig)
}