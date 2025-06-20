package persistence

import (
	"context"
	"ximanager/sources/persistence/gormdao/query"
	"ximanager/sources/tracing"

	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

var Module = fx.Module("persistence",
	fx.Provide(
		NewDatabaseConfig, NewPostgresDatabase,
		NewRedisConfig, NewRedis,
	),

	fx.Invoke(func(db *gorm.DB, redis *redis.Client, lc fx.Lifecycle, log *tracing.Logger) {
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				query.SetDefault(db)
				log.I("Query initialized successfully")

				if sqlDB, err := db.DB(); err != nil {
					log.F("Failed to get underlying sql.DB", tracing.InnerError, err)
				} else if err := sqlDB.PingContext(ctx); err != nil {
					log.F("Failed to ping PostgreSQL", tracing.InnerError, err)
				} else {
					log.I("PostgreSQL connection verified")
				}

				if err := redis.Ping(ctx).Err(); err != nil {
					log.F("Failed to ping Redis", tracing.InnerError, err)
				} else {
					log.I("Redis connection verified")
				}

				return nil
			},
			OnStop: func(ctx context.Context) error {
				log.I("Closing database connections")

				if sqlDB, err := db.DB(); err == nil {
					sqlDB.Close()
				} else {
					log.E("Failed to close PostgreSQL", tracing.InnerError, err)
				}

				redis.Close()

				return nil
			},
		})
	}),
)
