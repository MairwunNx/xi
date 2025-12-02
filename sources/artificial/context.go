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
	redis       *redis.Client
	config      *configuration.Config
	donations   *repository.DonationsRepository
	agentSystem *AgentSystem
	features    *features.FeatureManager
	tariffs     *repository.TariffsRepository
	log         *tracing.Logger
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
		redis:       redis,
		config:      config,
		donations:   donations,
		agentSystem: agentSystem,
		features:    fm,
		tariffs:     tariffs,
		log:         log,
	}, nil
}

func (x *ContextManager) getContextLimits(ctx context.Context, grade platform.UserGrade) (ContextLimits, error) {
	tariff, err := getTariffWithFallback(x.log, x.tariffs, grade)
	if err != nil {
		return ContextLimits{}, err
	}
	return ContextLimits{
		TTL:         tariff.ContextTTLSeconds,
		MaxMessages: tariff.ContextMaxMessages,
		MaxTokens:   tariff.ContextMaxTokens,
	}, nil
}

func (x *ContextManager) getChatHistoryKey(chatID platform.ChatID) string {
	return fmt.Sprintf("chat_history:%d", chatID)
}

func (x *ContextManager) Fetch(logger *tracing.Logger, chatID platform.ChatID, userGrade platform.UserGrade) ([]platform.RedisMessage, error) {
	defer tracing.ProfilePoint(logger, "Context fetch completed", "artificial.context.fetch", "chat_id", chatID, "user_grade", userGrade)()

	if !x.IsEnabled(logger, chatID) {
		logger.I("Context collection is disabled for this chat, returning empty", "chat_id", chatID)
		return []platform.RedisMessage{}, nil
	}

	startTime := time.Now()
	
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 5*time.Second)
	defer cancel()

	limits, err := x.getContextLimits(ctx, userGrade)
	if err != nil {
		logger.E("Failed to get context limits", tracing.InnerError, err)
		return nil, err
	}

	key := x.getChatHistoryKey(chatID)

	messageStrings, err := x.redis.LRange(ctx, key, 0, int64(limits.MaxMessages-1)).Result()
	redisLatency := time.Since(startTime)
	
	if err != nil {
		logger.E("Failed to fetch chat history from Redis", "key", key, "redis_latency_ms", redisLatency.Milliseconds(), tracing.InnerError, err)
		return nil, err
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

	recentCount := x.config.AI.Agents.Summarization.RecentMessagesCount
	if recentCount <= 0 {
		recentCount = 4 // Fallback to default
	}

	if len(allMessages) <= recentCount {
		x.logFetchSuccess(logger, chatID, userGrade, rawMessageCount, allMessages, skippedMessages, false, redisLatency, time.Since(startTime), limits.MaxTokens)
		return allMessages, nil
	}

	var lastFourMessages []platform.RedisMessage
	var olderMessages []platform.RedisMessage
	summarizationOccurred := false
	
	lastFourMessages = allMessages[len(allMessages)-recentCount:]
	olderMessages = allMessages[:len(allMessages)-recentCount]
	
	processedOlderMessages, summarizationOccurred, err := x.processMessagesWithSummarization(logger, olderMessages)
	if err != nil {
		logger.W("Failed to process messages with summarization, using original", tracing.InnerError, err)
		processedOlderMessages = olderMessages
	}
	
	finalMessages := append(processedOlderMessages, lastFourMessages...)
	
	if summarizationOccurred {
		x.updateRedisAfterSummarization(logger, chatID, userGrade, finalMessages, processedOlderMessages, lastFourMessages)
	}
	
	messages := x.applyTokenLimit(logger, finalMessages, limits.MaxTokens)
	x.logFetchSuccess(logger, chatID, userGrade, rawMessageCount, messages, skippedMessages, summarizationOccurred, redisLatency, time.Since(startTime), limits.MaxTokens)

	return messages, nil
}

func (x *ContextManager) Store(
	logger *tracing.Logger,
	chatID platform.ChatID,
	userGrade platform.UserGrade,
	message platform.RedisMessage,
) error {
	defer tracing.ProfilePoint(logger, "Context store completed", "artificial.context.store", "chat_id", chatID, "user_grade", userGrade, "message_role", message.Role)()

	if !x.IsEnabled(logger, chatID) {
		logger.I("Context collection is disabled for this chat, skipping store", "chat_id", chatID)
		return nil
	}

	startTime := time.Now()
	
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 5*time.Second)
	defer cancel()

	limits, err := x.getContextLimits(ctx, userGrade)
	if err != nil {
		logger.E("Failed to get context limits", tracing.InnerError, err)
		return err
	}

	key := x.getChatHistoryKey(chatID)

	messageStr, err := json.Marshal(message)
	if err != nil {
		logger.E("Failed to marshal message for Redis", tracing.InnerError, err)
		return err
	}

	messageTokens := tokenizer.Tokens(logger, message.Content)

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

	limits, err := x.getContextLimits(ctx, userGrade)
	if err != nil {
		logger.E("Failed to get context limits for stats", tracing.InnerError, err)
		return nil, err
	}

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
		MaxMessages:     limits.MaxMessages,
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
	olderMessages []platform.RedisMessage,
	recentMessages []platform.RedisMessage,
) {
	if err := x.replaceHistoryInRedis(logger, chatID, userGrade, finalMessages); err != nil {
		logger.E("Failed to update Redis with summarized history", tracing.InnerError, err)
	} else {
		logger.I("redis_history_updated_after_summarization",
			"chat_id", chatID,
			"total_messages", len(finalMessages),
			"older_messages", len(olderMessages),
			"recent_messages", len(recentMessages),
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

func (x *ContextManager) processMessagesWithSummarization(
	logger *tracing.Logger,
	messages []platform.RedisMessage,
) ([]platform.RedisMessage, bool, error) {
	messageFeatureEnabled := x.features.IsEnabled(features.FeatureMessageSummarization)
	clusterFeatureEnabled := x.features.IsEnabled(features.FeatureClusterSummarization)
	
	if (!messageFeatureEnabled && !clusterFeatureEnabled) || len(messages) == 0 {
		return messages, false, nil
	}
	
	startTime := time.Now()
	originalTokens := x.countTotalTokens(logger, messages)
	
	messagesAfterFirstPass, firstPassCount := messages, 0
	if messageFeatureEnabled {
		messagesAfterFirstPass, firstPassCount = x.summarizeIndividualMessages(logger, messages)
	}
	
	finalMessages, secondPassCount := messagesAfterFirstPass, 0
	if clusterFeatureEnabled {
		finalMessages, secondPassCount = x.summarizeClusters(logger, messagesAfterFirstPass)
	}
	
	summarizationOccurred := firstPassCount > 0 || secondPassCount > 0
	if summarizationOccurred {
		x.logSummarizationStats(logger, messages, finalMessages, firstPassCount, secondPassCount, originalTokens, time.Since(startTime))
	}
	
	return finalMessages, summarizationOccurred, nil
}

func (x *ContextManager) logSummarizationStats(
	logger *tracing.Logger,
	originalMessages []platform.RedisMessage,
	finalMessages []platform.RedisMessage,
	individualCount int,
	clusterCount int,
	originalTokens int,
	duration time.Duration,
) {
	finalTokens := x.countTotalTokens(logger, finalMessages)
	tokenReduction := originalTokens - finalTokens
	reductionPercent := 0
	if originalTokens > 0 {
		reductionPercent = int(float64(tokenReduction) / float64(originalTokens) * 100)
	}
	
	logger.I("summarization_completed",
		"original_messages", len(originalMessages),
		"final_messages", len(finalMessages),
		"individual_summarized", individualCount,
		"clusters_summarized", clusterCount,
		"original_tokens", originalTokens,
		"final_tokens", finalTokens,
		"tokens_reduced", tokenReduction,
		"reduction_percent", reductionPercent,
		"duration_ms", duration.Milliseconds(),
	)
}

func (x *ContextManager) summarizeIndividualMessages(
	logger *tracing.Logger,
	messages []platform.RedisMessage,
) ([]platform.RedisMessage, int) {
	
	result := make([]platform.RedisMessage, 0, len(messages))
	summarizedCount := 0
	
	for _, msg := range messages {
		if msg.Role == platform.MessageRoleSystem || msg.Role == platform.MessageRoleTool || msg.IsCompressed {
			result = append(result, msg)
			continue
		}
		
		tokens := tokenizer.Tokens(logger, msg.Content)
		
		if tokens > x.config.AI.Agents.Summarization.SingleMessageTokenThreshold {
			summarized, err := x.agentSystem.SummarizeContent(logger, msg.Content, "message")
			if err != nil {
				logger.W("Failed to summarize individual message, using original", tracing.InnerError, err)
				result = append(result, msg)
				continue
			}
			
			summarizedTokens := tokenizer.Tokens(logger, summarized)
			tokenReduction := tokens - summarizedTokens
			reductionPercent := 0
			if tokens > 0 {
				reductionPercent = int(float64(tokenReduction) / float64(tokens) * 100)
			}
			
			logger.I("individual_message_summarized",
				"role", msg.Role,
				"original_tokens", tokens,
				"summarized_tokens", summarizedTokens,
				"tokens_reduced", tokenReduction,
				"reduction_percent", reductionPercent,
			)
			
			summarizedMsg := platform.RedisMessage{
				Role:         msg.Role,
				Content:      summarized,
				IsCompressed: false,
			}
			result = append(result, summarizedMsg)
			summarizedCount++
		} else {
			result = append(result, msg)
		}
	}
	
	return result, summarizedCount
}

func (x *ContextManager) summarizeClusters(
	logger *tracing.Logger,
	messages []platform.RedisMessage,
) ([]platform.RedisMessage, int) {

	if len(messages) == 0 {
		return messages, 0
	}

	result := make([]platform.RedisMessage, 0, len(messages))
	summarizedClusters := 0
	i := 0

	for i < len(messages) {
		// Tool messages are added as-is
		if messages[i].Role == platform.MessageRoleTool {
			result = append(result, messages[i])
			i++
			continue
		}

		// If message doesn't start a pair (not user), add and continue
		if messages[i].Role != platform.MessageRoleUser {
			result = append(result, messages[i])
			i++
			continue
		}

		// Try to form a cluster of pairs: user -> [tool*] -> assistant
		clusterStart := i
		clusterEnd := i
		pairsInCluster := 0

		for clusterEnd < len(messages) && pairsInCluster < x.config.AI.Agents.Summarization.ClusterSize {
			// Pair must start with user
			if messages[clusterEnd].Role != platform.MessageRoleUser {
				break
			}

			pairStart := clusterEnd
			clusterEnd++ // Move past user

			// Skip tool messages between user and assistant
			for clusterEnd < len(messages) && messages[clusterEnd].Role == platform.MessageRoleTool {
				clusterEnd++
			}

			// Check if assistant follows
			if clusterEnd < len(messages) && messages[clusterEnd].Role == platform.MessageRoleAssistant {
				clusterEnd++ // Include assistant
				pairsInCluster++
			} else {
				// No assistant found — rollback, pair not formed
				clusterEnd = pairStart
				break
			}
		}

		// If formed less than 2 pairs — add as-is and CONTINUE (not break!)
		if pairsInCluster < 2 {
			if clusterEnd == clusterStart {
				// Couldn't form any pair — add current message and move on
				result = append(result, messages[i])
				i++
			} else {
				// Formed 1 pair — add it as-is
				result = append(result, messages[clusterStart:clusterEnd]...)
				i = clusterEnd
			}
			continue
		}

		// Formed cluster of >=2 pairs
		clusterMessages := messages[clusterStart:clusterEnd]
		clusterTokens := x.countTotalTokens(logger, clusterMessages)

		if clusterTokens > x.config.AI.Agents.Summarization.ClusterTokenThreshold {
			clusterContent := x.formatClusterContent(clusterMessages)

			summarized, err := x.agentSystem.SummarizeContent(logger, clusterContent, "cluster")
			if err != nil {
				logger.W("Failed to summarize cluster, using original messages", tracing.InnerError, err)
				result = append(result, clusterMessages...)
				i = clusterEnd
				continue
			}

			x.logClusterSummarization(logger, pairsInCluster, clusterMessages, clusterTokens, summarized)

			result = append(result, platform.RedisMessage{
				Role:         platform.MessageRoleSystem,
				Content:      summarized,
				IsCompressed: true,
			})
			summarizedClusters++
			i = clusterEnd
		} else {
			result = append(result, clusterMessages...)
			i = clusterEnd
		}
	}

	return result, summarizedClusters
}

func (x *ContextManager) formatClusterContent(messages []platform.RedisMessage) string {
	var builder strings.Builder
	for idx, msg := range messages {
		role := "User"
		if msg.Role == platform.MessageRoleAssistant {
			role = "Assistant"
		}
		builder.WriteString(fmt.Sprintf("[Message %d] %s: %s\n\n", idx+1, role, msg.Content))
	}
	return builder.String()
}

func (x *ContextManager) logClusterSummarization(
	logger *tracing.Logger,
	pairsCount int,
	clusterMessages []platform.RedisMessage,
	originalTokens int,
	summarized string,
) {
	summarizedTokens := tokenizer.Tokens(logger, summarized)
	tokenReduction := originalTokens - summarizedTokens
	reductionPercent := 0
	if originalTokens > 0 {
		reductionPercent = int(float64(tokenReduction) / float64(originalTokens) * 100)
	}
	
	logger.I("cluster_summarized",
		"pairs_count", pairsCount,
		"messages_in_cluster", len(clusterMessages),
		"original_tokens", originalTokens,
		"summarized_tokens", summarizedTokens,
		"tokens_reduced", tokenReduction,
		"reduction_percent", reductionPercent,
	)
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

	limits, err := x.getContextLimits(ctx, userGrade)
	if err != nil {
		return err
	}

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