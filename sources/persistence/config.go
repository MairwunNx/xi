package persistence

import (
	"time"
	"ximanager/sources/platform"
)

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
	TimeZone string
}

type RedisConfig struct {
	Host        string
	Port        int
	Password    string
	DB          int
	MaxRetries  int
	DialTimeout time.Duration
}

func NewDatabaseConfig() *DatabaseConfig {
	return &DatabaseConfig{
		Host:     platform.Get("POSTGRES_HOST", "localhost"),
		Port:     platform.Get("POSTGRES_PORT", "5432"),
		User:     platform.Get("POSTGRES_USER", "postgres"),
		Password: platform.Get("POSTGRES_PASSWORD", "password"),
		DBName:   platform.Get("POSTGRES_DB", "ximanager"),
		SSLMode:  platform.Get("POSTGRES_SSLMODE", "disable"),
		TimeZone: platform.Get("POSTGRES_TIMEZONE", "UTC"),
	}
}

func NewRedisConfig() *RedisConfig {
	return &RedisConfig{
		Host:        platform.Get("REDIS_HOST", "redis"),
		Port:        platform.GetAsInt("REDIS_PORT", 6379),
		Password:    platform.Get("REDIS_PASSWORD", ""),
		DB:          platform.GetAsInt("REDIS_DB", 0),
		MaxRetries:  platform.GetAsInt("REDIS_MAX_RETRIES", 5),
		DialTimeout: platform.GetAsDuration("REDIS_DIAL_TIMEOUT", "5s"),
	}
}