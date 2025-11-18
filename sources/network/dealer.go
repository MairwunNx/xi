package network

import (
	"ximanager/sources/tracing"

	"golang.org/x/net/proxy"
)

func NewProxyDialer(config *ProxyConfig, log *tracing.Logger) proxy.Dialer {
	dialer, err := proxy.SOCKS5(
		"tcp",
		config.ProxyAddress,
		&proxy.Auth{User: config.ProxyUser, Password: config.ProxyPass},
		proxy.Direct,
	)

	if err != nil {
		log.F("Failed to create proxy dialer", tracing.InnerError, err)
	}

	return dialer
}