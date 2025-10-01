package persistence

import (
	"strconv"
	"ximanager/sources/tracing"

	"github.com/redis/go-redis/v9"
)

func NewRedis(config *RedisConfig, log *tracing.Logger) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:                  config.Host + ":" + strconv.Itoa(config.Port),
		Password:              config.Password,
		DB:                    config.DB,
		MaxRetries:            config.MaxRetries,
		DialTimeout:           config.DialTimeout,
		ContextTimeoutEnabled: true,
	})

	log.I("Redis client initialized successfully")

	return rdb
}
