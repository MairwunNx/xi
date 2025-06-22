package repository

import (
	"context"
	"errors"
	"strings"
	"time"
	"ximanager/sources/persistence/entities"
	"ximanager/sources/persistence/gormdao/query"
	"ximanager/sources/platform"
	"ximanager/sources/tracing"

	"gorm.io/gorm"
)

var (
	ErrPinNotFound          = errors.New("pin not found")
	ErrPinLimitExceeded     = errors.New("pin limit exceeded")
	ErrPinLimitExceededChat = errors.New("pin limit exceeded for chat")
	ErrPinTooLong           = errors.New("pin message too long")
)

type PinsRepository struct{
	config *PinsConfig
}

func NewPinsRepository(config *PinsConfig) *PinsRepository {
	return &PinsRepository{config: config}
}

func (x *PinsRepository) CreatePin(logger *tracing.Logger, user *entities.User, chatID int64, message string) (*entities.Pin, error) {
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	message = strings.TrimSpace(strings.ToValidUTF8(message, ""))

	if len(message) > 1024 {
		return nil, ErrPinTooLong
	}

	q := query.Q.WithContext(ctx)
	pin := &entities.Pin{ChatID: chatID, UserID: user.ID, Message: message}

	err := q.Pin.Create(pin)
	if err != nil {
		logger.E("Failed to create pin", tracing.InnerError, err)
		return nil, err
	}

	logger.I("Created pin")
	return pin, nil
}

func (x *PinsRepository) GetPinsByChat(logger *tracing.Logger, chatID int64) ([]*entities.Pin, error) {
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Q.WithContext(ctx)
	pins, err := q.Pin.Where(query.Pin.ChatID.Eq(chatID)).Preload(query.Pin.User).Order(query.Pin.CreatedAt.Desc()).Find()
	if err != nil {
		logger.E("Failed to get pins by chat", tracing.InnerError, err)
		return nil, err
	}

	logger.I("Pins fetched by chat")
	return pins, nil
}

func (x *PinsRepository) GetPinByUserChatAndMessage(logger *tracing.Logger, user *entities.User, chatID int64, message string) (*entities.Pin, error) {
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Q.WithContext(ctx)
	pin, err := q.Pin.Where(query.Pin.ChatID.Eq(chatID), query.Pin.UserID.Eq(user.ID), query.Pin.Message.Eq(message)).First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			logger.W("Pin not found when expected")
			return nil, ErrPinNotFound
		} else {
			logger.E("Failed to get pin by user, chat and message", tracing.InnerError, err)
			return nil, err
		}
	}

	logger.I("Pin fetched by user, chat and message")
	return pin, nil
}

func (x *PinsRepository) DeletePin(logger *tracing.Logger, user *entities.User, chatID int64, message string) error {
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	message = strings.TrimSpace(strings.ToValidUTF8(message, ""))

	q := query.Q.WithContext(ctx)
	_, err := q.Pin.Where(query.Pin.ChatID.Eq(chatID), query.Pin.UserID.Eq(user.ID), query.Pin.Message.Eq(message)).Delete(&entities.Pin{})
	if err != nil {
		logger.E("Failed to delete pin", tracing.InnerError, err)
		return err
	}

	logger.I("Pin deleted")
	return nil
}

func (x *PinsRepository) CountPinsByUserInChat(logger *tracing.Logger, user *entities.User, chatID int64) (int64, error) {
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Q.WithContext(ctx)
	count, err := q.Pin.Where(query.Pin.ChatID.Eq(chatID), query.Pin.UserID.Eq(user.ID)).Count()
	if err != nil {
		logger.E("Failed to count pins by user in chat", tracing.InnerError, err)
		return 0, err
	}

	return count, nil
}

func (x *PinsRepository) GetPinLimit(logger *tracing.Logger, user *entities.User, donations []*entities.Donation) int {
	if len(donations) > 0 {
		return x.config.PinsLimitDonated
	}
	return x.config.PinsLimitDefault
}

func (x *PinsRepository) CountPinsByChat(logger *tracing.Logger, chatID int64) (int64, error) {
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Q.WithContext(ctx)

	count, err := q.Pin.Where(query.Pin.ChatID.Eq(chatID)).Count()
	if err != nil {
		logger.E("Failed to count pins by chat", tracing.InnerError, err)
		return 0, err
	}

	return count, nil
}

func (x *PinsRepository) CheckPinLimit(logger *tracing.Logger, user *entities.User, chatID int64, donations []*entities.Donation) error {
	chatPinsCount, err := x.CountPinsByChat(logger, chatID)
	if err != nil {
		return err
	}

	if chatPinsCount >= int64(x.config.PinsLimitChat) {
		return ErrPinLimitExceededChat
	}

	userPinsCount, err := x.CountPinsByUserInChat(logger, user, chatID)
	if err != nil {
		return err
	}

	userLimit := x.GetPinLimit(logger, user, donations)
	if userPinsCount >= int64(userLimit) {
		return ErrPinLimitExceeded
	}

	return nil
}
