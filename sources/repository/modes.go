package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"
	"ximanager/sources/persistence/entities"
	"ximanager/sources/persistence/gormdao/query"
	"ximanager/sources/platform"
	"ximanager/sources/tracing"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrModeInUse    = errors.New("mode is currently in use and cannot be deleted")
	ErrModeNotFound = errors.New("mode not found")
	ErrInvalidMode  = errors.New("invalid mode values")
)

type ModeConfig struct {
	Prompt string `json:"prompt"`
	Params *AIParams `json:"params,omitempty"`
	Final  bool   `json:"final,omitempty"`
}

type AIParams struct {
	// Nucleus sampling (0.1-1.0) - меньшие значения = более фокусированные ответы
	TopP *float32 `json:"top_p,omitempty"`
	
	// Top-K sampling (1-100) - количество наиболее вероятных токенов (специфично для Anthropic)
	TopK *int `json:"top_k,omitempty"`

	// Штраф за повторение тем (0.0-2.0) - не поддерживается Anthropic
	PresencePenalty *float32 `json:"presence_penalty,omitempty"`
	
	// Штраф за повторение слов (0.0-2.0) - не поддерживается Anthropic  
	FrequencyPenalty *float32 `json:"frequency_penalty,omitempty"`
	
	// Температура (0-2) - управляет случайностью ответов
	Temperature *float32 `json:"temperature,omitempty"`
}

func DefaultModeConfig(prompt string) *ModeConfig {
	return &ModeConfig{
		Prompt: prompt,
		Final:  false,
		Params: &AIParams{
			TopP:             nil,
			TopK:             nil,
			PresencePenalty:  nil,
			FrequencyPenalty: nil,
			Temperature:      nil,
		},
	}
}

type ModesRepository struct {
	users *UsersRepository
}

func NewModesRepository(users *UsersRepository) *ModesRepository {
	return &ModesRepository{users: users}
}

func (x *ModesRepository) SwitchMode(logger *tracing.Logger, chatID int64, userID int64) (*entities.Mode, error) {
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Q.WithContext(ctx)

	currentMode, err := x.GetModeByChat(logger, chatID)
	if err != nil {
		logger.E("Error getting current mode for switch", tracing.InnerError, err)
		return nil, err
	}

	availableModes, err := x.GetModesByChat(logger, chatID)
	if err != nil {
		logger.E("Error getting available modes for switch", tracing.InnerError, err)
		return nil, err
	}

	if len(availableModes) <= 1 {
		logger.W("Not enough modes to switch", "count", len(availableModes))
		return nil, errors.New("недостаточно режимов для переключения")
	}

	var nextMode *entities.Mode
	for i, mode := range availableModes {
		if mode.ID == currentMode.ID {
			nextIndex := (i + 1) % len(availableModes)
			nextMode = availableModes[nextIndex]
			break
		}
	}

	if nextMode == nil {
		logger.E("Current mode not found in available modes list")
		return nil, errors.New("текущий режим не найден в списке доступных режимов")
	}

	user, err := x.users.GetUserByEid(logger, userID)
	if err != nil {
		logger.E("Error getting user for mode switch", tracing.InnerError, err)
		return nil, err
	}

	selectedMode := &entities.SelectedMode{
		ChatID:     chatID,
		ModeID:     nextMode.ID,
		SwitchedBy: user.ID,
	}

	err = q.SelectedMode.Create(selectedMode)
	if err != nil {
		logger.E("Error creating selected mode record", tracing.InnerError, err)
		return nil, err
	}

	logger.I("Mode switched successfully", tracing.ModeId, nextMode.ID, tracing.ModeName, nextMode.Name)
	return nextMode, nil
}

func (x *ModesRepository) UpdateMode(logger *tracing.Logger, mode *entities.Mode) (*entities.Mode, error) {
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Q.WithContext(ctx)

	_, err := q.Mode.Where(query.Mode.ID.Eq(mode.ID)).Updates(mode)
	if err != nil {
		if errors.Is(err, gorm.ErrCheckConstraintViolated) {
			logger.E("New mode values are invalid")
			return nil, ErrInvalidMode
		}

		logger.E("Failed to update mode", tracing.InnerError, err)
		return nil, err
	}

	logger.I("Updated mode")
	return mode, nil
}

func (x *ModesRepository) MustUpdateMode(logger *tracing.Logger, mode *entities.Mode) *entities.Mode {
	mode, err := x.UpdateMode(logger, mode)
	if err != nil {
		logger.F("Got error while not expected", tracing.InnerError, err)
	}

	return mode
}

func (x *ModesRepository) GetModesByChat(logger *tracing.Logger, cid int64) ([]*entities.Mode, error) {
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Q.WithContext(ctx)

	modes, err := q.Mode.Where(
		query.Mode.ChatID.In(cid, 0),
		query.Mode.IsEnabled.Is(true),
	).Order(query.Mode.CreatedAt.Desc()).Find()

	if err != nil {
		logger.E("Failed to get modes", tracing.InnerError, err)
		return nil, err
	}

	logger.I("Retrieved modes")
	return modes, nil
}

func (x *ModesRepository) AddModeForChat(logger *tracing.Logger, cid int64, modeType string, name string, prompt string, euid int64) (*entities.Mode, error) {
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Q.WithContext(ctx)

	user, err := x.users.GetUserByEid(logger, euid)
	if err != nil {
		return nil, err
	}

	newMode := &entities.Mode{
		ChatID:    cid,
		Type:      modeType,
		Name:      name,
		Prompt:    prompt,
		Final:     platform.BoolPtr(false),
		IsEnabled: platform.BoolPtr(true),
		CreatedBy: &user.ID,
	}

	err = q.Mode.Create(newMode)
	if err != nil {
		logger.E("Failed to create mode", tracing.InnerError, err)
		return nil, err
	}

	logger.I("Created new mode", tracing.ModeId, newMode.ID, tracing.ModeName, newMode.Name, tracing.ChatId, cid)
	return newMode, nil
}

func (r *ModesRepository) GetModeByChat(logger *tracing.Logger, cid int64) (*entities.Mode, error) {
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Q.WithContext(ctx)

	cmode, err := q.SelectedMode.
		Where(query.SelectedMode.ChatID.Eq(cid)).
		Order(query.SelectedMode.SwitchedAt.Desc()).
		Preload(query.SelectedMode.Mode.Where(
			query.Mode.IsEnabled.Is(true),
			query.Mode.ChatID.In(cid, 0),
		)).
		First()

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.E("No selected mode, falling back to default")
		} else {
			logger.E("Failed to get selected mode, falling back to default", tracing.InnerError, err)
		}
	} else {
		logger.I("Gathered selected mode", tracing.ModeId, cmode.Mode.ID, tracing.ModeName, cmode.Mode.Name)
		return &cmode.Mode, nil
	}

	dmode, err := r.GetDefaultMode(logger)
	if err != nil {
		return nil, err
	}

	return dmode, nil
}

func (r *ModesRepository) GetDefaultMode(logger *tracing.Logger) (*entities.Mode, error) {
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Q.WithContext(ctx)

	mode, err := q.Mode.Where(
		query.Mode.ChatID.Eq(0),
		query.Mode.IsEnabled.Is(true),
	).Order(query.Mode.CreatedAt.Desc()).First()

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.E("No default mode")
			return nil, errors.New("no default mode")
		} else {
			logger.E("Failed to get default mode", tracing.InnerError, err)
			return nil, err
		}
	}

	logger.I("Gathered default mode", tracing.ModeId, mode.ID, tracing.ModeName, mode.Name)
	return mode, nil
}

func (r *ModesRepository) DeleteMode(logger *tracing.Logger, mode *entities.Mode) error {
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Q.WithContext(ctx)

	_, err := q.Mode.Where(query.Mode.ID.Eq(mode.ID)).Delete(&entities.Mode{})
	if err != nil {
		logger.E("Failed to delete mode", tracing.InnerError, err)
		return err
	}

	logger.I("Deleted mode", tracing.ModeId, mode.ID, tracing.ModeName, mode.Name)
	return nil
}

func (r *ModesRepository) MustDeleteMode(logger *tracing.Logger, mode *entities.Mode) {
	err := r.DeleteMode(logger, mode)
	if err != nil {
		logger.F("Got error while not expected", tracing.InnerError, err)
	}
}

func (x *ModesRepository) ParseModeConfig(mode *entities.Mode, logger *tracing.Logger) *ModeConfig {
	if mode.Config != nil && *mode.Config != "" {
		var config ModeConfig
		if err := json.Unmarshal([]byte(*mode.Config), &config); err != nil {
			logger.E("Failed to parse mode config JSON, falling back to prompt", "mode_id", mode.ID, tracing.InnerError, err)
			// Fallback на старый формат
			return DefaultModeConfig(mode.Prompt)
		}
		return &config
	}
	
	// Fallback на старый формат, если нет JSON конфигурации
	logger.I("Using fallback mode config", "mode_id", mode.ID)
	return DefaultModeConfig(mode.Prompt)
}

func (x *ModesRepository) SerializeModeConfig(config *ModeConfig) (string, error) {
	data, err := json.Marshal(config)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (x *ModesRepository) GetModeConfigByChat(logger *tracing.Logger, chatID int64) (*ModeConfig, error) {
	mode, err := x.GetModeByChat(logger, chatID)
	if err != nil {
		return nil, err
	}
	if mode == nil {
		return nil, nil
	}
	
	return x.ParseModeConfig(mode, logger), nil
}

func (x *ModesRepository) UpdateModeConfig(logger *tracing.Logger, modeID uuid.UUID, config *ModeConfig) error {
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 30*time.Second)
	defer cancel()

	configJSON, err := x.SerializeModeConfig(config)
	if err != nil {
		logger.E("Failed to serialize mode config", tracing.InnerError, err)
		return err
	}

	q := query.Q.WithContext(ctx)
	_, err = q.Mode.Where(query.Mode.ID.Eq(modeID)).Updates(map[string]interface{}{"prompt": config.Prompt, "config": configJSON, "final":  config.Final})
	if err != nil {
		logger.E("Failed to update mode config", tracing.InnerError, err)
		return err
	}

	logger.I("Mode config updated", "mode_id", modeID)
	return nil
}

func (x *ModesRepository) GetAISettingsForMode(config *ModeConfig, globalSettings *AIParams) *AIParams {
	if config.Params == nil {
		return globalSettings
	}
	
	// Создаем копию глобальных настроек
	result := &AIParams{}
	if globalSettings != nil {
		*result = *globalSettings
	}
	
	// Переопределяем только те значения, которые заданы в конфигурации режима
	if config.Params.TopP != nil {
		result.TopP = config.Params.TopP
	}
	if config.Params.PresencePenalty != nil {
		result.PresencePenalty = config.Params.PresencePenalty
	}
	if config.Params.FrequencyPenalty != nil {
		result.FrequencyPenalty = config.Params.FrequencyPenalty
	}
	if config.Params.Temperature != nil {
		result.Temperature = config.Params.Temperature
	}
	
	return result
}

func (x *ModesRepository) AddModeWithConfig(logger *tracing.Logger, cid int64, modeType string, name string, config *ModeConfig, euid int64) (*entities.Mode, error) {
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Q.WithContext(ctx)

	user, err := x.users.GetUserByEid(logger, euid)
	if err != nil {
		return nil, err
	}

	configJSON, err := x.SerializeModeConfig(config)
	if err != nil {
		logger.E("Failed to serialize mode config", tracing.InnerError, err)
		return nil, err
	}

	newMode := &entities.Mode{
		ChatID:    cid,
		Type:      modeType,
		Name:      name,
		Prompt:    config.Prompt,
		Config:    &configJSON,
		Final:     platform.BoolPtr(config.Final),
		IsEnabled: platform.BoolPtr(true),
		CreatedBy: &user.ID,
	}

	err = q.Mode.Create(newMode)
	if err != nil {
		logger.E("Failed to create mode", tracing.InnerError, err)
		return nil, err
	}

	logger.I("Created new mode with config", tracing.ModeId, newMode.ID, tracing.ModeName, newMode.Name, tracing.ChatId, cid, "final", platform.BoolValue(newMode.Final, false))
	return newMode, nil
}
