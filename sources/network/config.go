package network

import (
	"time"
	"ximanager/sources/platform"
)

type ProxyConfig struct {
	ProxyAddress   string
	ProxyUser      string
	ProxyPass      string
	TimeoutSeconds time.Duration
}

func NewProxyConfig() *ProxyConfig {
	return &ProxyConfig{
		ProxyAddress:   platform.Get("PROXY_ADDRESS", "localhost:9050"),
		ProxyUser:      platform.Get("PROXY_USER", "admin"),
		ProxyPass:      platform.Get("PROXY_PASS", "admin"),
		TimeoutSeconds: platform.GetAsDuration("HTTP_TIMEOUT_SECONDS", "5m"),
	}
}