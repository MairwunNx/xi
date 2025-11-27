package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"ximanager/sources/platform"
	"ximanager/sources/tracing"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	ChatStateNone               = 0
	ChatStateAwaitingModeType   = 1
	ChatStateAwaitingModeName   = 2
	ChatStateAwaitingGrade      = 3
	ChatStateAwaitingPrompt     = 4
	ChatStateAwaitingConfig     = 5
	ChatStateConfirmDelete      = 6
	ChatStateAwaitingNewName    = 7
)

const (
	chatStateTTL = 10 * time.Minute
)

type ChatStateData struct {
	Status    int       `json:"status"`
	UserID    int64     `json:"user_id"`
	ModeID    string    `json:"mode_id,omitempty"`
	ModeType  string    `json:"mode_type,omitempty"`
	ModeName  string    `json:"mode_name,omitempty"`
	ModeGrade string    `json:"mode_grade,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type ChatStateRepository struct {
	redis *redis.Client
	log   *tracing.Logger
}

func NewChatStateRepository(redis *redis.Client, log *tracing.Logger) *ChatStateRepository {
	return &ChatStateRepository{
		redis: redis,
		log:   log,
	}
}

func (r *ChatStateRepository) getChatStateKey(chatID int64, userID int64) string {
	return fmt.Sprintf("chat_state:%d:%d", chatID, userID)
}

func (r *ChatStateRepository) GetState(logger *tracing.Logger, chatID int64, userID int64) (*ChatStateData, error) {
	defer tracing.ProfilePoint(logger, "ChatState get completed", "repository.chatstate.get", "chat_id", chatID, "user_id", userID)()
	
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 5*time.Second)
	defer cancel()

	key := r.getChatStateKey(chatID, userID)
	data, err := r.redis.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		logger.E("Failed to get chat state from Redis", tracing.InnerError, err)
		return nil, err
	}

	var state ChatStateData
	if err := json.Unmarshal([]byte(data), &state); err != nil {
		logger.E("Failed to unmarshal chat state", tracing.InnerError, err)
		return nil, err
	}

	return &state, nil
}

func (r *ChatStateRepository) SetState(logger *tracing.Logger, chatID int64, userID int64, state *ChatStateData) error {
	defer tracing.ProfilePoint(logger, "ChatState set completed", "repository.chatstate.set", "chat_id", chatID, "user_id", userID, "status", state.Status)()
	
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 5*time.Second)
	defer cancel()

	state.CreatedAt = time.Now()
	data, err := json.Marshal(state)
	if err != nil {
		logger.E("Failed to marshal chat state", tracing.InnerError, err)
		return err
	}

	key := r.getChatStateKey(chatID, userID)
	if err := r.redis.Set(ctx, key, data, chatStateTTL).Err(); err != nil {
		logger.E("Failed to set chat state in Redis", tracing.InnerError, err)
		return err
	}

	logger.I("Chat state set successfully", "chat_id", chatID, "user_id", userID, "status", state.Status)
	return nil
}

func (r *ChatStateRepository) ClearState(logger *tracing.Logger, chatID int64, userID int64) error {
	defer tracing.ProfilePoint(logger, "ChatState clear completed", "repository.chatstate.clear", "chat_id", chatID, "user_id", userID)()
	
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 5*time.Second)
	defer cancel()

	key := r.getChatStateKey(chatID, userID)
	if err := r.redis.Del(ctx, key).Err(); err != nil {
		logger.E("Failed to clear chat state from Redis", tracing.InnerError, err)
		return err
	}

	logger.I("Chat state cleared", "chat_id", chatID, "user_id", userID)
	return nil
}

func (r *ChatStateRepository) HasActiveState(logger *tracing.Logger, chatID int64, userID int64) bool {
	state, err := r.GetState(logger, chatID, userID)
	if err != nil {
		return false
	}
	return state != nil && state.Status != ChatStateNone
}

func (r *ChatStateRepository) InitModeCreation(logger *tracing.Logger, chatID int64, userID int64) error {
	state := &ChatStateData{
		Status: ChatStateAwaitingModeType,
		UserID: userID,
	}
	return r.SetState(logger, chatID, userID, state)
}

func (r *ChatStateRepository) SetModeType(logger *tracing.Logger, chatID int64, userID int64, modeType string) error {
	state, err := r.GetState(logger, chatID, userID)
	if err != nil {
		return err
	}
	if state == nil {
		state = &ChatStateData{UserID: userID}
	}
	
	state.ModeType = modeType
	state.Status = ChatStateAwaitingModeName
	return r.SetState(logger, chatID, userID, state)
}

func (r *ChatStateRepository) SetModeName(logger *tracing.Logger, chatID int64, userID int64, modeName string) error {
	state, err := r.GetState(logger, chatID, userID)
	if err != nil {
		return err
	}
	if state == nil {
		return fmt.Errorf("no active state found")
	}
	
	state.ModeName = modeName
	state.Status = ChatStateAwaitingGrade
	return r.SetState(logger, chatID, userID, state)
}

func (r *ChatStateRepository) SetModeGrade(logger *tracing.Logger, chatID int64, userID int64, modeGrade string) error {
	state, err := r.GetState(logger, chatID, userID)
	if err != nil {
		return err
	}
	if state == nil {
		return fmt.Errorf("no active state found")
	}
	
	state.ModeGrade = modeGrade
	state.Status = ChatStateAwaitingPrompt
	return r.SetState(logger, chatID, userID, state)
}

func (r *ChatStateRepository) InitPromptEdit(logger *tracing.Logger, chatID int64, userID int64, modeID uuid.UUID) error {
	state := &ChatStateData{
		Status: ChatStateAwaitingPrompt,
		UserID: userID,
		ModeID: modeID.String(),
	}
	return r.SetState(logger, chatID, userID, state)
}

func (r *ChatStateRepository) InitConfigEdit(logger *tracing.Logger, chatID int64, userID int64, modeID uuid.UUID) error {
	state := &ChatStateData{
		Status: ChatStateAwaitingConfig,
		UserID: userID,
		ModeID: modeID.String(),
	}
	return r.SetState(logger, chatID, userID, state)
}

func (r *ChatStateRepository) InitDeleteConfirmation(logger *tracing.Logger, chatID int64, userID int64, modeID uuid.UUID) error {
	state := &ChatStateData{
		Status: ChatStateConfirmDelete,
		UserID: userID,
		ModeID: modeID.String(),
	}
	return r.SetState(logger, chatID, userID, state)
}

func (r *ChatStateRepository) InitNameEdit(logger *tracing.Logger, chatID int64, userID int64, modeID uuid.UUID) error {
	state := &ChatStateData{
		Status: ChatStateAwaitingNewName,
		UserID: userID,
		ModeID: modeID.String(),
	}
	return r.SetState(logger, chatID, userID, state)
}

func GetStatusName(status int) string {
	switch status {
	case ChatStateNone:
		return "none"
	case ChatStateAwaitingModeType:
		return "awaiting_mode_type"
	case ChatStateAwaitingModeName:
		return "awaiting_mode_name"
	case ChatStateAwaitingGrade:
		return "awaiting_grade"
	case ChatStateAwaitingPrompt:
		return "awaiting_prompt"
	case ChatStateAwaitingConfig:
		return "awaiting_config"
	case ChatStateConfirmDelete:
		return "confirm_delete"
	case ChatStateAwaitingNewName:
		return "awaiting_new_name"
	default:
		return "unknown"
	}
}