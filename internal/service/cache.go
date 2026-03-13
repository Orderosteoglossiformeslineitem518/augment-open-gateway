package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"augment-gateway/internal/database"
	"augment-gateway/internal/logger"

	"github.com/go-redis/redis/v8"
)

// CacheService 缓存服务
type CacheService struct {
	redis *database.RedisClient
}

// NewCacheService 创建缓存服务
func NewCacheService(redis *database.RedisClient) *CacheService {
	return &CacheService{
		redis: redis,
	}
}

// GetRedisClient 获取Redis客户端
func (s *CacheService) GetRedisClient() *database.RedisClient {
	return s.redis
}

// Token缓存管理

// CacheToken 缓存token信息
func (s *CacheService) CacheToken(ctx context.Context, token *database.Token) error {
	// 缓存1小时
	return s.redis.CacheToken(ctx, token, time.Hour)
}

// GetToken 获取token信息（优先从缓存）
func (s *CacheService) GetToken(ctx context.Context, tokenStr string) (*database.Token, bool, error) {
	token, err := s.redis.GetCachedToken(ctx, tokenStr)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get cached token: %w", err)
	}

	if token != nil {
		return token, true, nil // 缓存命中
	}

	return nil, false, nil // 缓存未命中
}

// InvalidateToken 使token缓存失效
func (s *CacheService) InvalidateToken(ctx context.Context, tokenStr string) error {
	return s.redis.DeleteCachedToken(ctx, tokenStr)
}

// 限流管理

// CheckRateLimit 检查限流
func (s *CacheService) CheckRateLimit(ctx context.Context, tokenStr string, limit int) (bool, error) {
	key := fmt.Sprintf("AUGMENT-GATEWAY:rate_limit:token:%s", tokenStr)
	window := time.Minute

	return s.redis.CheckRateLimit(ctx, key, limit, window)
}

// CheckGlobalRateLimit 检查全局限流
func (s *CacheService) CheckGlobalRateLimit(ctx context.Context, clientIP string, limit int) (bool, error) {
	key := fmt.Sprintf("AUGMENT-GATEWAY:rate_limit:global:%s", clientIP)
	window := time.Minute

	return s.redis.CheckRateLimit(ctx, key, limit, window)
}

// 统计计数器

// IncrementRequestCount 增加请求计数
func (s *CacheService) IncrementRequestCount(ctx context.Context, tokenStr string) error {
	key := fmt.Sprintf("AUGMENT-GATEWAY:stats:requests:%s:%s", tokenStr, time.Now().Format("2006-01-02"))
	return s.redis.IncrementCounter(ctx, key, 24*time.Hour)
}

// IncrementSuccessCount 增加成功计数
func (s *CacheService) IncrementSuccessCount(ctx context.Context, tokenStr string) error {
	key := fmt.Sprintf("AUGMENT-GATEWAY:stats:success:%s:%s", tokenStr, time.Now().Format("2006-01-02"))
	return s.redis.IncrementCounter(ctx, key, 24*time.Hour)
}

// IncrementErrorCount 增加错误计数
func (s *CacheService) IncrementErrorCount(ctx context.Context, tokenStr string) error {
	key := fmt.Sprintf("AUGMENT-GATEWAY:stats:errors:%s:%s", tokenStr, time.Now().Format("2006-01-02"))
	return s.redis.IncrementCounter(ctx, key, 24*time.Hour)
}

// GetDailyStats 获取每日统计
func (s *CacheService) GetDailyStats(ctx context.Context, tokenStr string, date time.Time) (map[string]int64, error) {
	dateStr := date.Format("2006-01-02")

	requestKey := fmt.Sprintf("AUGMENT-GATEWAY:stats:requests:%s:%s", tokenStr, dateStr)
	successKey := fmt.Sprintf("AUGMENT-GATEWAY:stats:success:%s:%s", tokenStr, dateStr)
	errorKey := fmt.Sprintf("AUGMENT-GATEWAY:stats:errors:%s:%s", tokenStr, dateStr)

	requests, _ := s.redis.GetCounter(ctx, requestKey)
	success, _ := s.redis.GetCounter(ctx, successKey)
	errors, _ := s.redis.GetCounter(ctx, errorKey)

	return map[string]int64{
		"requests": requests,
		"success":  success,
		"errors":   errors,
	}, nil
}

// 会话管理

// CreateSession 创建会话
func (s *CacheService) CreateSession(ctx context.Context, sessionID string, data interface{}) error {
	return s.redis.SetSession(ctx, sessionID, data, 24*time.Hour)
}

// GetSession 获取会话
func (s *CacheService) GetSession(ctx context.Context, sessionID string, dest interface{}) error {
	return s.redis.GetSession(ctx, sessionID, dest)
}

// DeleteSession 删除会话
func (s *CacheService) DeleteSession(ctx context.Context, sessionID string) error {
	return s.redis.DeleteSession(ctx, sessionID)
}

// 负载均衡

// GetNextTokenRoundRobin 轮询获取下一个token
func (s *CacheService) GetNextTokenRoundRobin(ctx context.Context, tokens []string) (string, error) {
	return s.redis.GetNextToken(ctx, tokens)
}

// 缓存预热

// WarmupTokenCache 预热token缓存
func (s *CacheService) WarmupTokenCache(ctx context.Context, tokens []*database.Token) error {
	for _, token := range tokens {
		if err := s.CacheToken(ctx, token); err != nil {
			return fmt.Errorf("failed to cache token %s: %w", token.Token, err)
		}
	}
	return nil
}

// 缓存清理

// ClearExpiredCache 清理过期缓存
func (s *CacheService) ClearExpiredCache(ctx context.Context) error {
	// Redis会自动清理过期键，这里可以添加自定义清理逻辑
	return nil
}

// 缓存统计

// GetCacheStats 获取缓存统计信息
func (s *CacheService) GetCacheStats(ctx context.Context) (map[string]interface{}, error) {
	return s.redis.GetRedisStats(ctx)
}

// 计数器方法

// GetCounter 获取计数器值
func (s *CacheService) GetCounter(ctx context.Context, key string) (int64, error) {
	return s.redis.GetCounter(ctx, key)
}

// IncrementCounter 增加计数器
func (s *CacheService) IncrementCounter(ctx context.Context, key string, expiry time.Duration) error {
	return s.redis.IncrementCounter(ctx, key, expiry)
}

// GetClient 获取Redis客户端
func (s *CacheService) GetClient() *redis.Client {
	return s.redis.GetClient()
}

// 积分同步计数器管理

// IncrementCreditSyncCounter 增加积分同步计数器
// 返回递增后的计数值
func (s *CacheService) IncrementCreditSyncCounter(ctx context.Context, tokenID string) (int64, error) {
	key := fmt.Sprintf("AUGMENT-GATEWAY:credit_sync_counter:%s", tokenID)

	// 递增计数器
	result := s.redis.GetClient().Incr(ctx, key)
	if result.Err() != nil {
		return 0, fmt.Errorf("递增积分同步计数器失败: %w", result.Err())
	}

	// 设置过期时间为24小时（仅在首次创建时设置）
	// 使用EXPIRE命令，如果键已存在且有TTL则不会重置
	s.redis.GetClient().Expire(ctx, key, 24*time.Hour)

	return result.Val(), nil
}

// GetCreditSyncCounter 获取积分同步计数器值
func (s *CacheService) GetCreditSyncCounter(ctx context.Context, tokenID string) (int64, error) {
	key := fmt.Sprintf("AUGMENT-GATEWAY:credit_sync_counter:%s", tokenID)

	result := s.redis.GetClient().Get(ctx, key)
	if result.Err() != nil {
		if errors.Is(result.Err(), redis.Nil) {
			// 键不存在，返回0
			return 0, nil
		}
		return 0, fmt.Errorf("获取积分同步计数器失败: %w", result.Err())
	}

	return result.Int64()
}

// ResetCreditSyncCounter 重置积分同步计数器
func (s *CacheService) ResetCreditSyncCounter(ctx context.Context, tokenID string) error {
	key := fmt.Sprintf("AUGMENT-GATEWAY:credit_sync_counter:%s", tokenID)

	// 删除计数器键，下次访问时会重新从0开始
	result := s.redis.GetClient().Del(ctx, key)
	if result.Err() != nil {
		return fmt.Errorf("重置积分同步计数器失败: %w", result.Err())
	}

	return nil
}

// SessionEvents缓存管理

// CacheSessionEvents 缓存TOKEN的模拟会话事件数据
func (s *CacheService) CacheSessionEvents(ctx context.Context, tokenStr string, sessionEvents interface{}) error {
	key := fmt.Sprintf("AUGMENT-GATEWAY:session_events:%s", tokenStr)
	// 缓存7天，确保数据持久性
	return s.redis.SetSession(ctx, key, sessionEvents, 7*24*time.Hour)
}

// GetSessionEvents 获取TOKEN的模拟会话事件数据
func (s *CacheService) GetSessionEvents(ctx context.Context, tokenStr string, dest interface{}) error {
	key := fmt.Sprintf("AUGMENT-GATEWAY:session_events:%s", tokenStr)
	return s.redis.GetSession(ctx, key, dest)
}

// DeleteSessionEvents 删除TOKEN的模拟会话事件数据
func (s *CacheService) DeleteSessionEvents(ctx context.Context, tokenStr string) error {
	key := fmt.Sprintf("AUGMENT-GATEWAY:session_events:%s", tokenStr)
	return s.redis.DeleteSession(ctx, key)
}

// FeatureVector缓存管理
// 注意：SetSession 会自动添加 AUGMENT-GATEWAY:session: 前缀，所以这里只需要使用简单的 key

// CacheFeatureVector 缓存TOKEN的模拟特征向量数据
func (s *CacheService) CacheFeatureVector(ctx context.Context, tokenStr string, featureVector interface{}) error {
	key := fmt.Sprintf("feature_vector:%s", tokenStr)
	// 缓存7天，确保数据持久性
	return s.redis.SetSession(ctx, key, featureVector, 7*24*time.Hour)
}

// CacheFeatureVectorPermanent 永久缓存TOKEN的模拟特征向量数据
func (s *CacheService) CacheFeatureVectorPermanent(ctx context.Context, tokenStr string, featureVector interface{}) error {
	key := fmt.Sprintf("feature_vector:%s", tokenStr)
	// 永久缓存，TOKEN失效时会移除
	return s.redis.SetSession(ctx, key, featureVector, 0) // 0表示永不过期
}

// GetFeatureVector 获取TOKEN的模拟特征向量数据
func (s *CacheService) GetFeatureVector(ctx context.Context, tokenStr string, dest interface{}) error {
	key := fmt.Sprintf("feature_vector:%s", tokenStr)
	return s.redis.GetSession(ctx, key, dest)
}

// DeleteFeatureVector 删除TOKEN的模拟特征向量数据
func (s *CacheService) DeleteFeatureVector(ctx context.Context, tokenStr string) error {
	key := fmt.Sprintf("feature_vector:%s", tokenStr)
	return s.redis.DeleteSession(ctx, key)
}

// SetSession 设置会话信息
func (s *CacheService) SetSession(ctx context.Context, sessionID string, data interface{}, expiry time.Duration) error {
	return s.redis.SetSession(ctx, sessionID, data, expiry)
}

// 批量操作

// BatchCacheTokens 批量缓存tokens
func (s *CacheService) BatchCacheTokens(ctx context.Context, tokens []*database.Token) error {
	for _, token := range tokens {
		if err := s.CacheToken(ctx, token); err != nil {
			// 记录错误但继续处理其他token
			logger.Warnf("Warning: failed to cache token %s: %v\n", token.Token, err)
		}
	}
	return nil
}

// Exists 检查键是否存在
func (s *CacheService) Exists(ctx context.Context, key string) (bool, error) {
	result := s.redis.GetClient().Exists(ctx, key)
	if result.Err() != nil {
		return false, result.Err()
	}
	return result.Val() > 0, nil
}

// BatchInvalidateTokens 批量使token缓存失效
func (s *CacheService) BatchInvalidateTokens(ctx context.Context, tokenStrs []string) error {
	for _, tokenStr := range tokenStrs {
		if err := s.InvalidateToken(ctx, tokenStr); err != nil {
			// 记录错误但继续处理其他token
			logger.Warnf("Warning: failed to invalidate token %s: %v\n", tokenStr, err)
		}
	}
	return nil
}

// 分布式锁

// AcquireLock 获取分布式锁
func (s *CacheService) AcquireLock(ctx context.Context, key string, expiry time.Duration) (bool, error) {
	lockKey := fmt.Sprintf("AUGMENT-GATEWAY:lock:%s", key)
	result := s.redis.GetClient().SetNX(ctx, lockKey, "locked", expiry)
	return result.Val(), result.Err()
}

// ReleaseLock 释放分布式锁
func (s *CacheService) ReleaseLock(ctx context.Context, key string) error {
	lockKey := fmt.Sprintf("AUGMENT-GATEWAY:lock:%s", key)
	return s.redis.GetClient().Del(ctx, lockKey).Err()
}

// SetNX 原子性设置键值，仅当键不存在时设置
func (s *CacheService) SetNX(ctx context.Context, key string, value interface{}, expiry time.Duration) (bool, error) {
	result := s.redis.GetClient().SetNX(ctx, key, value, expiry)
	return result.Val(), result.Err()
}

// 配置缓存（直接操作 Redis，不经过 SetSession/GetSession 避免多层前缀嵌套）

// CacheConfig 缓存配置
func (s *CacheService) CacheConfig(ctx context.Context, key string, value interface{}, expiry time.Duration) error {
	configKey := fmt.Sprintf("AUGMENT-GATEWAY:config:%s", key)
	jsonData, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal config data: %w", err)
	}
	return s.redis.GetClient().Set(ctx, configKey, jsonData, expiry).Err()
}

// GetCachedConfig 获取缓存的配置
func (s *CacheService) GetCachedConfig(ctx context.Context, key string, dest interface{}) error {
	configKey := fmt.Sprintf("AUGMENT-GATEWAY:config:%s", key)
	data, err := s.redis.GetClient().Get(ctx, configKey).Result()
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(data), dest)
}

// DeleteCachedConfig 删除缓存的配置
func (s *CacheService) DeleteCachedConfig(ctx context.Context, key string) error {
	configKey := fmt.Sprintf("AUGMENT-GATEWAY:config:%s", key)
	return s.redis.GetClient().Del(ctx, configKey).Err()
}

// TOKEN渠道绑定缓存管理

// tokenChannelBindingCacheKeyPrefix TOKEN渠道绑定缓存键前缀
const tokenChannelBindingCacheKeyPrefix = "token_channel_binding:"

// tokenChannelBindingCacheTTL TOKEN渠道绑定缓存有效期（30分钟）
const tokenChannelBindingCacheTTL = 30 * time.Minute

// CachedExternalChannel 用于缓存的外部渠道结构体
// 注意：database.ExternalChannel 的 APIKeyEncrypted 字段有 json:"-" 标签，会在JSON序列化时被忽略
// 因此需要使用这个专门的缓存结构体来保存所有必要字段
type CachedExternalChannel struct {
	ID                          uint                            `json:"id"`
	UserID                      uint                            `json:"user_id"`
	ProviderName                string                          `json:"provider_name"`
	Remark                      string                          `json:"remark"`
	WebsiteURL                  string                          `json:"website_url"`
	APIEndpoint                 string                          `json:"api_endpoint"`
	APIKeyEncrypted             string                          `json:"api_key_encrypted"` // 显式包含，不使用 json:"-"
	CustomUserAgent             string                          `json:"custom_user_agent"`
	Icon                        string                          `json:"icon"`
	Status                      string                          `json:"status"`
	LastTestLatency             *int64                          `json:"last_test_latency"`
	ThinkingSignatureEnabled    string                          `json:"thinking_signature_enabled"`
	ClaudeCodeSimulationEnabled string                          `json:"claude_code_simulation_enabled"` // ClaudeCode客户端模拟开关
	TitleGenerationModelMapping string                          `json:"title_generation_model_mapping"` // 标题生成模型映射
	SummaryModelMapping         string                          `json:"summary_model_mapping"`          // 对话总结模型映射
	Models                      []database.ExternalChannelModel `json:"models,omitempty"`
}

// ToExternalChannel 转换为 database.ExternalChannel
func (c *CachedExternalChannel) ToExternalChannel() *database.ExternalChannel {
	return &database.ExternalChannel{
		ID:                          c.ID,
		UserID:                      c.UserID,
		ProviderName:                c.ProviderName,
		Remark:                      c.Remark,
		WebsiteURL:                  c.WebsiteURL,
		APIEndpoint:                 c.APIEndpoint,
		APIKeyEncrypted:             c.APIKeyEncrypted,
		CustomUserAgent:             c.CustomUserAgent,
		Icon:                        c.Icon,
		Status:                      c.Status,
		LastTestLatency:             c.LastTestLatency,
		ThinkingSignatureEnabled:    c.ThinkingSignatureEnabled,
		ClaudeCodeSimulationEnabled: c.ClaudeCodeSimulationEnabled,
		TitleGenerationModelMapping: c.TitleGenerationModelMapping,
		SummaryModelMapping:         c.SummaryModelMapping,
		Models:                      c.Models,
	}
}

// FromExternalChannel 从 database.ExternalChannel 创建缓存结构体
func FromExternalChannel(ch *database.ExternalChannel) *CachedExternalChannel {
	return &CachedExternalChannel{
		ID:                          ch.ID,
		UserID:                      ch.UserID,
		ProviderName:                ch.ProviderName,
		Remark:                      ch.Remark,
		WebsiteURL:                  ch.WebsiteURL,
		APIEndpoint:                 ch.APIEndpoint,
		APIKeyEncrypted:             ch.APIKeyEncrypted,
		CustomUserAgent:             ch.CustomUserAgent,
		Icon:                        ch.Icon,
		Status:                      ch.Status,
		LastTestLatency:             ch.LastTestLatency,
		ThinkingSignatureEnabled:    ch.ThinkingSignatureEnabled,
		ClaudeCodeSimulationEnabled: ch.ClaudeCodeSimulationEnabled,
		TitleGenerationModelMapping: ch.TitleGenerationModelMapping,
		SummaryModelMapping:         ch.SummaryModelMapping,
		Models:                      ch.Models,
	}
}

// getTokenChannelBindingCacheKey 获取TOKEN渠道绑定缓存键
// tokenID: Token的雪花ID（string类型）
// userID: 用户ID
func getTokenChannelBindingCacheKey(tokenID string, userID uint) string {
	return fmt.Sprintf("%s%s:%d", tokenChannelBindingCacheKeyPrefix, tokenID, userID)
}

// CacheTokenChannelBinding 缓存TOKEN绑定的外部渠道
func (s *CacheService) CacheTokenChannelBinding(ctx context.Context, tokenID string, userID uint, channel *database.ExternalChannel) error {
	key := getTokenChannelBindingCacheKey(tokenID, userID)
	// 使用专门的缓存结构体，确保 APIKeyEncrypted 字段被序列化
	cached := FromExternalChannel(channel)
	return s.CacheConfig(ctx, key, cached, tokenChannelBindingCacheTTL)
}

// GetCachedTokenChannelBinding 获取缓存的TOKEN绑定外部渠道
func (s *CacheService) GetCachedTokenChannelBinding(ctx context.Context, tokenID string, userID uint) (*database.ExternalChannel, error) {
	key := getTokenChannelBindingCacheKey(tokenID, userID)
	var cached CachedExternalChannel
	if err := s.GetCachedConfig(ctx, key, &cached); err != nil {
		return nil, err
	}
	return cached.ToExternalChannel(), nil
}

// InvalidateTokenChannelBinding 使TOKEN渠道绑定缓存失效
func (s *CacheService) InvalidateTokenChannelBinding(ctx context.Context, tokenID string, userID uint) error {
	key := getTokenChannelBindingCacheKey(tokenID, userID)
	return s.DeleteCachedConfig(ctx, key)
}

// InvalidateAllChannelBindingsByUserAndChannel 根据用户ID和TOKEN列表清除相关缓存
// 需要传入tokenIDs列表，因为缓存键是 tokenID:userID 格式
func (s *CacheService) InvalidateAllChannelBindingsByUserAndChannel(ctx context.Context, userID uint, tokenIDs []string) error {
	for _, tokenID := range tokenIDs {
		if err := s.InvalidateTokenChannelBinding(ctx, tokenID, userID); err != nil {
			logger.Warnf("[缓存] 清除TOKEN渠道绑定缓存失败: tokenID=%s, userID=%d, error=%v", tokenID, userID, err)
		}
	}
	return nil
}

// 用户API令牌限流管理

// CheckUserTokenRateLimit 检查用户令牌限流
func (s *CacheService) CheckUserTokenRateLimit(ctx context.Context, tokenStr string, rateLimit int) (bool, error) {
	key := fmt.Sprintf("AUGMENT-GATEWAY:user_token_rate_limit:%s", tokenStr)

	// 使用滑动窗口限流 - 1分钟窗口，与RateLimitPerMinute字段语义一致
	now := time.Now().Unix()
	windowStart := now - 60 // 1分钟窗口 (60秒)

	pipe := s.redis.GetClient().Pipeline()

	// 移除过期的记录
	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart))

	// 添加当前请求
	pipe.ZAdd(ctx, key, &redis.Z{Score: float64(now), Member: now})

	// 获取当前窗口内的请求数
	pipe.ZCard(ctx, key)

	// 设置过期时间
	pipe.Expire(ctx, key, time.Minute*2) // 2分钟过期时间，确保数据清理及时

	results, err := pipe.Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to check user token rate limit: %w", err)
	}

	// 获取当前请求数
	count := results[2].(*redis.IntCmd).Val()

	return count <= int64(rateLimit), nil
}

// GetNextTokenFromList 从令牌列表中获取下一个令牌（轮询）
func (s *CacheService) GetNextTokenFromList(ctx context.Context, tokens []string) (string, error) {
	return s.redis.GetNextToken(ctx, tokens)
}

// 概览统计缓存

// CacheOverviewStats 缓存概览统计数据
func (s *CacheService) CacheOverviewStats(ctx context.Context, stats interface{}) error {
	key := "AUGMENT-GATEWAY:stats:overview"
	return s.redis.SetSession(ctx, key, stats, 10*time.Minute) // 缓存10分钟
}

// GetCachedOverviewStats 获取缓存的概览统计数据
func (s *CacheService) GetCachedOverviewStats(ctx context.Context, dest interface{}) error {
	key := "AUGMENT-GATEWAY:stats:overview"
	return s.redis.GetSession(ctx, key, dest)
}

// SetString 设置字符串值
func (s *CacheService) SetString(ctx context.Context, key string, value string, ttl time.Duration) error {
	return s.redis.GetClient().Set(ctx, key, value, ttl).Err()
}

// GetString 获取字符串值
func (s *CacheService) GetString(ctx context.Context, key string) (string, error) {
	result, err := s.redis.GetClient().Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", nil // 缓存未命中，返回空字符串和nil错误
		}
		return "", err
	}
	return result, nil
}

// ExtendTTL 延长缓存键的TTL
func (s *CacheService) ExtendTTL(ctx context.Context, key string, ttl time.Duration) error {
	return s.redis.GetClient().Expire(ctx, key, ttl).Err()
}

// DeleteKey 删除键
func (s *CacheService) DeleteKey(ctx context.Context, key string) error {
	return s.redis.GetClient().Del(ctx, key).Err()
}

// InvalidateOverviewStats 使概览统计缓存失效
func (s *CacheService) InvalidateOverviewStats(ctx context.Context) error {
	key := "AUGMENT-GATEWAY:stats:overview"
	return s.redis.GetClient().Del(ctx, key).Err()
}

// CacheHourlyUsageStats 缓存小时使用统计数据
func (s *CacheService) CacheHourlyUsageStats(ctx context.Context, cacheKey string, stats interface{}) error {
	return s.redis.SetSession(ctx, cacheKey, stats, 5*time.Minute) // 缓存5分钟
}

// GetCachedHourlyUsageStats 获取缓存的小时使用统计数据
func (s *CacheService) GetCachedHourlyUsageStats(ctx context.Context, cacheKey string, dest interface{}) error {
	return s.redis.GetSession(ctx, cacheKey, dest)
}

// CacheDailyUsageStats 缓存每日使用统计数据
func (s *CacheService) CacheDailyUsageStats(ctx context.Context, cacheKey string, stats interface{}) error {
	return s.redis.SetSession(ctx, cacheKey, stats, 5*time.Minute) // 缓存5分钟
}

// GetCachedDailyUsageStats 获取缓存的每日使用统计数据
func (s *CacheService) GetCachedDailyUsageStats(ctx context.Context, cacheKey string, dest interface{}) error {
	return s.redis.GetSession(ctx, cacheKey, dest)
}

// ActiveTokens缓存管理

// CacheActiveTokens 缓存活跃Token列表
func (s *CacheService) CacheActiveTokens(ctx context.Context, tokens []*database.Token) error {
	key := "AUGMENT-GATEWAY:active_tokens"
	return s.redis.SetSession(ctx, key, tokens, 5*time.Minute) // 缓存5分钟
}

// TOKEN使用人数统计

// GetTokenCurrentUsersCount 批量获取TOKEN当前使用人数
func (s *CacheService) GetTokenCurrentUsersCount(ctx context.Context, tokenIDs []string) (map[string]int, error) {
	// 初始化结果映射，确保所有TOKEN都有计数（默认为0）
	result := make(map[string]int)
	for _, tokenID := range tokenIDs {
		result[tokenID] = 0
	}

	if len(tokenIDs) == 0 {
		return result, nil
	}

	// 扫描所有用户TOKEN分配缓存
	pattern := "AUGMENT-GATEWAY:user_token_assignment:*"
	keys, err := s.ScanKeys(ctx, pattern)
	if err != nil {
		// 扫描失败时返回初始化的结果（所有TOKEN计数为0），而不是返回错误
		logger.Warnf("警告: 扫描用户TOKEN分配缓存失败: %v，返回默认计数\n", err)
		return result, nil
	}

	// 如果没有任何分配记录，返回初始化的结果
	if len(keys) == 0 {
		return result, nil
	}

	// 使用Pipeline批量获取所有缓存值
	pipe := s.redis.GetClient().Pipeline()
	cmds := make([]*redis.StringCmd, len(keys))
	for i, key := range keys {
		cmds[i] = pipe.Get(ctx, key)
	}

	_, err = pipe.Exec(ctx)
	if err != nil && !errors.Is(err, redis.Nil) {
		// Pipeline执行失败时返回初始化的结果，而不是返回错误
		logger.Warnf("警告: 批量获取TOKEN分配失败: %v，返回默认计数\n", err)
		return result, nil
	}

	// 统计每个TOKEN的使用人数
	for _, cmd := range cmds {
		assignedTokenID, err := cmd.Result()
		if err == nil && assignedTokenID != "" {
			// 检查这个TOKEN是否在我们要统计的列表中
			if _, exists := result[assignedTokenID]; exists {
				result[assignedTokenID]++
			}
		}
	}

	return result, nil
}

// GetCachedActiveTokens 获取缓存的活跃Token列表
func (s *CacheService) GetCachedActiveTokens(ctx context.Context) ([]*database.Token, bool, error) {
	key := "AUGMENT-GATEWAY:active_tokens"
	var tokens []*database.Token
	err := s.redis.GetSession(ctx, key, &tokens)
	if err != nil {
		return nil, false, nil // 缓存未命中或错误
	}
	return tokens, true, nil
}

// DeleteTokenCache 删除Token缓存
func (s *CacheService) DeleteTokenCache(ctx context.Context, tokenID string) error {
	// 根据tokenID获取token字符串，然后删除缓存
	// 这里我们使用通用的键删除方法
	// 如果需要根据tokenID删除，可能需要先查询获取token字符串
	// 为了简化，我们直接删除可能的缓存键
	tokenKey := fmt.Sprintf("AUGMENT-GATEWAY:token:%s", tokenID)
	return s.redis.GetClient().Del(ctx, tokenKey).Err()
}

// InvalidateActiveTokens 使活跃Token缓存失效
func (s *CacheService) InvalidateActiveTokens(ctx context.Context) error {
	key := "AUGMENT-GATEWAY:active_tokens"
	return s.redis.GetClient().Del(ctx, key).Err()
}

// ClearUserTokenRateLimit 清空用户令牌的频率限制缓存
func (s *CacheService) ClearUserTokenRateLimit(ctx context.Context, tokenStr string) error {
	key := fmt.Sprintf("AUGMENT-GATEWAY:user_token_rate_limit:%s", tokenStr)
	return s.redis.GetClient().Del(ctx, key).Err()
}

// ScanKeys 扫描匹配模式的Redis键
func (s *CacheService) ScanKeys(ctx context.Context, pattern string) ([]string, error) {
	var keys []string
	var cursor uint64

	for {
		// 使用SCAN命令扫描键，每次扫描100个
		scanKeys, newCursor, err := s.redis.GetClient().Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, fmt.Errorf("Redis SCAN失败: %w", err)
		}

		keys = append(keys, scanKeys...)
		cursor = newCursor

		// 如果cursor为0，表示扫描完成
		if cursor == 0 {
			break
		}
	}

	return keys, nil
}

// 公告已读状态管理

// MarkNotificationAsRead 标记公告为已读（使用Redis Set实现去重）
func (s *CacheService) MarkNotificationAsRead(ctx context.Context, notificationID, userToken string) error {
	key := fmt.Sprintf("AUGMENT-GATEWAY:notification_read:%s", notificationID)
	// 使用SADD命令添加到Set中，自动去重
	err := s.redis.GetClient().SAdd(ctx, key, userToken).Err()
	if err != nil {
		return fmt.Errorf("标记公告已读失败: %w", err)
	}

	// 设置过期时间为30天
	err = s.redis.GetClient().Expire(ctx, key, 30*24*time.Hour).Err()
	if err != nil {
		return fmt.Errorf("设置公告已读记录过期时间失败: %w", err)
	}

	return nil
}

// GetNotificationReadCount 获取公告已读数量
func (s *CacheService) GetNotificationReadCount(ctx context.Context, notificationID string) (int64, error) {
	key := fmt.Sprintf("AUGMENT-GATEWAY:notification_read:%s", notificationID)
	count, err := s.redis.GetClient().SCard(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, nil // 没有已读记录时返回0
		}
		return 0, fmt.Errorf("获取公告已读数量失败: %w", err)
	}
	return count, nil
}

// IsNotificationReadByToken 检查指定TOKEN是否已读某个公告
func (s *CacheService) IsNotificationReadByToken(ctx context.Context, notificationID, userToken string) (bool, error) {
	key := fmt.Sprintf("AUGMENT-GATEWAY:notification_read:%s", notificationID)
	exists, err := s.redis.GetClient().SIsMember(ctx, key, userToken).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false, nil // 没有记录时返回false
		}
		return false, fmt.Errorf("检查公告已读状态失败: %w", err)
	}
	return exists, nil
}

// Silent跳过机制管理（前45次不跳过，第46次及以后永久跳过）

// GetSilentSkipCounter 获取TOKEN的计费请求计数器
func (s *CacheService) GetSilentSkipCounter(ctx context.Context, token string) (int64, error) {
	key := fmt.Sprintf("AUGMENT-GATEWAY:silent_skip_counter:%s", token)
	result, err := s.redis.GetClient().Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, nil // 计数器不存在时返回0
		}
		return 0, fmt.Errorf("获取silent跳过计数器失败: %w", err)
	}

	// 将字符串转换为int64
	var counter int64
	if _, err := fmt.Sscanf(result, "%d", &counter); err != nil {
		return 0, fmt.Errorf("解析计数器值失败: %w", err)
	}

	return counter, nil
}

// IncrementSilentSkipCounter 递增TOKEN的计费请求计数器
func (s *CacheService) IncrementSilentSkipCounter(ctx context.Context, token string) (int64, error) {
	key := fmt.Sprintf("AUGMENT-GATEWAY:silent_skip_counter:%s", token)

	// 使用INCR命令原子性递增，如果键不存在会自动创建并设为1
	result, err := s.redis.GetClient().Incr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("递增计费请求计数器失败: %w", err)
	}

	// 只有在计数器首次创建时（值为1）才设置过期时间（35天）
	if result == 1 {
		err = s.redis.GetClient().Expire(ctx, key, 35*24*time.Hour).Err()
		if err != nil {
			return result, fmt.Errorf("设置计数器过期时间失败: %w", err)
		}
	}

	return result, nil
}

// ResetSilentSkipCounter 重置TOKEN的计费请求计数器
func (s *CacheService) ResetSilentSkipCounter(ctx context.Context, token string) error {
	key := fmt.Sprintf("AUGMENT-GATEWAY:silent_skip_counter:%s", token)
	return s.redis.GetClient().Del(ctx, key).Err()
}

// =====================================================
// 增强对话缓存管理
// 用于标记已被转发到外部渠道的对话，以便拦截后续的事件记录请求
// =====================================================

// CacheEnhancedConversation 缓存增强对话的conversation_id
// 当chat-stream请求被转发到外部渠道时调用，用于标记该对话已在外部处理
// 缓存过期时间为24小时
func (s *CacheService) CacheEnhancedConversation(ctx context.Context, conversationID string) error {
	if conversationID == "" {
		return nil
	}
	key := fmt.Sprintf("AUGMENT-GATEWAY:enhanced_conversation:%s", conversationID)
	// 存储当前时间作为标记值，过期时间24小时
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	return s.redis.GetClient().Set(ctx, key, timestamp, 24*time.Hour).Err()
}

// IsEnhancedConversation 检查conversation_id是否为增强对话（已被转发到外部渠道）
// 返回 (是否为增强对话, 错误)
// 如果Redis连接失败，返回 (false, error)，调用方应降级到正常转发
func (s *CacheService) IsEnhancedConversation(ctx context.Context, conversationID string) (bool, error) {
	if conversationID == "" {
		return false, nil
	}
	key := fmt.Sprintf("AUGMENT-GATEWAY:enhanced_conversation:%s", conversationID)
	result, err := s.redis.GetClient().Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("检查增强对话缓存失败: %w", err)
	}
	return result > 0, nil
}

// 系统公告已读状态管理

// SetSystemAnnouncementLastRead 设置用户最后阅读系统公告的时间
func (s *CacheService) SetSystemAnnouncementLastRead(ctx context.Context, userID uint) error {
	key := fmt.Sprintf("AUGMENT-GATEWAY:system_announcement_last_read:%d", userID)
	now := time.Now().Unix()
	err := s.redis.GetClient().Set(ctx, key, now, 90*24*time.Hour).Err() // 90天有效期
	if err != nil {
		return fmt.Errorf("设置系统公告已读时间失败: %w", err)
	}
	return nil
}

// GetSystemAnnouncementLastRead 获取用户最后阅读系统公告的时间戳
func (s *CacheService) GetSystemAnnouncementLastRead(ctx context.Context, userID uint) (int64, error) {
	key := fmt.Sprintf("AUGMENT-GATEWAY:system_announcement_last_read:%d", userID)
	result, err := s.redis.GetClient().Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, nil // 没有记录时返回0，表示从未阅读
		}
		return 0, fmt.Errorf("获取系统公告已读时间失败: %w", err)
	}

	var timestamp int64
	if _, err := fmt.Sscanf(result, "%d", &timestamp); err != nil {
		return 0, fmt.Errorf("解析已读时间戳失败: %w", err)
	}
	return timestamp, nil
}
