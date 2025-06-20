package platform

import (
	"context"
	"time"
)

var defaultTimeout = GetAsDuration("CONTEXT_TIMEOUT", "5s")

func ContextTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, defaultTimeout)
}

func ContextTimeoutVal(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, timeout)
}