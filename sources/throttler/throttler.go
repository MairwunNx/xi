package throttler

import (
	"context"
	"fmt"
	"time"
	"ximanager/sources/platform"
	"ximanager/sources/tracing"

	"github.com/redis/go-redis/v9"
)

type Throttler struct {
	client *redis.Client
	config *ThrottlerConfig
	log    *tracing.Logger
	ctx    context.Context
}

func NewThrottler(client *redis.Client, config *ThrottlerConfig, log *tracing.Logger) *Throttler {
	ctx := context.Background()
	return &Throttler{client: client, config: config, log: log, ctx: ctx}
}

func (x *Throttler) IsAllowed(userId int64) bool {
	ctx, cancel := platform.ContextTimeout(x.ctx)
	defer cancel()

	key := fmt.Sprintf("throttle:%d", userId)

	success, err := x.client.SetNX(ctx, key, time.Now().Unix(), x.config.Limit).Result()
	if err != nil {
		x.log.E("Error setting throttle key", tracing.InnerError, err)
		return true
	}

	return success
}
