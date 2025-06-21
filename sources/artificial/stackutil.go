package artificial

import (
	"ximanager/sources/repository"

	deepseek "github.com/cohesion-org/deepseek-go"
	constants "github.com/cohesion-org/deepseek-go/constants"
	anthropic "github.com/liushuangls/go-anthropic/v2"
	"github.com/sashabaranov/go-openai"
)

func MessagePairsToOpenAI(pairs []repository.MessagePair) []openai.ChatCompletionMessage {
	var messages []openai.ChatCompletionMessage
	
	for i := len(pairs) - 1; i >= 0; i-- {
		pair := pairs[i]
		
		userContent := string(pair.UserMessage.MessageText)
		if userContent != "" {
			messages = append(messages, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleUser,
				Content: userContent,
			})
		}
		
		if pair.XiResponse != nil {
			xiContent := string(pair.XiResponse.MessageText)
			if xiContent != "" {
				messages = append(messages, openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleAssistant,
					Content: xiContent,
				})
			}
		}
	}
	
	return messages
}

func MessagePairsToDeepseek(pairs []repository.MessagePair) []deepseek.ChatCompletionMessage {
	var messages []deepseek.ChatCompletionMessage
	
	for i := len(pairs) - 1; i >= 0; i-- {
		pair := pairs[i]
		
		userContent := string(pair.UserMessage.MessageText)
		if userContent != "" {
			messages = append(messages, deepseek.ChatCompletionMessage{
				Role:    constants.ChatMessageRoleUser,
				Content: userContent,
			})
		}
		
		if pair.XiResponse != nil {
			xiContent := string(pair.XiResponse.MessageText)
			if xiContent != "" {
				messages = append(messages, deepseek.ChatCompletionMessage{
					Role:    constants.ChatMessageRoleAssistant,
					Content: xiContent,
				})
			}
		}
	}
	
	return messages
}

func MessagePairsToAnthropic(pairs []repository.MessagePair) []anthropic.Message {
	var messages []anthropic.Message
	
	for i := len(pairs) - 1; i >= 0; i-- {
		pair := pairs[i]
		
		userContent := string(pair.UserMessage.MessageText)
		if userContent != "" {
			messages = append(messages, anthropic.NewUserTextMessage(userContent))
		}
		
		if pair.XiResponse != nil {
			xiContent := string(pair.XiResponse.MessageText)
			if xiContent != "" {
				messages = append(messages, anthropic.NewAssistantTextMessage(xiContent))
			}
		}
	}
	
	return messages
}