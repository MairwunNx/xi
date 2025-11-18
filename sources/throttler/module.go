package throttler

import "go.uber.org/fx"

var Module = fx.Module("throttler", fx.Provide(NewThrottler))