package repository

import (
	"context"
	"time"
	"ximanager/sources/persistence/entities"
	"ximanager/sources/persistence/gormdao/query"
	"ximanager/sources/platform"
	"ximanager/sources/tracing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type UsageRepository struct{}

func NewUsageRepository() *UsageRepository {
	return &UsageRepository{}
}

func (x *UsageRepository) SaveUsage(logger *tracing.Logger, userID uuid.UUID, chatID int64, cost decimal.Decimal, tokens int, cacheReadTokens int, cacheWriteTokens int, anotherCost decimal.Decimal, anotherTokens int) error {
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	usage := &entities.Usage{
		UserID:           userID,
		ChatID:           chatID,
		Cost:             cost,
		Tokens:           tokens,
		CacheReadTokens:  cacheReadTokens,
		CacheWriteTokens: cacheWriteTokens,
	}

	if !anotherCost.IsZero() {
		usage.AnotherCost = &anotherCost
	}

	if anotherTokens > 0 {
		usage.AnotherTokens = &anotherTokens
	}

	q := query.Q.WithContext(ctx)
	err := q.Usage.Create(usage)
	if err != nil {
		logger.E("Failed to save usage", tracing.InnerError, err)
		return err
	}

	logger.I("Usage saved", "cost", cost, "tokens", tokens, "cache_read", cacheReadTokens, "cache_write", cacheWriteTokens, "another_cost", anotherCost, "another_tokens", anotherTokens)
	return nil
}

func (x *UsageRepository) GetTotalCost(logger *tracing.Logger) (decimal.Decimal, error) {
	defer tracing.ProfilePoint(logger, "Usage get total cost completed", "repository.usage.get.total.cost")()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	var totalCost *decimal.Decimal
	q := query.Q.WithContext(ctx)

	err := q.Usage.
		Select(query.Usage.Cost.Sum()).
		Row().Scan(&totalCost)

	if err != nil {
		logger.E("Failed to get total cost", tracing.InnerError, err)
		return decimal.Zero, err
	}

	if totalCost == nil {
		return decimal.Zero, nil
	}

	return *totalCost, nil
}

func (x *UsageRepository) GetTotalCostLastMonth(logger *tracing.Logger) (decimal.Decimal, error) {
	defer tracing.ProfilePoint(logger, "Usage get total cost last month completed", "repository.usage.get.total.cost.last.month")()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	lastMonth := time.Now().AddDate(0, -1, 0)
	var totalCost *decimal.Decimal
	q := query.Q.WithContext(ctx)

	err := q.Usage.
		Where(query.Usage.CreatedAt.Gte(lastMonth)).
		Select(query.Usage.Cost.Sum()).
		Row().Scan(&totalCost)

	if err != nil {
		logger.E("Failed to get total cost for last month", tracing.InnerError, err)
		return decimal.Zero, err
	}

	if totalCost == nil {
		return decimal.Zero, nil
	}

	return *totalCost, nil
}

func (x *UsageRepository) GetUserCost(logger *tracing.Logger, user *entities.User) (decimal.Decimal, error) {
	defer tracing.ProfilePoint(logger, "Usage get user cost completed", "repository.usage.get.user.cost", "user_id", user.ID)()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	var totalCost *decimal.Decimal
	q := query.Q.WithContext(ctx)

	err := q.Usage.
		Where(query.Usage.UserID.Eq(user.ID)).
		Select(query.Usage.Cost.Sum()).
		Row().Scan(&totalCost)

	if err != nil {
		logger.E("Failed to get user cost", tracing.InnerError, err)
		return decimal.Zero, err
	}

	if totalCost == nil {
		return decimal.Zero, nil
	}

	return *totalCost, nil
}

func (x *UsageRepository) GetUserCostLastMonth(logger *tracing.Logger, user *entities.User) (decimal.Decimal, error) {
	defer tracing.ProfilePoint(logger, "Usage get user cost last month completed", "repository.usage.get.user.cost.last.month", "user_id", user.ID)()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	lastMonth := time.Now().AddDate(0, -1, 0)
	var totalCost *decimal.Decimal
	q := query.Q.WithContext(ctx)

	err := q.Usage.
		Where(query.Usage.UserID.Eq(user.ID)).
		Where(query.Usage.CreatedAt.Gte(lastMonth)).
		Select(query.Usage.Cost.Sum()).
		Row().Scan(&totalCost)

	if err != nil {
		logger.E("Failed to get user cost for last month", tracing.InnerError, err)
		return decimal.Zero, err
	}

	if totalCost == nil {
		return decimal.Zero, nil
	}

	return *totalCost, nil
}

func (x *UsageRepository) GetUserCostSince(logger *tracing.Logger, user *entities.User, since time.Time) (decimal.Decimal, error) {
	defer tracing.ProfilePoint(logger, "Usage get user cost since completed", "repository.usage.get.user.cost.since", "user_id", user.ID)()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	var totalCost *decimal.Decimal
	q := query.Q.WithContext(ctx)

	err := q.Usage.
		Where(query.Usage.UserID.Eq(user.ID)).
		Where(query.Usage.CreatedAt.Gte(since)).
		Select(query.Usage.Cost.Sum()).
		Row().Scan(&totalCost)

	if err != nil {
		logger.E("Failed to get user cost since", "since", since, tracing.InnerError, err)
		return decimal.Zero, err
	}

	if totalCost == nil {
		return decimal.Zero, nil
	}

	return *totalCost, nil
}

func (x *UsageRepository) GetTotalTokens(logger *tracing.Logger) (int64, error) {
	defer tracing.ProfilePoint(logger, "Usage get total tokens completed", "repository.usage.get.total.tokens")()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	var totalTokens *int64
	q := query.Q.WithContext(ctx)

	err := q.Usage.
		Select(query.Usage.Tokens.Sum()).
		Row().Scan(&totalTokens)

	if err != nil {
		logger.E("Failed to get total tokens", tracing.InnerError, err)
		return 0, err
	}

	if totalTokens == nil {
		return 0, nil
	}

	return *totalTokens, nil
}

func (x *UsageRepository) GetTotalTokensLastMonth(logger *tracing.Logger) (int64, error) {
	defer tracing.ProfilePoint(logger, "Usage get total tokens last month completed", "repository.usage.get.total.tokens.last.month")()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	lastMonth := time.Now().AddDate(0, -1, 0)
	var totalTokens *int64
	q := query.Q.WithContext(ctx)

	err := q.Usage.
		Where(query.Usage.CreatedAt.Gte(lastMonth)).
		Select(query.Usage.Tokens.Sum()).
		Row().Scan(&totalTokens)

	if err != nil {
		logger.E("Failed to get total tokens for last month", tracing.InnerError, err)
		return 0, err
	}

	if totalTokens == nil {
		return 0, nil
	}

	return *totalTokens, nil
}

func (x *UsageRepository) GetUserTokens(logger *tracing.Logger, user *entities.User) (int64, error) {
	defer tracing.ProfilePoint(logger, "Usage get user tokens completed", "repository.usage.get.user.tokens", "user_id", user.ID)()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	var totalTokens *int64
	q := query.Q.WithContext(ctx)

	err := q.Usage.
		Where(query.Usage.UserID.Eq(user.ID)).
		Select(query.Usage.Tokens.Sum()).
		Row().Scan(&totalTokens)

	if err != nil {
		logger.E("Failed to get user tokens", tracing.InnerError, err)
		return 0, err
	}

	if totalTokens == nil {
		return 0, nil
	}

	return *totalTokens, nil
}

func (x *UsageRepository) GetUserTokensLastMonth(logger *tracing.Logger, user *entities.User) (int64, error) {
	defer tracing.ProfilePoint(logger, "Usage get user tokens last month completed", "repository.usage.get.user.tokens.last.month", "user_id", user.ID)()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	lastMonth := time.Now().AddDate(0, -1, 0)
	var totalTokens *int64
	q := query.Q.WithContext(ctx)

	err := q.Usage.
		Where(query.Usage.UserID.Eq(user.ID)).
		Where(query.Usage.CreatedAt.Gte(lastMonth)).
		Select(query.Usage.Tokens.Sum()).
		Row().Scan(&totalTokens)

	if err != nil {
		logger.E("Failed to get user tokens for last month", tracing.InnerError, err)
		return 0, err
	}

	if totalTokens == nil {
		return 0, nil
	}

	return *totalTokens, nil
}

func (x *UsageRepository) GetAverageDailyCost(logger *tracing.Logger) (decimal.Decimal, error) {
	defer tracing.ProfilePoint(logger, "Usage get average daily cost completed", "repository.usage.get.average.daily.cost")()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Q.WithContext(ctx)

	firstUsage, err := q.Usage.
		Order(query.Usage.CreatedAt).
		First()

	if err != nil || firstUsage == nil {
		logger.E("Failed to get first usage record", tracing.InnerError, err)
		return decimal.Zero, nil
	}

	daysSince := time.Since(firstUsage.CreatedAt).Hours() / 24
	if daysSince < 1 {
		daysSince = 1
	}

	totalCost, err := x.GetTotalCost(logger)
	if err != nil {
		return decimal.Zero, err
	}

	avgDailyCost := totalCost.Div(decimal.NewFromFloat(daysSince))

	return avgDailyCost, nil
}

func (x *UsageRepository) GetUserAverageDailyCost(logger *tracing.Logger, user *entities.User) (decimal.Decimal, error) {
	defer tracing.ProfilePoint(logger, "Usage get user average daily cost completed", "repository.usage.get.user.average.daily.cost", "user_id", user.ID)()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	q := query.Q.WithContext(ctx)

	firstUsage, err := q.Usage.
		Where(query.Usage.UserID.Eq(user.ID)).
		Order(query.Usage.CreatedAt).
		First()

	if err != nil || firstUsage == nil {
		logger.E("Failed to get first user usage record", tracing.InnerError, err)
		return decimal.Zero, nil
	}

	daysSince := time.Since(firstUsage.CreatedAt).Hours() / 24
	if daysSince < 1 {
		daysSince = 1
	}

	totalCost, err := x.GetUserCost(logger, user)
	if err != nil {
		return decimal.Zero, err
	}

	avgDailyCost := totalCost.Div(decimal.NewFromFloat(daysSince))

	return avgDailyCost, nil
}

func (x *UsageRepository) GetUserDailyCost(logger *tracing.Logger, user *entities.User) (decimal.Decimal, error) {
	defer tracing.ProfilePoint(logger, "Usage get user daily cost completed", "repository.usage.get.user.daily.cost", "user_id", user.ID)()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	var totalCost *decimal.Decimal
	q := query.Q.WithContext(ctx)

	err := q.Usage.
		Where(query.Usage.UserID.Eq(user.ID)).
		Where(query.Usage.CreatedAt.Gte(startOfDay)).
		Select(query.Usage.Cost.Sum()).
		Row().Scan(&totalCost)

	if err != nil {
		logger.E("Failed to get user daily cost", tracing.InnerError, err)
		return decimal.Zero, err
	}

	if totalCost == nil {
		return decimal.Zero, nil
	}

	return *totalCost, nil
}

func (x *UsageRepository) GetUserMonthlyCost(logger *tracing.Logger, user *entities.User) (decimal.Decimal, error) {
	defer tracing.ProfilePoint(logger, "Usage get user monthly cost completed", "repository.usage.get.user.monthly.cost", "user_id", user.ID)()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	var totalCost *decimal.Decimal
	q := query.Q.WithContext(ctx)

	err := q.Usage.
		Where(query.Usage.UserID.Eq(user.ID)).
		Where(query.Usage.CreatedAt.Gte(startOfMonth)).
		Select(query.Usage.Cost.Sum()).
		Row().Scan(&totalCost)

	if err != nil {
		logger.E("Failed to get user monthly cost", tracing.InnerError, err)
		return decimal.Zero, err
	}

	if totalCost == nil {
		return decimal.Zero, nil
	}

	return *totalCost, nil
}

func (x *UsageRepository) GetTotalAnotherCost(logger *tracing.Logger) (decimal.Decimal, error) {
	defer tracing.ProfilePoint(logger, "Usage get total another cost completed", "repository.usage.get.total.another.cost")()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	var totalCost *decimal.Decimal
	q := query.Q.WithContext(ctx)

	err := q.Usage.
		Select(query.Usage.AnotherCost.Sum()).
		Row().Scan(&totalCost)

	if err != nil {
		logger.E("Failed to get total another cost", tracing.InnerError, err)
		return decimal.Zero, err
	}

	if totalCost == nil {
		return decimal.Zero, nil
	}

	return *totalCost, nil
}

func (x *UsageRepository) GetTotalAnotherCostLastMonth(logger *tracing.Logger) (decimal.Decimal, error) {
	defer tracing.ProfilePoint(logger, "Usage get total another cost last month completed", "repository.usage.get.total.another.cost.last.month")()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	lastMonth := time.Now().AddDate(0, -1, 0)
	var totalCost *decimal.Decimal
	q := query.Q.WithContext(ctx)

	err := q.Usage.
		Where(query.Usage.CreatedAt.Gte(lastMonth)).
		Select(query.Usage.AnotherCost.Sum()).
		Row().Scan(&totalCost)

	if err != nil {
		logger.E("Failed to get total another cost for last month", tracing.InnerError, err)
		return decimal.Zero, err
	}

	if totalCost == nil {
		return decimal.Zero, nil
	}

	return *totalCost, nil
}

func (x *UsageRepository) GetUserAnotherCost(logger *tracing.Logger, user *entities.User) (decimal.Decimal, error) {
	defer tracing.ProfilePoint(logger, "Usage get user another cost completed", "repository.usage.get.user.another.cost", "user_id", user.ID)()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	var totalCost *decimal.Decimal
	q := query.Q.WithContext(ctx)

	err := q.Usage.
		Where(query.Usage.UserID.Eq(user.ID)).
		Select(query.Usage.AnotherCost.Sum()).
		Row().Scan(&totalCost)

	if err != nil {
		logger.E("Failed to get user another cost", tracing.InnerError, err)
		return decimal.Zero, err
	}

	if totalCost == nil {
		return decimal.Zero, nil
	}

	return *totalCost, nil
}

func (x *UsageRepository) GetUserAnotherCostLastMonth(logger *tracing.Logger, user *entities.User) (decimal.Decimal, error) {
	defer tracing.ProfilePoint(logger, "Usage get user another cost last month completed", "repository.usage.get.user.another.cost.last.month", "user_id", user.ID)()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	lastMonth := time.Now().AddDate(0, -1, 0)
	var totalCost *decimal.Decimal
	q := query.Q.WithContext(ctx)

	err := q.Usage.
		Where(query.Usage.UserID.Eq(user.ID)).
		Where(query.Usage.CreatedAt.Gte(lastMonth)).
		Select(query.Usage.AnotherCost.Sum()).
		Row().Scan(&totalCost)

	if err != nil {
		logger.E("Failed to get user another cost for last month", tracing.InnerError, err)
		return decimal.Zero, err
	}

	if totalCost == nil {
		return decimal.Zero, nil
	}

	return *totalCost, nil
}

func (x *UsageRepository) GetTotalAnotherTokens(logger *tracing.Logger) (int64, error) {
	defer tracing.ProfilePoint(logger, "Usage get total another tokens completed", "repository.usage.get.total.another.tokens")()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	var totalTokens *int64
	q := query.Q.WithContext(ctx)

	err := q.Usage.
		Select(query.Usage.AnotherTokens.Sum()).
		Row().Scan(&totalTokens)

	if err != nil {
		logger.E("Failed to get total another tokens", tracing.InnerError, err)
		return 0, err
	}

	if totalTokens == nil {
		return 0, nil
	}

	return *totalTokens, nil
}

func (x *UsageRepository) GetTotalAnotherTokensLastMonth(logger *tracing.Logger) (int64, error) {
	defer tracing.ProfilePoint(logger, "Usage get total another tokens last month completed", "repository.usage.get.total.another.tokens.last.month")()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	lastMonth := time.Now().AddDate(0, -1, 0)
	var totalTokens *int64
	q := query.Q.WithContext(ctx)

	err := q.Usage.
		Where(query.Usage.CreatedAt.Gte(lastMonth)).
		Select(query.Usage.AnotherTokens.Sum()).
		Row().Scan(&totalTokens)

	if err != nil {
		logger.E("Failed to get total another tokens for last month", tracing.InnerError, err)
		return 0, err
	}

	if totalTokens == nil {
		return 0, nil
	}

	return *totalTokens, nil
}

func (x *UsageRepository) GetUserAnotherTokens(logger *tracing.Logger, user *entities.User) (int64, error) {
	defer tracing.ProfilePoint(logger, "Usage get user another tokens completed", "repository.usage.get.user.another.tokens", "user_id", user.ID)()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	var totalTokens *int64
	q := query.Q.WithContext(ctx)

	err := q.Usage.
		Where(query.Usage.UserID.Eq(user.ID)).
		Select(query.Usage.AnotherTokens.Sum()).
		Row().Scan(&totalTokens)

	if err != nil {
		logger.E("Failed to get user another tokens", tracing.InnerError, err)
		return 0, err
	}

	if totalTokens == nil {
		return 0, nil
	}

	return *totalTokens, nil
}

func (x *UsageRepository) GetUserAnotherTokensLastMonth(logger *tracing.Logger, user *entities.User) (int64, error) {
	defer tracing.ProfilePoint(logger, "Usage get user another tokens last month completed", "repository.usage.get.user.another.tokens.last.month", "user_id", user.ID)()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	lastMonth := time.Now().AddDate(0, -1, 0)
	var totalTokens *int64
	q := query.Q.WithContext(ctx)

	err := q.Usage.
		Where(query.Usage.UserID.Eq(user.ID)).
		Where(query.Usage.CreatedAt.Gte(lastMonth)).
		Select(query.Usage.AnotherTokens.Sum()).
		Row().Scan(&totalTokens)

	if err != nil {
		logger.E("Failed to get user another tokens for last month", tracing.InnerError, err)
		return 0, err
	}

	if totalTokens == nil {
		return 0, nil
	}

	return *totalTokens, nil
}

func (x *UsageRepository) GetActiveUsersCount(logger *tracing.Logger, since time.Time) (int64, error) {
	defer tracing.ProfilePoint(logger, "Usage get active users count completed", "repository.usage.get.active.users.count")()
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 20*time.Second)
	defer cancel()

	var count int64
	q := query.Q.WithContext(ctx)

	count, err := q.Usage.
		Where(query.Usage.CreatedAt.Gte(since)).
		Select(query.Usage.UserID).
		Distinct().
		Count()

	if err != nil {
		logger.E("Failed to get active users count", tracing.InnerError, err)
		return 0, err
	}

	return count, nil
}
