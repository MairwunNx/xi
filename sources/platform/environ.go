package platform

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
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

func GetAsBool(key string, defaultValue bool) bool {
	if valueStr := os.Getenv(key); valueStr != "" {
		if value, err := strconv.ParseBool(valueStr); err == nil {
			return value
		}
	}
	return defaultValue
}

func GetDecimal(key, defaultValue string) decimal.Decimal {
	if valueStr := Get(key, defaultValue); valueStr != "" {
		if value, err := decimal.NewFromString(valueStr); err == nil {
			return value
		}
	}
	d, _ := decimal.NewFromString(defaultValue)
	return d
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