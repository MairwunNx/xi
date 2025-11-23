package repository

import (
	"context"
	"time"
	"ximanager/sources/persistence/entities"
	"ximanager/sources/persistence/gormdao/query"
	"ximanager/sources/platform"
	"ximanager/sources/tracing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type MessagesRepository struct {
	users *UsersRepository
}

func NewMessagesRepository(users *UsersRepository) *MessagesRepository {
	return &MessagesRepository{
		users: users,
	}
}

func (x *MessagesRepository) SaveMessage(logger *tracing.Logger, msg *tgbotapi.Message, isXiResponse bool) error {
	defer tracing.ProfilePoint(logger, "Messages save message completed", "repository.messages.save.message", "chat_id", msg.Chat.ID, "user_id", msg.From.ID)()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	user, err := x.users.GetUserByEid(logger, msg.From.ID)
	if err != nil {
		logger.E("Failed to get user", tracing.InnerError, err)
		return err
	}

	message := &entities.Message{
		ChatID:       msg.Chat.ID,
		IsXiResponse: isXiResponse,
		UserID:       &user.ID,
	}

	q := query.Q.WithContext(ctx)
	err = q.Message.Create(message)
	if err != nil {
		logger.E("Failed to save message", tracing.InnerError, err)
		return err
	}

	logger.I("Message saved")

	return nil
}

func (x *MessagesRepository) GetTotalUserQuestionsCount(logger *tracing.Logger) (int64, error) {
	defer tracing.ProfilePoint(logger, "Messages get total user questions count completed", "repository.messages.get.total.user.questions.count")()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Message.WithContext(ctx)
	count, err := q.
		Where(query.Message.IsXiResponse.Is(false)).
		Count()

	if err != nil {
		logger.E("Failed to count total user questions", tracing.InnerError, err)
		return 0, err
	}

	return count, nil
}

func (x *MessagesRepository) GetUserQuestionsInChatCount(logger *tracing.Logger, chatID int64) (int64, error) {
	defer tracing.ProfilePoint(logger, "Messages get user questions in chat count completed", "repository.messages.get.user.questions.in.chat.count", "chat_id", chatID)()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Message.WithContext(ctx)
	count, err := q.
		Where(query.Message.IsXiResponse.Is(false)).
		Where(query.Message.ChatID.Eq(chatID)).
		Count()

	if err != nil {
		logger.E("Failed to count user questions in chat", tracing.InnerError, err)
		return 0, err
	}

	return count, nil
}

func (x *MessagesRepository) GetUserPersonalQuestionsCount(logger *tracing.Logger, user *entities.User) (int64, error) {
	defer tracing.ProfilePoint(logger, "Messages get user personal questions count completed", "repository.messages.get.user.personal.questions.count", "user_id", user.ID)()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Message.WithContext(ctx)
	count, err := q.
		Where(query.Message.IsXiResponse.Is(false)).
		Where(query.Message.UserID.Eq(user.ID)).
		Count()

	if err != nil {
		logger.E("Failed to count user personal questions", tracing.InnerError, err)
		return 0, err
	}

	return count, nil
}

func (x *MessagesRepository) GetUserPersonalQuestionsInChatCount(logger *tracing.Logger, user *entities.User, chatID int64) (int64, error) {
	defer tracing.ProfilePoint(logger, "Messages get user personal questions in chat count completed", "repository.messages.get.user.personal.questions.in.chat.count", "user_id", user.ID, "chat_id", chatID)()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Message.WithContext(ctx)
	count, err := q.
		Where(query.Message.IsXiResponse.Is(false)).
		Where(query.Message.UserID.Eq(user.ID)).
		Where(query.Message.ChatID.Eq(chatID)).
		Count()

	if err != nil {
		logger.E("Failed to count user personal questions in chat", tracing.InnerError, err)
		return 0, err
	}

	return count, nil
}

func (x *MessagesRepository) GetUniqueChatCount(logger *tracing.Logger) (int64, error) {
	defer tracing.ProfilePoint(logger, "Messages get unique chat count completed", "repository.messages.get.unique.chat.count")()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Message.WithContext(ctx)
	var result []struct {
		ChatID int64 `gorm:"column:chat_id"`
	}

	err := q.
		Select(query.Message.ChatID).
		Group(query.Message.ChatID).
		Scan(&result)

	if err != nil {
		logger.E("Failed to count unique chats", tracing.InnerError, err)
		return 0, err
	}

	return int64(len(result)), nil
}

func (x *MessagesRepository) GetAllChatIDs(logger *tracing.Logger) ([]int64, error) {
	defer tracing.ProfilePoint(logger, "Messages get all chat ids completed", "repository.messages.get.all.chat.ids")()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Message.WithContext(ctx)
	var result []int64

	err := q.
		Select(query.Message.ChatID).
		Distinct(query.Message.ChatID).
		Scan(&result)

	if err != nil {
		logger.E("Failed to get all chat ids", tracing.InnerError, err)
		return nil, err
	}

	return result, nil
}