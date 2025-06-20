package external

import (
	"os"
	"strconv"
)

type OutsidersConfig struct {
	StartupPort int
	MetricsPort int
}

func NewOutsidersConfig() *OutsidersConfig {
	return &OutsidersConfig{
		StartupPort: getEnvAsInt("OUTSIDERS_STARTUP_PORT", 10000),
		MetricsPort: getEnvAsInt("OUTSIDERS_METRICS_PORT", 10001),
	}
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return defaultValue
} 