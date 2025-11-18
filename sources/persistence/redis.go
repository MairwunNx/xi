package persistence

import (
	"strconv"
	"ximanager/sources/configuration"
	"ximanager/sources/tracing"

	"github.com/redis/go-redis/v9"
)

func NewRedis(config *configuration.Config, log *tracing.Logger) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:                  config.Redis.Host + ":" + strconv.Itoa(config.Redis.Port),
		Password:              config.Redis.Password,
		DB:                    config.Redis.DB,
		MaxRetries:            config.Redis.MaxRetries,
		DialTimeout:           config.Redis.DialTimeout,
		ContextTimeoutEnabled: true,
	})

	log.I("Redis client initialized successfully")
	return rdb
}