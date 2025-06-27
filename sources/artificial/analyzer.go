package artificial

import (
	"context"
	"encoding/json"
	"time"
	"ximanager/sources/platform"
	"ximanager/sources/tracing"
)

type MessageTopic struct {
	Type        string  `json:"type"`
	Temperature float32 `json:"temperature"`
}

type TopicAnalysisResult struct {
	DetectedTopic MessageTopic `json:"detected_topic"`
	Confidence    float32      `json:"confidence"`
}

var MessageTopics = map[string]MessageTopic{
	"coding":           {"Coding / Math", 0.0},
	"data_analysis":    {"Data Cleaning / Data Analysis", 1.0},
	"conversation":     {"General Conversation", 1.3},
	"translation":      {"Translation", 1.3},
	"creative_writing": {"Creative Writing / Poetry", 1.5},
}

type TopicAnalyzer struct {
	openaiClient *OpenAIClient
}

func NewTopicAnalyzer(openaiClient *OpenAIClient) *TopicAnalyzer {
	return &TopicAnalyzer{
		openaiClient: openaiClient,
	}
}

func (ta *TopicAnalyzer) AnalyzeMessageTopic(logger *tracing.Logger, message string, persona string) (*TopicAnalysisResult, error) {
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 30*time.Second)
	defer cancel()

	response, err := ta.openaiClient.ResponseMediumWeight(ctx, logger, prompt, message, persona)
	if err != nil {
		logger.E("Failed to analyze message topic", tracing.InnerError, err)
		return nil, err
	}

	var result TopicAnalysisResult
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		logger.E("Failed to parse topic analysis result", tracing.InnerError, err, "response", response)
		return &TopicAnalysisResult{
			DetectedTopic: MessageTopics["conversation"],
			Confidence:    1.0,
		}, nil
	}

	logger.I("Message topic analyzed", "topic", result.DetectedTopic.Type, "temperature", result.DetectedTopic.Temperature, "confidence", result.Confidence)

	return &result, nil
}

func (ta *TopicAnalyzer) GetTemperatureForTopic(topicType string) float32 {
	if topic, exists := MessageTopics[topicType]; exists {
		return topic.Temperature
	}
	return MessageTopics["conversation"].Temperature
}

const prompt = `Проанализируй следующее сообщение пользователя и определи к какой из 5 категорий оно относится.

КАТЕГОРИИ И ТИПЫ:
1. "coding" - Программирование, математика, вычисления, алгоритмы, код, технические задачи
2. "data_analysis" - Анализ данных, статистика, обработка информации, исследования  
3. "conversation" - Обычные разговоры, вопросы о жизни, обсуждения, социальные темы
4. "translation" - Переводы текстов, языковые вопросы, изучение языков
5. "creative_writing" - Творческое письмо, поэзия, истории, креативные задачи

ВАЖНО: Верни ответ ТОЛЬКО в формате JSON:
{
  "detected_topic": {
    "type": "один_из_пяти_типов_выше", 
    "temperature": соответствующая_температура
  },
  "confidence": уверенность_от_0_до_1
}

СООТВЕТСТВИЕ ТЕМПЕРАТУР:
- coding: 0.0
- data_analysis: 1.0  
- conversation: 1.3
- translation: 1.3
- creative_writing: 1.5`