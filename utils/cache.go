package utils

import (
	"log"

	lru "github.com/hashicorp/golang-lru/v2"
)

// Cache 封装了 LRU 缓存，用于存储已见过的 Token 或池子，防止重复处理
type Cache struct {
	lru *lru.Cache[string, struct{}]
}

// NewCache 创建一个新的 LRU 缓存，size 决定了最大存储数量
func NewCache(size int) *Cache {
	c, err := lru.New[string, struct{}](size)
	if err != nil {
		log.Fatalf("failed to create cache: %v", err)
	}
	return &Cache{lru: c}
}

// Add 将一个键添加到缓存中
func (c *Cache) Add(key string) {
	c.lru.Add(key, struct{}{})
}

// Contains 检查键是否存在于缓存中
func (c *Cache) Contains(key string) bool {
	return c.lru.Contains(key)
}

// Clear 清空缓存
func (c *Cache) Purge() {
	c.lru.Purge()
}
