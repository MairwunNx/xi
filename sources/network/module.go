package network

import "go.uber.org/fx"

var Module = fx.Module("network", fx.Provide(NewProxyConfig, NewProxyDialer, NewProxyClient))