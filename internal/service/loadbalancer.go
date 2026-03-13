package service

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"augment-gateway/internal/database"

	"gorm.io/gorm"
)

// LoadBalancerService 负载均衡服务
type LoadBalancerService struct {
	db              *gorm.DB
	cache           *CacheService
	tokenService    *TokenService
	mu              sync.RWMutex
	roundRobinIndex map[string]int // 轮询索引
}

// NewLoadBalancerService 创建负载均衡服务
func NewLoadBalancerService(db *gorm.DB, cache *CacheService, tokenService *TokenService) *LoadBalancerService {
	return &LoadBalancerService{
		db:              db,
		cache:           cache,
		tokenService:    tokenService,
		roundRobinIndex: make(map[string]int),
	}
}

// LoadBalancerStrategy 负载均衡策略
type LoadBalancerStrategy string

const (
	StrategyRoundRobin LoadBalancerStrategy = "round_robin"
	StrategyRandom     LoadBalancerStrategy = "random"
	StrategyWeighted   LoadBalancerStrategy = "weighted"
	StrategyLeastConn  LoadBalancerStrategy = "least_conn"
)

// TokenGroup 代表一组可用的token
type TokenGroup struct {
	Name     string                 `json:"name"`
	Strategy LoadBalancerStrategy   `json:"strategy"`
	Tokens   []*database.Token      `json:"tokens"`
	Weights  map[string]int         `json:"weights"` // token_id -> weight
	Config   map[string]interface{} `json:"config"`
}

// SelectToken 根据负载均衡策略选择token
func (s *LoadBalancerService) SelectToken(ctx context.Context, groupName string) (*database.Token, error) {
	// 获取token组
	group, err := s.getTokenGroup(ctx, groupName)
	if err != nil {
		return nil, fmt.Errorf("failed to get token group: %w", err)
	}

	if len(group.Tokens) == 0 {
		return nil, fmt.Errorf("no available tokens in group %s", groupName)
	}

	// 过滤健康的token
	healthyTokens := s.filterHealthyTokens(ctx, group.Tokens)
	if len(healthyTokens) == 0 {
		return nil, fmt.Errorf("no healthy tokens available in group %s", groupName)
	}

	// 根据策略选择token
	switch group.Strategy {
	case StrategyRoundRobin:
		return s.selectRoundRobin(ctx, groupName, healthyTokens)
	case StrategyRandom:
		return s.selectRandom(healthyTokens)
	case StrategyWeighted:
		return s.selectWeighted(healthyTokens, group.Weights)
	case StrategyLeastConn:
		return s.selectLeastConnections(ctx, healthyTokens)
	default:
		return s.selectRoundRobin(ctx, groupName, healthyTokens)
	}
}

// selectRoundRobin 轮询选择
func (s *LoadBalancerService) selectRoundRobin(_ context.Context, groupName string, tokens []*database.Token) (*database.Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	index := s.roundRobinIndex[groupName]
	token := tokens[index%len(tokens)]
	s.roundRobinIndex[groupName] = (index + 1) % len(tokens)

	return token, nil
}

// selectRandom 随机选择
func (s *LoadBalancerService) selectRandom(tokens []*database.Token) (*database.Token, error) {
	if len(tokens) == 0 {
		return nil, fmt.Errorf("no tokens available")
	}

	rand.Seed(time.Now().UnixNano())
	index := rand.Intn(len(tokens))
	return tokens[index], nil
}

// selectWeighted 加权选择
func (s *LoadBalancerService) selectWeighted(tokens []*database.Token, weights map[string]int) (*database.Token, error) {
	if len(tokens) == 0 {
		return nil, fmt.Errorf("no tokens available")
	}

	// 计算总权重
	totalWeight := 0
	for _, token := range tokens {
		weight := weights[token.ID]
		if weight <= 0 {
			weight = 1 // 默认权重
		}
		totalWeight += weight
	}

	if totalWeight == 0 {
		return s.selectRandom(tokens)
	}

	// 随机选择
	rand.Seed(time.Now().UnixNano())
	randomWeight := rand.Intn(totalWeight)

	currentWeight := 0
	for _, token := range tokens {
		weight := weights[token.ID]
		if weight <= 0 {
			weight = 1
		}
		currentWeight += weight
		if randomWeight < currentWeight {
			return token, nil
		}
	}

	// 备选方案
	return tokens[0], nil
}

// selectLeastConnections 最少连接选择
func (s *LoadBalancerService) selectLeastConnections(ctx context.Context, tokens []*database.Token) (*database.Token, error) {
	if len(tokens) == 0 {
		return nil, fmt.Errorf("no tokens available")
	}

	var selectedToken *database.Token
	minConnections := int64(-1)

	for _, token := range tokens {
		// 从缓存获取当前连接数
		connections, err := s.getTokenConnections(ctx, token.Token)
		if err != nil {
			connections = 0 // 默认为0
		}

		if minConnections == -1 || connections < minConnections {
			minConnections = connections
			selectedToken = token
		}
	}

	return selectedToken, nil
}

// getTokenGroup 获取token组
func (s *LoadBalancerService) getTokenGroup(ctx context.Context, groupName string) (*TokenGroup, error) {
	// 从缓存获取配置
	var config database.LoadBalancerConfig
	if err := s.db.WithContext(ctx).First(&config).Error; err != nil {
		return nil, fmt.Errorf("failed to get load balancer config: %w", err)
	}

	// 获取所有活跃的token
	tokens, err := s.tokenService.GetActiveTokens(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get active tokens: %w", err)
	}

	group := &TokenGroup{
		Name:     groupName,
		Strategy: LoadBalancerStrategy(config.Strategy),
		Tokens:   tokens,
		Weights:  make(map[string]int),
		Config:   make(map[string]interface{}),
	}

	return group, nil
}

// filterHealthyTokens 过滤活跃的token
func (s *LoadBalancerService) filterHealthyTokens(_ context.Context, tokens []*database.Token) []*database.Token {
	var healthyTokens []*database.Token

	for _, token := range tokens {
		if token.IsActive() {
			healthyTokens = append(healthyTokens, token)
		}
	}

	return healthyTokens
}

// getTokenConnections 获取token当前连接数
func (s *LoadBalancerService) getTokenConnections(ctx context.Context, tokenStr string) (int64, error) {
	key := fmt.Sprintf("connections:%s", tokenStr)
	return s.cache.GetCounter(ctx, key)
}

// IncrementConnections 增加连接数
func (s *LoadBalancerService) IncrementConnections(ctx context.Context, tokenStr string) error {
	key := fmt.Sprintf("connections:%s", tokenStr)
	return s.cache.IncrementCounter(ctx, key, time.Hour)
}

// DecrementConnections 减少连接数
func (s *LoadBalancerService) DecrementConnections(ctx context.Context, tokenStr string) error {
	key := fmt.Sprintf("connections:%s", tokenStr)
	client := s.cache.GetClient()
	return client.Decr(ctx, key).Err()
}

// UpdateStrategy 更新负载均衡策略
func (s *LoadBalancerService) UpdateStrategy(ctx context.Context, strategy LoadBalancerStrategy) error {
	return s.db.WithContext(ctx).Model(&database.LoadBalancerConfig{}).
		Where("id = ?", 1).
		Update("strategy", string(strategy)).Error
}

// GetStrategy 获取当前负载均衡策略
func (s *LoadBalancerService) GetStrategy(ctx context.Context) (LoadBalancerStrategy, error) {
	var config database.LoadBalancerConfig
	if err := s.db.WithContext(ctx).First(&config).Error; err != nil {
		return StrategyRoundRobin, fmt.Errorf("failed to get strategy: %w", err)
	}
	return LoadBalancerStrategy(config.Strategy), nil
}

// SetTokenWeight 设置token权重
func (s *LoadBalancerService) SetTokenWeight(ctx context.Context, tokenID string, weight int) error {
	// 这里可以实现权重存储逻辑
	// 可以存储在数据库或缓存中
	key := fmt.Sprintf("weight:token:%s", tokenID)
	return s.cache.SetSession(ctx, key, weight, 24*time.Hour)
}

// GetTokenWeight 获取token权重
func (s *LoadBalancerService) GetTokenWeight(ctx context.Context, tokenID string) (int, error) {
	key := fmt.Sprintf("weight:token:%s", tokenID)
	var weight int
	if err := s.cache.GetSession(ctx, key, &weight); err != nil {
		return 1, nil // 默认权重为1
	}
	return weight, nil
}

// GetLoadBalancerStats 获取负载均衡统计
func (s *LoadBalancerService) GetLoadBalancerStats(ctx context.Context) (map[string]interface{}, error) {
	strategy, err := s.GetStrategy(ctx)
	if err != nil {
		return nil, err
	}

	tokens, err := s.tokenService.GetActiveTokens(ctx)
	if err != nil {
		return nil, err
	}

	healthyCount := 0
	totalConnections := int64(0)

	for _, token := range tokens {
		if token.IsActive() {
			healthyCount++
		}
		connections, _ := s.getTokenConnections(ctx, token.Token)
		totalConnections += connections
	}

	stats := map[string]interface{}{
		"strategy":          string(strategy),
		"total_tokens":      len(tokens),
		"healthy_tokens":    healthyCount,
		"total_connections": totalConnections,
		"round_robin_state": s.roundRobinIndex,
	}

	return stats, nil
}

// ResetRoundRobinIndex 重置轮询索引
func (s *LoadBalancerService) ResetRoundRobinIndex(groupName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.roundRobinIndex[groupName] = 0
}
