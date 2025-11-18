package network

import "go.uber.org/fx"

var Module = fx.Module("network", fx.Provide(NewProxyDialer, NewProxyClient))