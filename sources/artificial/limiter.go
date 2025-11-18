package artificial

import (
	"context"
	"fmt"
	"time"
	"ximanager/sources/platform"
	"ximanager/sources/tracing"

	"github.com/redis/go-redis/v9"
)

type UsageType string

const (
	UsageTypeVision  UsageType = "vision"
	UsageTypeDialer  UsageType = "dialer"
	UsageTypeWhisper UsageType = "whisper"
)

type UsageLimiter struct {
	redis  *redis.Client
	config *AIConfig
}

func NewUsageLimiter(redis *redis.Client, config *AIConfig) *UsageLimiter {
	return &UsageLimiter{
		redis:  redis,
		config: config,
	}
}

type LimitCheckResult struct {
	Exceeded bool
	IsDaily  bool
}

func (x *UsageLimiter) getUsageLimits(grade platform.UserGrade) UsageLimits {
	if limits, ok := x.config.GradeLimits[grade]; ok {
		return limits.Usage
	}
	return x.config.GradeLimits[platform.GradeBronze].Usage
}

func (x *UsageLimiter) getUsageKey(usageType UsageType, period string, userID int64) string {
	now := time.Now()
	var timePart string
	switch period {
	case "daily":
		timePart = now.Format("2006-01-02")
	case "monthly":
		timePart = now.Format("2006-01")
	}
	return fmt.Sprintf("usage:%s:%s:%d:%s", usageType, period, userID, timePart)
}

func (x *UsageLimiter) checkAndIncrement(
	logger *tracing.Logger,
	userID int64,
	userGrade platform.UserGrade,
	usageType UsageType,
) (*LimitCheckResult, error) {
	limits := x.getUsageLimits(userGrade)

	var dailyLimit, monthlyLimit int
	switch usageType {
	case UsageTypeVision:
		dailyLimit = limits.VisionDaily
		monthlyLimit = limits.VisionMonthly
	case UsageTypeDialer:
		dailyLimit = limits.DialerDaily
		monthlyLimit = limits.DialerMonthly
	case UsageTypeWhisper:
		dailyLimit = limits.WhisperDaily
		monthlyLimit = limits.WhisperMonthly
	}

	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 5*time.Second)
	defer cancel()

	monthlyKey := x.getUsageKey(usageType, "monthly", userID)
	monthlyCount, err := x.redis.Get(ctx, monthlyKey).Int()
	if err != nil && err != redis.Nil {
		logger.E("Failed to get monthly usage from Redis", "key", monthlyKey, tracing.InnerError, err)
		return nil, err
	}
	
	monthlyUsagePercent := 0
	monthlyRemaining := monthlyLimit - monthlyCount
	if monthlyLimit > 0 {
		monthlyUsagePercent = int(float64(monthlyCount) / float64(monthlyLimit) * 100)
	}
	
	if monthlyCount >= monthlyLimit {
		logger.I("usage_limit_exceeded",
			"user_id", userID,
			"user_grade", userGrade,
			"usage_type", usageType,
			"limit_type", "monthly",
			"current_usage", monthlyCount,
			"limit", monthlyLimit,
			"usage_percent", monthlyUsagePercent,
		)
		return &LimitCheckResult{Exceeded: true, IsDaily: false}, nil
	}

	dailyKey := x.getUsageKey(usageType, "daily", userID)
	dailyCount, err := x.redis.Get(ctx, dailyKey).Int()
	if err != nil && err != redis.Nil {
		logger.E("Failed to get daily usage from Redis", "key", dailyKey, tracing.InnerError, err)
		return nil, err
	}
	
	dailyUsagePercent := 0
	dailyRemaining := dailyLimit - dailyCount
	if dailyLimit > 0 {
		dailyUsagePercent = int(float64(dailyCount) / float64(dailyLimit) * 100)
	}
	
	if dailyCount >= dailyLimit {
		logger.I("usage_limit_exceeded",
			"user_id", userID,
			"user_grade", userGrade,
			"usage_type", usageType,
			"limit_type", "daily",
			"current_usage", dailyCount,
			"limit", dailyLimit,
			"usage_percent", dailyUsagePercent,
		)
		return &LimitCheckResult{Exceeded: true, IsDaily: true}, nil
	}

	// Increment both counts
	pipe := x.redis.TxPipeline()
	pipe.Incr(ctx, dailyKey)
	pipe.Incr(ctx, monthlyKey)
	// Set TTL for keys to expire shortly after the period ends to save space
	pipe.Expire(ctx, dailyKey, 25*time.Hour)
	pipe.Expire(ctx, monthlyKey, 32*24*time.Hour)

	if _, err := pipe.Exec(ctx); err != nil {
		logger.E("Failed to increment usage in Redis", tracing.InnerError, err)
		return nil, err
	}

	dailyCount++
	monthlyCount++
	dailyUsagePercent = int(float64(dailyCount) / float64(dailyLimit) * 100)
	monthlyUsagePercent = int(float64(monthlyCount) / float64(monthlyLimit) * 100)
	dailyRemaining = dailyLimit - dailyCount
	monthlyRemaining = monthlyLimit - monthlyCount

	logger.I("usage_check_success",
		"user_id", userID,
		"user_grade", userGrade,
		"usage_type", usageType,
		"daily_usage", dailyCount,
		"daily_limit", dailyLimit,
		"daily_remaining", dailyRemaining,
		"daily_usage_percent", dailyUsagePercent,
		"monthly_usage", monthlyCount,
		"monthly_limit", monthlyLimit,
		"monthly_remaining", monthlyRemaining,
		"monthly_usage_percent", monthlyUsagePercent,
	)

	return &LimitCheckResult{Exceeded: false}, nil
}