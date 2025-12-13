package artificial

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"ximanager/sources/configuration"
	"ximanager/sources/features"
	"ximanager/sources/platform"
	"ximanager/sources/repository"
	"ximanager/sources/texting/tokenizer"
	"ximanager/sources/tracing"

	"github.com/redis/go-redis/v9"
)

type ContextManager struct {
	redis                *redis.Client
	config               *configuration.Config
	donations            *repository.DonationsRepository
	agentSystem          *AgentSystem
	features             *features.FeatureManager
	tariffs              *repository.TariffsRepository
	log                  *tracing.Logger
	summarizationHandler SummarizationHandler
}

type SummarizationHandler interface {
	OnSummarization(chatID platform.ChatID)
}

func NewContextManager(
	redis *redis.Client,
	config *configuration.Config,
	donations *repository.DonationsRepository,
	agentSystem *AgentSystem,
	fm *features.FeatureManager,
	tariffs *repository.TariffsRepository,
	log *tracing.Logger,
) (*ContextManager, error) {
	return &ContextManager{
		redis:                redis,
		config:               config,
		donations:            donations,
		agentSystem:          agentSystem,
		features:             fm,
		tariffs:              tariffs,
		log:                  log,
		summarizationHandler: nil,
	}, nil
}

func (x *ContextManager) SetSummarizationHandler(handler SummarizationHandler) {
	x.summarizationHandler = handler
}

func (x *ContextManager) getContextLimits() ContextLimits {
	return ContextLimits{
		TTL:       int(x.config.Redis.MessagesTTL.Seconds()),
		MaxTokens: x.config.AI.Agents.Summarization.MaxContextTokens,
	}
}

func (x *ContextManager) getChatHistoryKey(chatID platform.ChatID) string {
	return fmt.Sprintf("chat_history:%d", chatID)
}

func (x *ContextManager) Fetch(logger *tracing.Logger, chatID platform.ChatID, userGrade platform.UserGrade) ([]platform.RedisMessage, bool, error) {
	defer tracing.ProfilePoint(logger, "Context fetch completed", "artificial.context.fetch", "chat_id", chatID, "user_grade", userGrade)()

	if !x.IsEnabled(logger, chatID) {
		logger.I("Context collection is disabled for this chat, returning empty", "chat_id", chatID)
		return []platform.RedisMessage{}, false, nil
	}

	startTime := time.Now()

	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 5*time.Second)
	defer cancel()

	limits := x.getContextLimits()
	key := x.getChatHistoryKey(chatID)

	messageStrings, err := x.redis.LRange(ctx, key, 0, -1).Result()
	redisLatency := time.Since(startTime)

	if err != nil {
		logger.E("Failed to fetch chat history from Redis", "key", key, "redis_latency_ms", redisLatency.Milliseconds(), tracing.InnerError, err)
		return nil, false, err
	}

	rawMessageCount := len(messageStrings)
	var allMessages []platform.RedisMessage
	skippedMessages := 0

	for _, msgStr := range messageStrings {
		var msg platform.RedisMessage
		if err := json.Unmarshal([]byte(msgStr), &msg); err != nil {
			logger.W("Failed to parse message from Redis, skipping", "message", msgStr, tracing.InnerError, err)
			skippedMessages++
			continue
		}
		allMessages = append(allMessages, msg)
	}

	x.reverseMessages(allMessages)

	recentCount := x.config.AI.Agents.Summarization.RecentMessagesToKeep
	if recentCount <= 0 {
		recentCount = 3
	}

	totalTokens := x.countTotalTokens(logger, allMessages)
	triggerThreshold := int(float64(limits.MaxTokens) * float64(x.config.AI.Agents.Summarization.TriggerThresholdPercent) / 100.0)
	if triggerThreshold <= 0 {
		triggerThreshold = int(float64(limits.MaxTokens) * 0.75)
	}

	summarizationNeeded := totalTokens > triggerThreshold
	summarizationOccurred := false

	var finalMessages []platform.RedisMessage
	if summarizationNeeded && len(allMessages) > recentCount {
		logger.I("context_summarization_triggered",
			"total_tokens", totalTokens,
			"trigger_threshold", triggerThreshold,
			"max_tokens", limits.MaxTokens,
			"messages_count", len(allMessages),
		)

		processedMessages, err := x.summarizeHistory(logger, allMessages, recentCount)
		if err != nil {
			logger.W("Failed to summarize history, using original", tracing.InnerError, err)
			finalMessages = allMessages
		} else {
			finalMessages = processedMessages
			summarizationOccurred = true
		}
	} else {
		finalMessages = allMessages
	}

	if summarizationOccurred {
		x.updateRedisAfterSummarization(logger, chatID, userGrade, finalMessages)
		if x.summarizationHandler != nil {
			x.summarizationHandler.OnSummarization(chatID)
		}
	}

	messages := x.applyTokenLimit(logger, finalMessages, limits.MaxTokens)
	x.logFetchSuccess(logger, chatID, userGrade, rawMessageCount, messages, skippedMessages, summarizationOccurred, redisLatency, time.Since(startTime), limits.MaxTokens)

	return messages, summarizationOccurred, nil
}

func (x *ContextManager) Store(
	logger *tracing.Logger,
	chatID platform.ChatID,
	userGrade platform.UserGrade,
	message platform.RedisMessage,
) error {
	defer tracing.ProfilePoint(logger, "Context store completed", "artificial.context.store", "chat_id", chatID, "user_grade", userGrade, "message_role", message.Role)()

	if message.Role == platform.MessageRoleTool {
		logger.D("Skipping tool message storage", "chat_id", chatID)
		return nil
	}

	if !x.IsEnabled(logger, chatID) {
		logger.I("Context collection is disabled for this chat, skipping store", "chat_id", chatID)
		return nil
	}

	startTime := time.Now()

	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 5*time.Second)
	defer cancel()

	limits := x.getContextLimits()
	key := x.getChatHistoryKey(chatID)

	messageStr, err := json.Marshal(message)
	if err != nil {
		logger.E("Failed to marshal message for Redis", tracing.InnerError, err)
		return err
	}

	messageTokens := tokenizer.Tokens(logger, message.Content)

	pipe := x.redis.TxPipeline()
	pipe.LPush(ctx, key, messageStr)
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
		"ttl_seconds", limits.TTL,
		"duration_ms", duration.Milliseconds(),
	)

	return nil
}

func (x *ContextManager) Clear(logger *tracing.Logger, chatID platform.ChatID) error {
	defer tracing.ProfilePoint(logger, "Context clear completed", "artificial.context.clear", "chat_id", chatID)()
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

func (x *ContextManager) getContextEnabledKey(chatID platform.ChatID) string {
	return fmt.Sprintf("chat_context_enabled:%d", chatID)
}

func (x *ContextManager) SetEnabled(logger *tracing.Logger, chatID platform.ChatID, enabled bool) error {
	defer tracing.ProfilePoint(logger, "Context set enabled completed", "artificial.context.set.enabled", "chat_id", chatID, "enabled", enabled)()

	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 5*time.Second)
	defer cancel()

	key := x.getContextEnabledKey(chatID)

	if enabled {
		// Remove the key to enable (default is enabled)
		err := x.redis.Del(ctx, key).Err()
		if err != nil {
			logger.E("Failed to enable context", "key", key, tracing.InnerError, err)
			return err
		}
	} else {
		// Set key to "0" to disable
		err := x.redis.Set(ctx, key, "0", 0).Err()
		if err != nil {
			logger.E("Failed to disable context", "key", key, tracing.InnerError, err)
			return err
		}
	}

	logger.I("Context enabled status changed", "chat_id", chatID, "enabled", enabled)
	return nil
}

func (x *ContextManager) IsEnabled(logger *tracing.Logger, chatID platform.ChatID) bool {
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 5*time.Second)
	defer cancel()

	key := x.getContextEnabledKey(chatID)
	val, err := x.redis.Get(ctx, key).Result()
	if err == redis.Nil {
		// Key doesn't exist = enabled (default)
		return true
	}
	if err != nil {
		logger.W("Failed to check context enabled status, assuming enabled", tracing.InnerError, err)
		return true
	}

	return val != "0"
}

type ContextStats struct {
	Enabled           bool
	CurrentMessages   int
	MaxMessages       int
	CurrentTokens     int
	MaxTokens         int
}

func (x *ContextManager) GetStats(logger *tracing.Logger, chatID platform.ChatID, userGrade platform.UserGrade) (*ContextStats, error) {
	defer tracing.ProfilePoint(logger, "Context get stats completed", "artificial.context.get.stats", "chat_id", chatID)()

	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 5*time.Second)
	defer cancel()

	limits := x.getContextLimits()
	key := x.getChatHistoryKey(chatID)
	messageStrings, err := x.redis.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		logger.E("Failed to fetch chat history for stats", tracing.InnerError, err)
		return nil, err
	}

	var totalTokens int
	for _, msgStr := range messageStrings {
		var msg platform.RedisMessage
		if err := json.Unmarshal([]byte(msgStr), &msg); err != nil {
			continue
		}
		totalTokens += tokenizer.Tokens(logger, msg.Content)
	}

	enabled := x.IsEnabled(logger, chatID)

	return &ContextStats{
		Enabled:         enabled,
		CurrentMessages: len(messageStrings),
		MaxMessages:     0,
		CurrentTokens:   totalTokens,
		MaxTokens:       limits.MaxTokens,
	}, nil
}

func (x *ContextManager) reverseMessages(messages []platform.RedisMessage) {
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
}

func (x *ContextManager) applyTokenLimit(logger *tracing.Logger, messages []platform.RedisMessage, maxTokens int) []platform.RedisMessage {
	var result []platform.RedisMessage
	totalTokens := 0

	for _, msg := range messages {
		msgTokens := tokenizer.Tokens(logger, msg.Content)
		if totalTokens+msgTokens > maxTokens {
			logger.I("Token limit exceeded, truncating chat history",
				"total_tokens", totalTokens,
				"max_tokens", maxTokens,
				"remaining_messages", len(messages)-len(result),
			)
			break
		}
		result = append(result, msg)
		totalTokens += msgTokens
	}

	return result
}

func (x *ContextManager) updateRedisAfterSummarization(
	logger *tracing.Logger,
	chatID platform.ChatID,
	userGrade platform.UserGrade,
	finalMessages []platform.RedisMessage,
) {
	if err := x.replaceHistoryInRedis(logger, chatID, userGrade, finalMessages); err != nil {
		logger.E("Failed to update Redis with summarized history", tracing.InnerError, err)
	} else {
		logger.I("redis_history_updated_after_summarization",
			"chat_id", chatID,
			"total_messages", len(finalMessages),
		)
	}
}

func (x *ContextManager) logFetchSuccess(
	logger *tracing.Logger,
	chatID platform.ChatID,
	userGrade platform.UserGrade,
	rawCount int,
	messages []platform.RedisMessage,
	skipped int,
	summarized bool,
	redisLatency time.Duration,
	totalDuration time.Duration,
	maxTokens int,
) {
	totalTokens := x.countTotalTokens(logger, messages)
	tokenUsagePercent := 0
	if maxTokens > 0 {
		tokenUsagePercent = int(float64(totalTokens) / float64(maxTokens) * 100)
	}

	logger.I("context_fetch_success",
		"chat_id", chatID,
		"user_grade", userGrade,
		"raw_message_count", rawCount,
		"fetched_message_count", len(messages),
		"skipped_messages", skipped,
		"total_tokens", totalTokens,
		"max_tokens", maxTokens,
		"token_usage_percent", tokenUsagePercent,
		"summarization_occurred", summarized,
		"redis_latency_ms", redisLatency.Milliseconds(),
		"total_duration_ms", totalDuration.Milliseconds(),
	)
}

func (x *ContextManager) countTotalTokens(logger *tracing.Logger, messages []platform.RedisMessage) int {
	total := 0
	for _, msg := range messages {
		total += tokenizer.Tokens(logger, msg.Content)
	}
	return total
}

func (x *ContextManager) summarizeHistory(
	logger *tracing.Logger,
	messages []platform.RedisMessage,
	recentToKeep int,
) ([]platform.RedisMessage, error) {
	if len(messages) <= recentToKeep {
		return messages, nil
	}

	startTime := time.Now()
	originalTokens := x.countTotalTokens(logger, messages)

	recentMessages := messages[len(messages)-recentToKeep:]
	olderMessages := messages[:len(messages)-recentToKeep]

	var messagesToSummarize []platform.RedisMessage
	var previousSummary string

	for _, msg := range olderMessages {
		if msg.Role == platform.MessageRoleSystem && msg.IsCompressed {
			previousSummary = msg.Content
		} else if msg.Role != platform.MessageRoleSystem {
			messagesToSummarize = append(messagesToSummarize, msg)
		}
	}

	if len(messagesToSummarize) == 0 && previousSummary == "" {
		return recentMessages, nil
	}

	if len(messagesToSummarize) == 0 && previousSummary != "" {
		summarizedMessage := platform.RedisMessage{
			Role:         platform.MessageRoleSystem,
			Content:      previousSummary,
			IsCompressed: true,
		}
		return append([]platform.RedisMessage{summarizedMessage}, recentMessages...), nil
	}

	historyText := x.formatHistoryForSummarization(messagesToSummarize, previousSummary)

	summarized, err := x.agentSystem.SummarizeContent(logger, historyText, "history")
	if err != nil {
		return nil, err
	}

	summarizedTokens := tokenizer.Tokens(logger, summarized)
	tokenReduction := originalTokens - (summarizedTokens + x.countTotalTokens(logger, recentMessages))

	logger.I("history_summarized",
		"original_messages", len(olderMessages),
		"recent_messages", len(recentMessages),
		"original_tokens", originalTokens,
		"summarized_tokens", summarizedTokens,
		"tokens_reduced", tokenReduction,
		"duration_ms", time.Since(startTime).Milliseconds(),
	)

	summarizedMessage := platform.RedisMessage{
		Role:         platform.MessageRoleSystem,
		Content:      summarized,
		IsCompressed: true,
	}

	return append([]platform.RedisMessage{summarizedMessage}, recentMessages...), nil
}

func (x *ContextManager) formatHistoryForSummarization(messages []platform.RedisMessage, previousSummary string) string {
	var builder strings.Builder

	if previousSummary != "" {
		builder.WriteString(fmt.Sprintf("[Summary] System: %s\n\n", previousSummary))
	}

	for idx, msg := range messages {
		role := "User"
		switch msg.Role {
		case platform.MessageRoleAssistant:
			role = "Assistant"
		case platform.MessageRoleSystem:
			role = "System"
		}
		builder.WriteString(fmt.Sprintf("[%d] %s: %s\n\n", idx+1, role, msg.Content))
	}
	return builder.String()
}

func (x *ContextManager) replaceHistoryInRedis(
	logger *tracing.Logger,
	chatID platform.ChatID,
	userGrade platform.UserGrade,
	messages []platform.RedisMessage,
) error {
	key := x.getChatHistoryKey(chatID)

	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 10*time.Second)
	defer cancel()

	limits := x.getContextLimits()

	// Use transaction pipeline for atomicity — Del + LPush + Expire all together
	pipe := x.redis.TxPipeline()

	// Delete old history (inside pipeline for atomicity)
	pipe.Del(ctx, key)

	// Store new history in reverse order (LPUSH adds to the beginning)
	for i := len(messages) - 1; i >= 0; i-- {
		msgStr, err := json.Marshal(messages[i])
		if err != nil {
			logger.W("Failed to marshal message, skipping", tracing.InnerError, err)
			continue
		}
		pipe.LPush(ctx, key, msgStr)
	}

	// Set expiration
	pipe.Expire(ctx, key, time.Duration(limits.TTL)*time.Second)

	if _, err := pipe.Exec(ctx); err != nil {
		logger.E("Failed to replace history in Redis", tracing.InnerError, err)
		return err
	}

	logger.I("redis_history_replaced",
		"chat_id", chatID,
		"message_count", len(messages),
		"ttl_seconds", limits.TTL,
	)

	return nil
}
