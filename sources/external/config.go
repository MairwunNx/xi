package external

import (
	"os"
	"strconv"
)

type OutsidersConfig struct {
	StartupPort          int
	SystemMetricsPort   int
	ApplicationMetricsPort int
}

func NewOutsidersConfig() *OutsidersConfig {
	return &OutsidersConfig{
		StartupPort:          getEnvAsInt("OUTSIDERS_STARTUP_PORT", 10000),
		SystemMetricsPort:   getEnvAsInt("OUTSIDERS_SYSTEM_METRICS_PORT", 9090),
		ApplicationMetricsPort: getEnvAsInt("OUTSIDERS_APPLICATION_METRICS_PORT", 9091),
	}
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return defaultValue
}
