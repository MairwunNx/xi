package platform

import (
	"context"
	"time"
)

func ContextTimeoutVal(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, timeout)
}