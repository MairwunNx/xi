package artificial

import (
	"net/http"
	"ximanager/sources/repository"

	openrouter "github.com/revrost/go-openrouter"
)

func NewOpenRouterClient(config *AIConfig, client *http.Client) *openrouter.Client {
	clientConfig := openrouter.DefaultConfig(config.OpenRouterToken)
	clientConfig.HTTPClient = client
	clientConfig.XTitle = "Xi Manager"
	clientConfig.HttpReferer = "https://github.com/mairwunnx/xi"

	return openrouter.NewClientWithConfig(*clientConfig)
}

func OpenRouterMessageStackFrom(pairs []repository.MessagePair) []openrouter.ChatCompletionMessage {
	var messages []openrouter.ChatCompletionMessage

	for i := len(pairs) - 1; i >= 0; i-- {
		pair := pairs[i]

		uc := string(pair.UserMessage.MessageText)
		if uc != "" {
			messages = append(messages, openrouter.ChatCompletionMessage{
				Role:    openrouter.ChatMessageRoleUser,
				Content: openrouter.Content{Text: uc},
			})
		}

		if pair.XiResponse != nil {
			xc := string(pair.XiResponse.MessageText)
			if xc != "" {
				messages = append(messages, openrouter.ChatCompletionMessage{
					Role:    openrouter.ChatMessageRoleAssistant,
					Content: openrouter.Content{Text: xc},
				})
			}
		}
	}

	return messages
}