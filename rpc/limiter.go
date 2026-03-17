package rpc

import (
	"context"

	"golang.org/x/time/rate"
)

// RateLimiter 封装了 RPS 限速逻辑
type RateLimiter struct {
	limiter *rate.Limiter
}

// NewRateLimiter 创建一个新的限速器
func NewRateLimiter(rps int) *RateLimiter {
	return &RateLimiter{
		limiter: rate.NewLimiter(rate.Limit(rps), rps),
	}
}

// Wait 等待直到允许下一个请求
func (rl *RateLimiter) Wait(ctx context.Context) error {
	return rl.limiter.Wait(ctx)
}
