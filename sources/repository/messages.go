package repository

import (
	"context"
	"time"
	"ximanager/sources/persistence/entities"
	"ximanager/sources/persistence/gormdao/query"
	"ximanager/sources/platform"
	"ximanager/sources/texting"
	"ximanager/sources/tracing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type MessagePair struct {
	UserMessage *entities.Message `json:"user_message"`
	XiResponse  *entities.Message `json:"xi_response,omitempty"`
	TokenCount  int               `json:"token_count"`
}

type MessagesRepository struct {
	users     *UsersRepository
	batchSize int
}

func NewMessagesRepository(users *UsersRepository) *MessagesRepository {
	return &MessagesRepository{
		users:     users,
		batchSize: platform.GetAsInt("MESSAGES_FETCH_BATCH_SIZE", 50),
	}
}

func (x *MessagesRepository) SaveMessage(logger *tracing.Logger, msg *tgbotapi.Message, text string, isXiResponse bool) error {
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	user, err := x.users.GetUserByEid(logger, msg.From.ID)
	if err != nil {
		logger.E("Failed to get user", tracing.InnerError, err)
		return err
	}

	message := &entities.Message{
		ChatID:       msg.Chat.ID,
		MessageText:  entities.EncryptedText(text),
		IsAggressive: false,
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

func (x *MessagesRepository) GetMessagePairs(logger *tracing.Logger, user *entities.User, chatID int64) ([]MessagePair, error) {
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 5*time.Minute)
	defer cancel()

	if !platform.BoolValue(user.IsStackAllowed, false) {
		logger.I("User stack access denied")
		return []MessagePair{}, nil
	}

	if !platform.BoolValue(user.IsStackEnabled, true) {
		logger.I("User stack usage disabled")
		return []MessagePair{}, nil
	}

	if user.WindowLimit == 0 {
		logger.I("User window limit is 0")
		return []MessagePair{}, nil
	}

	contextStartTime := time.Now().Add(-x.getContextTimeLimit(chatID))

	batchSize := x.batchSize
	var messagePairs []MessagePair
	var tt int64 = 0
	offset := 0

	q := query.Message.WithContext(ctx)

	for {
		batch, err := tracing.ReportExecutionForRE(logger, func() ([]*entities.Message, error) {
			return q.
				Where(query.Message.IsRemoved.Is(false)).
				Where(query.Message.ChatID.Eq(chatID)).
				Where(query.Message.MessageTime.Gte(contextStartTime)).
				Order(query.Message.MessageTime.Desc()).
				Limit(batchSize).
				Offset(offset).
				Find()
		}, func(l *tracing.Logger) {
			l.I("Messages batch retrieved", "offset", offset, "batch_size", batchSize)
		})

		if err != nil {
			logger.E("Failed to query messages batch", tracing.InnerError, err, "offset", offset, "batch_size", batchSize)
			return nil, err
		}

		if len(batch) == 0 {
			logger.I("No more messages to load", "offset", offset)
			break
		}

		batchPairs := x.zip(logger, batch)

		pairsAdded := 0
		for _, pair := range batchPairs {
			userTokens := texting.Tokens(logger, string(pair.UserMessage.MessageText))
			xiTokens := 0
			if pair.XiResponse != nil {
				xiTokens = texting.Tokens(logger, string(pair.XiResponse.MessageText))
			}

			pairTokens := int64(userTokens + xiTokens)
			pair.TokenCount = userTokens + xiTokens

			if tt+pairTokens > user.WindowLimit {
				logger.I("Token limit reached for message pairs", "total_tokens", tt, "limit", user.WindowLimit, "current_pair_tokens", pairTokens)
				return messagePairs, nil
			}

			tt += pairTokens
			messagePairs = append(messagePairs, pair)
			pairsAdded++
		}

		logger.I("Processed message pairs batch", "batch_size", len(batch), "pairs_found", len(batchPairs), "pairs_added", pairsAdded, "total_pairs", len(messagePairs), "total_tokens", tt)

		if pairsAdded == len(batchPairs) {
			offset += batchSize
		} else {
			break
		}
	}

	logger.I("Message pairs retrieved", "count", len(messagePairs), "total_tokens", tt, "window_limit", user.WindowLimit)

	return messagePairs, nil
}

func (x *MessagesRepository) MustGetMessagePairs(logger *tracing.Logger, user *entities.User, chatID int64) []MessagePair {
	messagePairs, err := x.GetMessagePairs(logger, user, chatID)
	if err != nil {
		logger.F("Failed to get message pairs", tracing.InnerError, err)
		return []MessagePair{}
	}
	return messagePairs
}

// zip группирует сообщения в пары (User Message + Xi Response)
// Сообщения приходят в порядке DESC (самые новые сначала)
func (x *MessagesRepository) zip(logger *tracing.Logger, messages []*entities.Message) []MessagePair {
	var pairs []MessagePair

	// Обрабатываем сообщения в обратном порядке (самые старые сначала)
	// чтобы правильно строить пары: User Message -> Xi Response
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]

		// Если это сообщение пользователя, создаем новую пару
		if !msg.IsXiResponse {
			pair := MessagePair{
				UserMessage: msg,
				XiResponse:  nil,
			}

			// Ищем соответствующий ответ Xi ПОСЛЕ этого сообщения (более новый по времени)
			for j := i - 1; j >= 0; j-- {
				nextMsg := messages[j]

				// Если нашли ответ Xi в том же чате, привязываем его
				if nextMsg.IsXiResponse && nextMsg.ChatID == msg.ChatID {
					pair.XiResponse = nextMsg
					break
				}

				// Если встретили другое сообщение пользователя, прерываем поиск
				if !nextMsg.IsXiResponse {
					break
				}
			}

			pairs = append(pairs, pair)
		}
	}

	// Возвращаем пары в правильном порядке (самые новые сначала)
	// для соответствия порядку исходных сообщений
	for i := 0; i < len(pairs)/2; i++ {
		pairs[i], pairs[len(pairs)-1-i] = pairs[len(pairs)-1-i], pairs[i]
	}

	logger.I("Grouped messages into pairs", "messages_count", len(messages), "pairs_count", len(pairs))
	return pairs
}

func (x *MessagesRepository) GetTotalUserQuestionsCount(logger *tracing.Logger) (int64, error) {
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

func (x *MessagesRepository) MarkChatMessagesAsRemoved(logger *tracing.Logger, chatID int64, fromTime time.Time) error {
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 60*time.Second)
	defer cancel()

	q := query.Message.WithContext(ctx)
	_, err := q.
		Where(query.Message.ChatID.Eq(chatID)).
		Where(query.Message.MessageTime.Lte(fromTime)).
		Update(query.Message.IsRemoved, true)

	if err != nil {
		logger.E("Failed to mark chat messages as removed", tracing.InnerError, err, "from_time", fromTime)
		return err
	}

	logger.I("Marked chat messages as removed", "from_time", fromTime)
	return nil
}

func (x *MessagesRepository) isPrivateChat(chatID int64) bool {
	return chatID > 0
}

func (x *MessagesRepository) getContextTimeLimit(chatID int64) time.Duration {
	if x.isPrivateChat(chatID) {
		return 7 * 24 * time.Hour
	}
	return 2 * 24 * time.Hour
}
