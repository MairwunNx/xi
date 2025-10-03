package artificial

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"ximanager/sources/platform"
	"ximanager/sources/repository"
	"ximanager/sources/tracing"

	"github.com/pkoukk/tiktoken-go"
	"github.com/redis/go-redis/v9"
)

type ContextManager struct {
	redis     *redis.Client
	config    *AIConfig
	donations *repository.DonationsRepository
	tokenizer *tiktoken.Tiktoken
}

func NewContextManager(
	redis *redis.Client,
	config *AIConfig,
	donations *repository.DonationsRepository,
) (*ContextManager, error) {
	tokenizer, err := tiktoken.GetEncoding("cl100k_base")
	if err != nil {
		return nil, fmt.Errorf("failed to get tokenizer encoding: %w", err)
	}
	return &ContextManager{
		redis:     redis,
		config:    config,
		donations: donations,
		tokenizer: tokenizer,
	}, nil
}

func (x *ContextManager) getContextLimits(grade platform.UserGrade) ContextLimits {
	if limits, ok := x.config.GradeLimits[grade]; ok {
		return limits.Context
	}
	return x.config.GradeLimits[platform.GradeBronze].Context
}

func (x *ContextManager) getChatHistoryKey(chatID platform.ChatID) string {
	return fmt.Sprintf("chat_history:%d", chatID)
}

func (x *ContextManager) Fetch(logger *tracing.Logger, chatID platform.ChatID, userGrade platform.UserGrade) ([]platform.RedisMessage, error) {
	startTime := time.Now()
	limits := x.getContextLimits(userGrade)
	key := x.getChatHistoryKey(chatID)

	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 5*time.Second)
	defer cancel()

	messageStrings, err := x.redis.LRange(ctx, key, 0, int64(limits.MaxMessages-1)).Result()
	redisLatency := time.Since(startTime)
	
	if err != nil {
		logger.E("Failed to fetch chat history from Redis", "key", key, "redis_latency_ms", redisLatency.Milliseconds(), tracing.InnerError, err)
		return nil, err
	}

	rawMessageCount := len(messageStrings)
	var messages []platform.RedisMessage
	var totalTokens int
	skippedMessages := 0
	truncatedByTokens := false

	for _, msgStr := range messageStrings {
		var msg platform.RedisMessage
		if err := json.Unmarshal([]byte(msgStr), &msg); err != nil {
			logger.W("Failed to parse message from Redis, skipping", "message", msgStr, tracing.InnerError, err)
			skippedMessages++
			continue
		}

		msgTokens := len(x.tokenizer.Encode(msg.Content, nil, nil))
		if totalTokens+msgTokens > limits.MaxTokens {
			truncatedByTokens = true
			logger.I("Token limit exceeded, truncating chat history", "total_tokens", totalTokens, "max_tokens", limits.MaxTokens, "remaining_messages", rawMessageCount-len(messages))
			break
		}

		messages = append(messages, msg)
		totalTokens += msgTokens
	}

	// Reverse messages to get correct chronological order (LPUSH stores them in reverse)
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	totalDuration := time.Since(startTime)
	tokenUsagePercent := 0
	if limits.MaxTokens > 0 {
		tokenUsagePercent = int(float64(totalTokens) / float64(limits.MaxTokens) * 100)
	}

	logger.I("context_fetch_success",
		"chat_id", chatID,
		"user_grade", userGrade,
		"raw_message_count", rawMessageCount,
		"fetched_message_count", len(messages),
		"skipped_messages", skippedMessages,
		"total_tokens", totalTokens,
		"max_tokens", limits.MaxTokens,
		"token_usage_percent", tokenUsagePercent,
		"truncated_by_tokens", truncatedByTokens,
		"redis_latency_ms", redisLatency.Milliseconds(),
		"total_duration_ms", totalDuration.Milliseconds(),
	)

	return messages, nil
}

func (x *ContextManager) Store(
	logger *tracing.Logger,
	chatID platform.ChatID,
	userGrade platform.UserGrade,
	message platform.RedisMessage,
) error {
	startTime := time.Now()
	limits := x.getContextLimits(userGrade)
	key := x.getChatHistoryKey(chatID)

	messageStr, err := json.Marshal(message)
	if err != nil {
		logger.E("Failed to marshal message for Redis", tracing.InnerError, err)
		return err
	}

	messageTokens := len(x.tokenizer.Encode(message.Content, nil, nil))

	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 5*time.Second)
	defer cancel()

	pipe := x.redis.TxPipeline()
	pipe.LPush(ctx, key, messageStr)
	pipe.LTrim(ctx, key, 0, int64(limits.MaxMessages-1))
	pipe.Expire(ctx, key, time.Duration(limits.TTL)*time.Second)

	if _, err := pipe.Exec(ctx); err != nil {
		duration := time.Since(startTime)
		logger.E("Failed to store message in Redis", "key", key, "duration_ms", duration.Milliseconds(), tracing.InnerError, err)
		return err
	}

	duration := time.Since(startTime)
	logger.I("context_store_success",
		"chat_id", chatID,
		"user_grade", userGrade,
		"message_role", message.Role,
		"message_tokens", messageTokens,
		"max_messages", limits.MaxMessages,
		"ttl_seconds", limits.TTL,
		"duration_ms", duration.Milliseconds(),
	)

	return nil
}

func (x *ContextManager) Clear(logger *tracing.Logger, chatID platform.ChatID) error {
	key := x.getChatHistoryKey(chatID)

	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 5*time.Second)
	defer cancel()

	err := x.redis.Del(ctx, key).Err()
	if err != nil {
		logger.E("Failed to clear chat history from Redis", "key", key, tracing.InnerError, err)
		return err
	}

	logger.I("Chat history cleared successfully", "key", key)
	return nil
}
