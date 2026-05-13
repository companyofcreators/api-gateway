package ratelimit

import (
	"context"
	_ "embed"
	"time"

	domain "github.com/companyofcreators/api-gateway/internal/domain/ratelimit"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

//go:embed scripts/sliding_window.lua
var slidingWindowScript string

type RedisSlidingWindowLimiter struct {
	client *redis.Client
	script *redis.Script
}

func NewRedisSlidingWindowLimiter(
	client *redis.Client,
) *RedisSlidingWindowLimiter {
	return &RedisSlidingWindowLimiter{
		client: client,
		script: redis.NewScript(slidingWindowScript),
	}
}

func (r *RedisSlidingWindowLimiter) Allow(
	ctx context.Context,
	key string,
	limit int,
	window time.Duration,
) (*domain.Result, error) {

	now := time.Now().UnixMilli()

	requestID := uuid.NewString()

	result, err := r.script.Run(
		ctx,
		r.client,
		[]string{key},
		now,
		window.Milliseconds(),
		limit,
		requestID,
	).Result()

	if err != nil {
		return nil, err
	}

	values := result.([]interface{})

	allowed := values[0].(int64) == 1

	remaining := int(values[1].(int64))

	resetUnixMs := values[2].(int64)

	return &domain.Result{
		Allowed:   allowed,
		Remaining: remaining,
		ResetAt:   time.UnixMilli(resetUnixMs),
	}, nil
}
