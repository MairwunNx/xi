package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"ximanager/sources/persistence/entities"
	"ximanager/sources/persistence/gormdao/query"
	"ximanager/sources/platform"
	"ximanager/sources/tracing"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	fallbackPrompt = "Ты - Xi Manager, представитель великого Xi. Следуешь китайским традициям. Отвечай на вопросы пользователя и сопровождай эмодзи. Но на всякий предупреди пользователя, что сейчас наблюдаются технические проблемы с Xi Manager ботом (это кстати ты), и время ответа может быть увеличено, а качество ответов снижено."
)

var (
	ErrModeInUse    = errors.New("mode is currently in use and cannot be deleted")
	ErrModeNotFound = errors.New("mode not found")
	ErrInvalidMode  = errors.New("invalid mode values")
)

type ModeConfig struct {
	Prompt string    `json:"prompt"`
	Params *AIParams `json:"params,omitempty"`
	Final  bool      `json:"final,omitempty"`
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
	if strings.TrimSpace(prompt) == "" {
		prompt = fallbackPrompt
	}

	return &ModeConfig{
		Prompt: prompt,
		Final:  true,
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
	users     *UsersRepository
	donations *DonationsRepository
}

func NewModesRepository(users *UsersRepository, donations *DonationsRepository) *ModesRepository {
	return &ModesRepository{users: users, donations: donations}
}

func (x *ModesRepository) GetAllModes(logger *tracing.Logger) ([]*entities.Mode, error) {
	defer tracing.ProfilePoint(logger, "Modes get all completed", "repository.modes.get.all")()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Q.WithContext(ctx)

	// Get all modes, then filter to get latest by type
	modes, err := q.Mode.
		Where(query.Mode.IsEnabled.Is(true)).
		Order(query.Mode.CreatedAt.Desc()).
		Find()

	if err != nil {
		logger.E("Failed to get all modes", tracing.InnerError, err)
		return nil, err
	}

	// Distinct by type - keep only the latest version for each type
	modesByType := make(map[string]*entities.Mode)
	for _, mode := range modes {
		if _, exists := modesByType[mode.Type]; !exists {
			modesByType[mode.Type] = mode
		}
	}

	result := make([]*entities.Mode, 0, len(modesByType))
	for _, mode := range modesByType {
		result = append(result, mode)
	}

	logger.I("Retrieved all modes", "count", len(result))
	return result, nil
}

func (x *ModesRepository) GetAllModesIncludingDisabled(logger *tracing.Logger) ([]*entities.Mode, error) {
	defer tracing.ProfilePoint(logger, "Modes get all including disabled completed", "repository.modes.get.all.including.disabled")()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Q.WithContext(ctx)

	modes, err := q.Mode.
		Order(query.Mode.CreatedAt.Desc()).
		Find()

	if err != nil {
		logger.E("Failed to get all modes", tracing.InnerError, err)
		return nil, err
	}

	// Distinct by type - keep only the latest version for each type
	modesByType := make(map[string]*entities.Mode)
	for _, mode := range modes {
		if _, exists := modesByType[mode.Type]; !exists {
			modesByType[mode.Type] = mode
		}
	}

	result := make([]*entities.Mode, 0, len(modesByType))
	for _, mode := range modesByType {
		result = append(result, mode)
	}

	logger.I("Retrieved all modes including disabled", "count", len(result))
	return result, nil
}

func (x *ModesRepository) GetModesForUser(logger *tracing.Logger, user *entities.User) ([]*entities.Mode, error) {
	defer tracing.ProfilePoint(logger, "Modes get for user completed", "repository.modes.get.for.user", "user_id", user.ID)()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	userGrade, err := x.donations.GetUserGrade(logger, user)
	if err != nil {
		logger.E("Failed to get user grade", tracing.InnerError, err)
		return nil, err
	}

	allowedGrades := x.getAllowedGrades(userGrade)

	q := query.Q.WithContext(ctx)

	modes, err := q.Mode.Where(
		query.Mode.IsEnabled.Is(true),
		q.Mode.Or(query.Mode.Grade.In(allowedGrades...), query.Mode.Grade.IsNull()),
	).Order(query.Mode.CreatedAt.Desc()).Find()

	if err != nil {
		logger.E("Failed to get modes for user", tracing.InnerError, err)
		return nil, err
	}

	// Distinct by type - keep only the latest version for each type
	modesByType := make(map[string]*entities.Mode)
	for _, mode := range modes {
		if _, exists := modesByType[mode.Type]; !exists {
			modesByType[mode.Type] = mode
		}
	}

	result := make([]*entities.Mode, 0, len(modesByType))
	for _, mode := range modesByType {
		result = append(result, mode)
	}

	logger.I("Retrieved modes for user", "count", len(result), "user_grade", userGrade)
	return result, nil
}

func (x *ModesRepository) GetAllModesWithAvailability(logger *tracing.Logger, user *entities.User) (available []*entities.Mode, unavailable []*entities.Mode, userGrade platform.UserGrade, err error) {
	defer tracing.ProfilePoint(logger, "Modes get with availability completed", "repository.modes.get.with.availability", "user_id", user.ID)()

	userGrade, err = x.donations.GetUserGrade(logger, user)
	if err != nil {
		logger.E("Failed to get user grade", tracing.InnerError, err)
		return nil, nil, "", err
	}

	allModes, err := x.GetAllModes(logger)
	if err != nil {
		return nil, nil, "", err
	}

	allowedGrades := x.getAllowedGrades(userGrade)

	for _, mode := range allModes {
		if x.isModeAvailableForGrades(mode, allowedGrades) {
			available = append(available, mode)
		} else {
			unavailable = append(unavailable, mode)
		}
	}

	logger.I("Got modes with availability", "available", len(available), "unavailable", len(unavailable), "user_grade", userGrade)
	return available, unavailable, userGrade, nil
}

func (x *ModesRepository) getAllowedGrades(userGrade platform.UserGrade) []string {
	allowedGrades := []string{}
	switch userGrade {
	case platform.GradeGold:
		allowedGrades = []string{platform.GradeBronze, platform.GradeSilver, platform.GradeGold}
	case platform.GradeSilver:
		allowedGrades = []string{platform.GradeBronze, platform.GradeSilver}
	case platform.GradeBronze:
		allowedGrades = []string{platform.GradeBronze}
	default:
		allowedGrades = []string{platform.GradeBronze}
	}
	allowedGrades = append(allowedGrades, "")
	return allowedGrades
}

func (x *ModesRepository) isModeAvailableForGrades(mode *entities.Mode, allowedGrades []string) bool {
	if mode.Grade == nil || *mode.Grade == "" {
		return true
	}
	for _, g := range allowedGrades {
		if *mode.Grade == g {
			return true
		}
	}
	return false
}

func (x *ModesRepository) GetModeByType(logger *tracing.Logger, modeType string) (*entities.Mode, error) {
	return x.getModeByType(logger, modeType, true)
}

func (x *ModesRepository) GetModeByTypeIncludingDisabled(logger *tracing.Logger, modeType string) (*entities.Mode, error) {
	return x.getModeByType(logger, modeType, false)
}

func (x *ModesRepository) getModeByType(logger *tracing.Logger, modeType string, enabledOnly bool) (*entities.Mode, error) {
	defer tracing.ProfilePoint(logger, "Modes get by type completed", "repository.modes.get.by.type", "mode_type", modeType, "enabled_only", enabledOnly)()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Q.WithContext(ctx)

	modeQuery := q.Mode.Where(query.Mode.Type.Eq(modeType))
	if enabledOnly {
		modeQuery = modeQuery.Where(query.Mode.IsEnabled.Is(true))
	}

	mode, err := modeQuery.Order(query.Mode.CreatedAt.Desc()).First()

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrModeNotFound
		}
		logger.E("Failed to get mode by type", tracing.InnerError, err)
		return nil, err
	}

	logger.I("Got mode by type", "mode_id", mode.ID, "mode_name", mode.Name)
	return mode, nil
}

func (x *ModesRepository) GetModeByID(logger *tracing.Logger, modeID uuid.UUID) (*entities.Mode, error) {
	defer tracing.ProfilePoint(logger, "Modes get by id completed", "repository.modes.get.by.id", "mode_id", modeID)()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Q.WithContext(ctx)

	mode, err := q.Mode.
		Where(query.Mode.ID.Eq(modeID)).
		First()

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrModeNotFound
		}
		logger.E("Failed to get mode by id", tracing.InnerError, err)
		return nil, err
	}

	logger.I("Got mode by id", "mode_id", mode.ID, "mode_name", mode.Name)
	return mode, nil
}

func (x *ModesRepository) SetModeForChat(logger *tracing.Logger, chatID int64, modeID uuid.UUID, userID uuid.UUID) error {
	defer tracing.ProfilePoint(logger, "Modes set for chat completed", "repository.modes.set.for.chat", "chat_id", chatID, "mode_id", modeID)()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Q.WithContext(ctx)

	selectedMode := &entities.SelectedMode{
		ChatID:     chatID,
		ModeID:     modeID,
		SwitchedBy: userID,
	}

	err := q.SelectedMode.Create(selectedMode)
	if err != nil {
		logger.E("Error creating selected mode record", tracing.InnerError, err)
		return err
	}

	logger.I("Mode set for chat successfully", "chat_id", chatID, "mode_id", modeID)
	return nil
}

func (x *ModesRepository) GetCurrentModeForChat(logger *tracing.Logger, chatID int64) (*entities.Mode, error) {
	defer tracing.ProfilePoint(logger, "Modes get current for chat completed", "repository.modes.get.current.for.chat", "chat_id", chatID)()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Q.WithContext(ctx)

	selectedMode, err := q.SelectedMode.
		Where(query.SelectedMode.ChatID.Eq(chatID)).
		Order(query.SelectedMode.SwitchedAt.Desc()).
		First()

	if err == nil {
		mode, err := q.Mode.
			Where(
				query.Mode.ID.Eq(selectedMode.ModeID),
				query.Mode.IsEnabled.Is(true),
			).
			First()

		if err == nil {
			logger.I("Gathered selected mode", tracing.ModeId, mode.ID, tracing.ModeName, mode.Name)
			return mode, nil
		} else {
			logger.W("Selected mode not found or disabled, falling back to default",
				"selected_mode_id", selectedMode.ModeID, tracing.InnerError, err)
		}
	} else {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.I("No selected mode for chat, falling back to default")
		} else {
			logger.E("Failed to get selected mode, falling back to default", tracing.InnerError, err)
		}
	}

	// Fallback: get any enabled mode
	mode, err := q.Mode.
		Where(query.Mode.IsEnabled.Is(true)).
		Order(query.Mode.CreatedAt.Desc()).
		First()

	if err == nil {
		logger.I("Using fallback mode", tracing.ModeId, mode.ID, tracing.ModeName, mode.Name)
		return mode, nil
	}

	// Last fallback: default mode
	dmode, err := x.GetDefaultMode(logger)
	if err != nil {
		return nil, err
	}

	return dmode, nil
}

func (x *ModesRepository) GetDefaultMode(logger *tracing.Logger) (*entities.Mode, error) {
	defer tracing.ProfilePoint(logger, "Modes get default mode completed", "repository.modes.get.default.mode")()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Q.WithContext(ctx)

	mode, err := q.Mode.Where(
		query.Mode.IsEnabled.Is(true),
	).Order(query.Mode.CreatedAt.Asc()).First()

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

func (x *ModesRepository) CreateMode(logger *tracing.Logger, modeType string, name string, config *ModeConfig, grade string, creatorEUID int64) (*entities.Mode, error) {
	defer tracing.ProfilePoint(logger, "Modes create completed", "repository.modes.create", "mode_type", modeType, "name", name)()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Q.WithContext(ctx)

	user, err := x.users.GetUserByEid(logger, creatorEUID)
	if err != nil {
		return nil, err
	}

	configJSON, err := x.SerializeModeConfig(config)
	if err != nil {
		logger.E("Failed to serialize mode config", tracing.InnerError, err)
		return nil, err
	}

	var gradePtr *string
	if grade != "" {
		gradePtr = &grade
	}

	newMode := &entities.Mode{
		Type:      modeType,
		Name:      name,
		Config:    &configJSON,
		Grade:     gradePtr,
		Final:     platform.BoolPtr(config.Final),
		IsEnabled: platform.BoolPtr(true),
		CreatedBy: &user.ID,
	}

	err = q.Mode.Create(newMode)
	if err != nil {
		logger.E("Failed to create mode", tracing.InnerError, err)
		return nil, err
	}

	logger.I("Created new mode", tracing.ModeId, newMode.ID, tracing.ModeName, newMode.Name, "grade", grade)
	return newMode, nil
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

func (x *ModesRepository) DeleteMode(logger *tracing.Logger, mode *entities.Mode) error {
	defer tracing.ProfilePoint(logger, "Modes delete mode completed", "repository.modes.delete.mode", "mode_id", mode.ID, "mode_name", mode.Name)()
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

func (x *ModesRepository) ParseModeConfig(mode *entities.Mode, logger *tracing.Logger) *ModeConfig {
	defer tracing.ProfilePoint(logger, "Modes parse mode config completed", "repository.modes.parse.mode.config", "mode_id", mode.ID)()
	if mode.Config != nil && *mode.Config != "" {
		var config ModeConfig
		if err := json.Unmarshal([]byte(*mode.Config), &config); err != nil {
			logger.E("Failed to parse mode config JSON, using fallback", "mode_id", mode.ID, tracing.InnerError, err)
			return DefaultModeConfig("")
		}

		if strings.TrimSpace(config.Prompt) == "" {
			logger.E("Empty prompt in mode config JSON, using default prompt", "mode_id", mode.ID)
			config.Prompt = fallbackPrompt
		}

		return &config
	}

	logger.I("Using fallback mode config", "mode_id", mode.ID)
	return DefaultModeConfig("")
}

func (x *ModesRepository) SerializeModeConfig(config *ModeConfig) (string, error) {
	data, err := json.Marshal(config)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (x *ModesRepository) GetModeConfigForChat(logger *tracing.Logger, chatID int64) (*ModeConfig, error) {
	defer tracing.ProfilePoint(logger, "Modes get mode config for chat completed", "repository.modes.get.mode.config.for.chat", "chat_id", chatID)()
	mode, err := x.GetCurrentModeForChat(logger, chatID)
	if err != nil {
		return nil, err
	}
	if mode == nil {
		return nil, nil
	}

	return x.ParseModeConfig(mode, logger), nil
}

func (x *ModesRepository) UpdateModeConfig(logger *tracing.Logger, modeID uuid.UUID, config *ModeConfig) error {
	defer tracing.ProfilePoint(logger, "Modes update mode config completed", "repository.modes.update.mode.config", "mode_id", modeID)()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 30*time.Second)
	defer cancel()

	configJSON, err := x.SerializeModeConfig(config)
	if err != nil {
		logger.E("Failed to serialize mode config", tracing.InnerError, err)
		return err
	}

	q := query.Q.WithContext(ctx)
	_, err = q.Mode.Where(query.Mode.ID.Eq(modeID)).Updates(map[string]interface{}{"config": configJSON, "final": config.Final})
	if err != nil {
		logger.E("Failed to update mode config", tracing.InnerError, err)
		return err
	}

	logger.I("Mode config updated", "mode_id", modeID)
	return nil
}

func (x *ModesRepository) UpdateModePrompt(logger *tracing.Logger, modeID uuid.UUID, prompt string) error {
	defer tracing.ProfilePoint(logger, "Modes update mode prompt completed", "repository.modes.update.mode.prompt", "mode_id", modeID)()

	mode, err := x.GetModeByID(logger, modeID)
	if err != nil {
		return err
	}

	config := x.ParseModeConfig(mode, logger)
	config.Prompt = prompt

	return x.UpdateModeConfig(logger, modeID, config)
}

// GetAISettingsForMode merges mode-specific settings with global settings
func (x *ModesRepository) GetAISettingsForMode(config *ModeConfig, globalSettings *AIParams) *AIParams {
	if config.Params == nil {
		return globalSettings
	}

	result := &AIParams{}
	if globalSettings != nil {
		*result = *globalSettings
	}

	if config.Params.TopP != nil {
		result.TopP = config.Params.TopP
	}
	if config.Params.TopK != nil {
		result.TopK = config.Params.TopK
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

func (x *ModesRepository) UpdateModeName(logger *tracing.Logger, modeID uuid.UUID, name string) error {
	defer tracing.ProfilePoint(logger, "Modes update mode name completed", "repository.modes.update.mode.name", "mode_id", modeID)()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Q.WithContext(ctx)
	_, err := q.Mode.Where(query.Mode.ID.Eq(modeID)).Updates(map[string]interface{}{"name": name})
	if err != nil {
		logger.E("Failed to update mode name", tracing.InnerError, err)
		return err
	}

	logger.I("Mode name updated", "mode_id", modeID, "new_name", name)
	return nil
}
