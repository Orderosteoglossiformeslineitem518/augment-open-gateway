package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"augment-gateway/internal/config"

	"github.com/go-redis/redis/v8"
)

// RedisClient Redis客户端包装
type RedisClient struct {
	client *redis.Client
}

// InitRedis 初始化Redis连接
func InitRedis(cfg *config.RedisConfig) (*RedisClient, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.GetRedisAddr(),
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping Redis: %w", err)
	}

	return &RedisClient{client: rdb}, nil
}

// GetClient 获取原始Redis客户端
func (r *RedisClient) GetClient() *redis.Client {
	return r.client
}

// Close 关闭Redis连接
func (r *RedisClient) Close() error {
	return r.client.Close()
}

// Token缓存相关方法

// CacheToken 缓存token信息
func (r *RedisClient) CacheToken(ctx context.Context, token *Token, expiry time.Duration) error {
	key := fmt.Sprintf("AUGMENT-GATEWAY:token:%s", token.Token)
	data, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("failed to marshal token: %w", err)
	}
	return r.client.Set(ctx, key, data, expiry).Err()
}

// GetCachedToken 获取缓存的token信息
func (r *RedisClient) GetCachedToken(ctx context.Context, tokenStr string) (*Token, error) {
	key := fmt.Sprintf("AUGMENT-GATEWAY:token:%s", tokenStr)
	data, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // 缓存未命中
		}
		return nil, fmt.Errorf("failed to get cached token: %w", err)
	}

	var token Token
	if err := json.Unmarshal([]byte(data), &token); err != nil {
		return nil, fmt.Errorf("failed to unmarshal token: %w", err)
	}

	return &token, nil
}

// DeleteCachedToken 删除缓存的token
func (r *RedisClient) DeleteCachedToken(ctx context.Context, tokenStr string) error {
	key := fmt.Sprintf("AUGMENT-GATEWAY:token:%s", tokenStr)
	return r.client.Del(ctx, key).Err()
}

// 限流相关方法

// CheckRateLimit 检查限流
func (r *RedisClient) CheckRateLimit(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	now := time.Now().Unix()
	windowStart := now - int64(window.Seconds())

	pipe := r.client.Pipeline()

	// 删除窗口外的记录
	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart))

	// 添加当前请求
	pipe.ZAdd(ctx, key, &redis.Z{Score: float64(now), Member: now})

	// 获取当前窗口内的请求数
	pipe.ZCard(ctx, key)

	// 设置过期时间
	pipe.Expire(ctx, key, window)

	results, err := pipe.Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to execute rate limit pipeline: %w", err)
	}

	// 获取当前请求数
	count := results[2].(*redis.IntCmd).Val()

	return count <= int64(limit), nil
}

// 统计相关方法

// IncrementCounter 增加计数器
func (r *RedisClient) IncrementCounter(ctx context.Context, key string, expiry time.Duration) error {
	pipe := r.client.Pipeline()
	pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, expiry)
	_, err := pipe.Exec(ctx)
	return err
}

// GetCounter 获取计数器值
func (r *RedisClient) GetCounter(ctx context.Context, key string) (int64, error) {
	return r.client.Get(ctx, key).Int64()
}

// 会话相关方法

// SetSession 设置会话信息
func (r *RedisClient) SetSession(ctx context.Context, sessionID string, data interface{}, expiry time.Duration) error {
	key := fmt.Sprintf("AUGMENT-GATEWAY:session:%s", sessionID)
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal session data: %w", err)
	}
	return r.client.Set(ctx, key, jsonData, expiry).Err()
}

// GetSession 获取会话信息
func (r *RedisClient) GetSession(ctx context.Context, sessionID string, dest interface{}) error {
	key := fmt.Sprintf("AUGMENT-GATEWAY:session:%s", sessionID)
	data, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return fmt.Errorf("session not found")
		}
		return fmt.Errorf("failed to get session: %w", err)
	}

	return json.Unmarshal([]byte(data), dest)
}

// DeleteSession 删除会话
func (r *RedisClient) DeleteSession(ctx context.Context, sessionID string) error {
	key := fmt.Sprintf("AUGMENT-GATEWAY:session:%s", sessionID)
	return r.client.Del(ctx, key).Err()
}

// 负载均衡相关方法

// GetNextToken 获取下一个可用token（轮询）
func (r *RedisClient) GetNextToken(ctx context.Context, tokens []string) (string, error) {
	if len(tokens) == 0 {
		return "", fmt.Errorf("no tokens available")
	}

	key := "AUGMENT-GATEWAY:lb:round_robin:index"

	// 获取当前索引
	index, err := r.client.Get(ctx, key).Int64()
	if err != nil && err != redis.Nil {
		return "", fmt.Errorf("failed to get round robin index: %w", err)
	}

	// 计算下一个索引
	nextIndex := (index + 1) % int64(len(tokens))

	// 更新索引
	if err := r.client.Set(ctx, key, nextIndex, time.Hour).Err(); err != nil {
		return "", fmt.Errorf("failed to update round robin index: %w", err)
	}

	return tokens[index], nil
}

// GetRedisStats 获取Redis统计信息
func (r *RedisClient) GetRedisStats(ctx context.Context) (map[string]interface{}, error) {
	info, err := r.client.Info(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get Redis info: %w", err)
	}

	poolStats := r.client.PoolStats()

	stats := map[string]interface{}{
		"info": info,
		"pool_stats": map[string]interface{}{
			"hits":        poolStats.Hits,
			"misses":      poolStats.Misses,
			"timeouts":    poolStats.Timeouts,
			"total_conns": poolStats.TotalConns,
			"idle_conns":  poolStats.IdleConns,
			"stale_conns": poolStats.StaleConns,
		},
	}

	return stats, nil
}
