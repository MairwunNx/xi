package artificial

import (
	"fmt"
	"ximanager/sources/persistence/entities"
	"ximanager/sources/repository"
	"ximanager/sources/tracing"

	"github.com/dustin/go-humanize"
	"github.com/shopspring/decimal"
)

type LimitType string

const (
	LimitTypeDaily   LimitType = "daily"
	LimitTypeMonthly LimitType = "monthly"
)

type SpendingLimitExceededError struct {
	UserGrade    string
	LimitType    LimitType
	LimitAmount  string
	CurrentSpend string
}

func (e *SpendingLimitExceededError) Error() string {
	return fmt.Sprintf("Spending limit exceeded: %s %s, current spend: %s, limit: %s", e.LimitType, e.UserGrade, e.CurrentSpend, e.LimitAmount)
}

func IsSpendingLimitExceeded(err error) bool {
	_, ok := err.(*SpendingLimitExceededError)
	return ok
}

type SpendingLimiter struct {
	config    *AIConfig
	donations *repository.DonationsRepository
	usage     *repository.UsageRepository
}

func NewSpendingLimiter(config *AIConfig, donations *repository.DonationsRepository, usage *repository.UsageRepository) *SpendingLimiter {
	return &SpendingLimiter{
		config:    config,
		donations: donations,
		usage:     usage,
	}
}

func (s *SpendingLimiter) CheckSpendingLimits(logger *tracing.Logger, user *entities.User) error {
	userGrade, err := s.donations.GetUserGrade(logger, user)
	if err != nil {
		logger.E("Failed to get user grade, defaulting to bronze", tracing.InnerError, err)
		userGrade = repository.UserGradeBronze
	}

	dailyLimit, monthlyLimit := s.getLimitsForGrade(userGrade)

	dailySpent, err := s.usage.GetUserDailyCost(logger, user)
	if err != nil {
		logger.E("Failed to get user daily cost", tracing.InnerError, err)
		return nil
	}

	if dailySpent.GreaterThanOrEqual(dailyLimit) {
		return &SpendingLimitExceededError{
			UserGrade:    s.getGradeDisplayName(userGrade),
			LimitType:    LimitTypeDaily,
			LimitAmount:  "$" + humanize.Comma(dailyLimit.IntPart()),
			CurrentSpend: "$" + humanize.Comma(dailySpent.IntPart()),
		}
	}

	monthlySpent, err := s.usage.GetUserMonthlyCost(logger, user)
	if err != nil {
		logger.E("Failed to get user monthly cost", tracing.InnerError, err)
		return nil
	}

	if monthlySpent.GreaterThanOrEqual(monthlyLimit) {
		return &SpendingLimitExceededError{
			UserGrade:    s.getGradeDisplayName(userGrade),
			LimitType:    LimitTypeMonthly,
			LimitAmount:  "$" + humanize.Comma(monthlyLimit.IntPart()),
			CurrentSpend: "$" + humanize.Comma(monthlySpent.IntPart()),
		}
	}

	logger.I("Spending limits check passed", "user_grade", userGrade, "daily_spent", dailySpent.String(), "daily_limit", dailyLimit.String(), "monthly_spent", monthlySpent.String(), "monthly_limit", monthlyLimit.String())
	return nil
}

func (s *SpendingLimiter) getLimitsForGrade(grade repository.UserGrade) (daily, monthly decimal.Decimal) {
	switch grade {
	case repository.UserGradeBronze:
		return s.config.SpendingLimits.BronzeDailyLimit, s.config.SpendingLimits.BronzeMonthlyLimit
	case repository.UserGradeSilver:
		return s.config.SpendingLimits.SilverDailyLimit, s.config.SpendingLimits.SilverMonthlyLimit
	case repository.UserGradeGold:
		return s.config.SpendingLimits.GoldDailyLimit, s.config.SpendingLimits.GoldMonthlyLimit
	default:
		return s.config.SpendingLimits.BronzeDailyLimit, s.config.SpendingLimits.BronzeMonthlyLimit
	}
}

func (s *SpendingLimiter) getGradeDisplayName(grade repository.UserGrade) string {
	switch grade {
	case repository.UserGradeBronze:
		return "бронзовый"
	case repository.UserGradeSilver:
		return "серебряный"
	case repository.UserGradeGold:
		return "золотой"
	default:
		return "базовый"
	}
} 