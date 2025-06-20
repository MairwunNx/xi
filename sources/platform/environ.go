package platform

import (
	"os"
	"strconv"
	"strings"
	"time"
)

func Get(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func GetAsInt(key string, defaultValue int) int {
	if valueStr := os.Getenv(key); valueStr != "" {
		if value, err := strconv.Atoi(valueStr); err == nil {
			return value
		}
	}
	return defaultValue
}

func GetAsSlice(key string, defaultValue []string) []string {
	if valueStr := os.Getenv(key); valueStr != "" {
		return strings.Split(valueStr, ",")
	}
	return defaultValue
}

func GetAsDuration(key, defaultValue string) time.Duration {
	if valueStr := os.Getenv(key); valueStr != "" {
		if value, err := time.ParseDuration(valueStr); err == nil {
			return value
		}
	}
	if duration, err := time.ParseDuration(defaultValue); err == nil {
		return duration
	}
	return 5 * time.Second
}