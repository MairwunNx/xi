package artificial

import (
	"net/http"
	"ximanager/sources/platform"

	openrouter "github.com/revrost/go-openrouter"
)

func NewOpenRouterClient(config *AIConfig, client *http.Client) *openrouter.Client {
	clientConfig := openrouter.DefaultConfig(config.OpenRouterToken)
	clientConfig.HTTPClient = client
	clientConfig.XTitle = "Xi Manager"
	clientConfig.HttpReferer = "https://github.com/mairwunnx/xi"

	return openrouter.NewClientWithConfig(*clientConfig)
}

func OpenRouterMessageStackFrom(pairs []platform.RedisMessage) []openrouter.ChatCompletionMessage {
	var messages []openrouter.ChatCompletionMessage

	for i := len(pairs) - 1; i >= 0; i-- {
		pair := pairs[i]

		uc := pair.Content
		if uc != "" {
			messages = append(messages, openrouter.ChatCompletionMessage{
				Role:    openrouter.ChatMessageRoleUser,
				Content: openrouter.Content{Text: uc},
			})
		}

		if pair.Role == platform.MessageRoleAssistant {
			xc := pair.Content
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