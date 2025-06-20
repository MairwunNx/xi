package repository

import (
	"context"
	"errors"
	"time"
	"ximanager/sources/persistence/entities"
	"ximanager/sources/persistence/gormdao/query"
	"ximanager/sources/platform"
	"ximanager/sources/tracing"

	"gorm.io/gorm"
)

var (
	ErrModeInUse    = errors.New("mode is currently in use and cannot be deleted")
	ErrModeNotFound = errors.New("mode not found")
	ErrInvalidMode  = errors.New("invalid mode values")
)

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
		IsEnabled: true,
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
		Preload(query.SelectedMode.Mode.Where(query.Mode.IsEnabled.Is(true))).
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
