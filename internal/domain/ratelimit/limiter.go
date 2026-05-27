package ratelimit

import (
	"context"
	"time"
)

type Result struct {
	Allowed   bool
	Remaining int
	ResetAt   time.Time
}

type Limiter interface {
	Allow(
		ctx context.Context,
		key string,
		limit int,
		window time.Duration,
	) (*Result, error)
}