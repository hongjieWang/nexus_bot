package rpc

import (
	"context"
	"log"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
)

// Client 统一管理带限速和重试机制的 RPC 连接
type Client struct {
	*ethclient.Client
	url     string
	limiter *RateLimiter
}

// NewClient 初始化 RPC 客户端
func NewClient(url string, rps int) (*Client, error) {
	ec, err := ethclient.Dial(url)
	if err != nil {
		return nil, err
	}

	return &Client{
		Client:  ec,
		url:     url,
		limiter: NewRateLimiter(rps),
	}, nil
}

// CallWithRetry 执行带重试的调用（模板方法）
func (c *Client) CallWithRetry(ctx context.Context, fn func() error, maxRetries int) error {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		// 限速等待
		if err := c.limiter.Wait(ctx); err != nil {
			return err
		}

		if err := fn(); err != nil {
			lastErr = err
			log.Printf("RPC error (attempt %d/%d): %v", i+1, maxRetries, err)
			time.Sleep(time.Duration(i*500) * time.Millisecond) // 指数退避
			continue
		}
		return nil
	}
	return lastErr
}

// Call 执行带重试和泛型结果返回的调用
func Call[T any](c *Client, ctx context.Context, fn func(ctx context.Context) (T, error)) (T, error) {
	var zero T
	var lastErr error
	maxRetries := 5 // 默认重试次数

	for i := 0; i < maxRetries; i++ {
		if err := c.limiter.Wait(ctx); err != nil {
			return zero, err
		}

		result, err := fn(ctx)
		if err == nil {
			return result, nil
		}

		lastErr = err
		// 这里可以加入对 429 等错误的特殊判断
		time.Sleep(time.Duration(i*500) * time.Millisecond)
	}
	return zero, lastErr
}

// Close 关闭连接
func (c *Client) Close() {
	if c.Client != nil {
		c.Client.Close()
	}
}
