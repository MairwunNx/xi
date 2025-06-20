package network

import "os"

type ProxyConfig struct {
	ProxyAddress string
	ProxyUser    string
	ProxyPass    string
}

func NewProxyConfig() *ProxyConfig {
	return &ProxyConfig{
		ProxyAddress: getEnv("PROXY_ADDRESS", "localhost:9050"),
		ProxyUser:    getEnv("PROXY_USER", "admin"),
		ProxyPass:    getEnv("PROXY_PASS", "admin"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}