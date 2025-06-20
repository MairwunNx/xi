package throttler

import (
	"time"
	"ximanager/sources/platform"
)

type ThrottlerConfig struct {
	Limit time.Duration
}

func NewThrottlerConfig() *ThrottlerConfig {
	return &ThrottlerConfig{Limit: platform.GetAsDuration("REQUEST_THROTTLE_LIMIT", "5s")}
}