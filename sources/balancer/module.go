package balancer

import "go.uber.org/fx"

var Module = fx.Module("balancer",
	fx.Provide(NewAIBalancerConfig, NewAIBalancer),
) 