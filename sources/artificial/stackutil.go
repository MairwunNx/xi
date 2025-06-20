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
		
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: string(pair.UserMessage.MessageText),
		})
		
		if pair.XiResponse != nil {
			messages = append(messages, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleAssistant,
				Content: string(pair.XiResponse.MessageText),
			})
		}
	}
	
	return messages
}

func MessagePairsToDeepseek(pairs []repository.MessagePair) []deepseek.ChatCompletionMessage {
	var messages []deepseek.ChatCompletionMessage
	
	for i := len(pairs) - 1; i >= 0; i-- {
		pair := pairs[i]
		
		messages = append(messages, deepseek.ChatCompletionMessage{
			Role:    constants.ChatMessageRoleUser,
			Content: string(pair.UserMessage.MessageText),
		})
		
		if pair.XiResponse != nil {
			messages = append(messages, deepseek.ChatCompletionMessage{
				Role:    constants.ChatMessageRoleAssistant,
				Content: string(pair.XiResponse.MessageText),
			})
		}
	}
	
	return messages
}

func MessagePairsToAnthropic(pairs []repository.MessagePair) []anthropic.Message {
	var messages []anthropic.Message
	
	for i := len(pairs) - 1; i >= 0; i-- {
		pair := pairs[i]
		
		messages = append(messages, anthropic.NewUserTextMessage(string(pair.UserMessage.MessageText)))
		
		if pair.XiResponse != nil {
			messages = append(messages, anthropic.NewAssistantTextMessage(string(pair.XiResponse.MessageText)))
		}
	}
	
	return messages
}