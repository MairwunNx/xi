package repository

import (
	"context"
	"time"
	"ximanager/sources/persistence/gormdao/query"
	"ximanager/sources/platform"
	"ximanager/sources/tracing"

	"github.com/redis/go-redis/v9"
)

type HealthRepository struct {
	redis *redis.Client
}

func NewHealthRepository(redis *redis.Client) *HealthRepository {
	return &HealthRepository{
		redis: redis,
	}
}

func (x *HealthRepository) CheckDatabaseHealth(logger *tracing.Logger) error {
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 1*time.Second)
	defer cancel()

	q := query.Q.WithContext(ctx)
	_, err := q.User.Limit(1).Find()
	if err != nil {
		logger.E("Database health check failed", tracing.InnerError, err)
		return err
	}

	logger.I("Database health check passed")
	return nil
}

func (x *HealthRepository) CheckRedisHealth(logger *tracing.Logger) error {
	ctx, cancel := platform.ContextTimeoutVal(context.Background(), 1*time.Second)
	defer cancel()

	err := x.redis.Ping(ctx).Err()
	if err != nil {
		logger.E("Redis health check failed", tracing.InnerError, err)
		return err
	}

	logger.I("Redis health check passed")
	return nil
}