package network

import (
	"ximanager/sources/configuration"
	"ximanager/sources/tracing"

	"golang.org/x/net/proxy"
)

func NewProxyDialer(config *configuration.Config, log *tracing.Logger) proxy.Dialer {
	dialer, err := proxy.SOCKS5(
		"tcp",
		config.Proxy.URL,
		&proxy.Auth{User: config.Proxy.User, Password: config.Proxy.Password},
		proxy.Direct,
	)

	if err != nil {
		log.F("Failed to create proxy dialer", tracing.InnerError, err)
	}

	return dialer
}