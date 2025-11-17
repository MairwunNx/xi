package platform

type ChatID int64

type UserGrade = string

const (
	GradeBronze UserGrade = "bronze"
	GradeSilver UserGrade = "silver"
	GradeGold   UserGrade = "gold"
)

type MessageRole = string

const (
	MessageRoleUser      MessageRole = "user"
	MessageRoleAssistant MessageRole = "assistant"
	MessageRoleSystem    MessageRole = "system"
)

type RedisMessage struct {
	Role         MessageRole `json:"role"`
	Content      string      `json:"content"`
	IsCompressed bool        `json:"is_compressed,omitempty"`
}