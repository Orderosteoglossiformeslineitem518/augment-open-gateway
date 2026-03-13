package service

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"augment-gateway/internal/logger"
)

// PoolTokenLimitService 号池TOKEN切换限制服务
type PoolTokenLimitService struct {
	cache *CacheService
}

// NewPoolTokenLimitService 创建号池TOKEN切换限制服务
func NewPoolTokenLimitService(cache *CacheService) *PoolTokenLimitService {
	return &PoolTokenLimitService{
		cache: cache,
	}
}

// PoolTokenSwitchResult 号池TOKEN切换结果
type PoolTokenSwitchResult struct {
	CanSwitch    bool   `json:"can_switch"`    // 是否可以切换
	CurrentCount int    `json:"current_count"` // 当前切换次数
	MaxPerDay    int    `json:"max_per_day"`   // 每日最大切换次数
	ExceedsLimit bool   `json:"exceeds_limit"` // 是否超过限制
	Message      string `json:"message"`       // 提示信息
}

const (
	// MaxPoolTokenSwitchPerDay 每日最大号池TOKEN切换次数
	MaxPoolTokenSwitchPerDay = 5
)

// CheckPoolTokenSwitchLimit 检查用户今天的号池TOKEN切换次数限制
func (s *PoolTokenLimitService) CheckPoolTokenSwitchLimit(ctx context.Context, userID int) (*PoolTokenSwitchResult, error) {
	// 生成缓存键
	cacheKey := s.getPoolTokenSwitchCacheKey(userID)

	// 获取当前切换次数
	currentCount, err := s.getCurrentSwitchCount(ctx, cacheKey)
	if err != nil {
		logger.Infof("[号池TOKEN限制] 获取用户%d切换次数失败: %v\n", userID, err)
		// 获取失败时假设为0，不阻止操作
		currentCount = 0
	}

	result := &PoolTokenSwitchResult{
		CurrentCount: currentCount,
		MaxPerDay:    MaxPoolTokenSwitchPerDay,
		ExceedsLimit: currentCount >= MaxPoolTokenSwitchPerDay,
	}

	if result.ExceedsLimit {
		result.CanSwitch = true // 仍然可以切换，但会返回模拟响应
		result.Message = fmt.Sprintf("您今天已经切换了%d次号池TOKEN（每天最多%d次），请等待24小时后再试",
			currentCount, MaxPoolTokenSwitchPerDay)
	} else {
		result.CanSwitch = true
		result.Message = fmt.Sprintf("今天还可以切换%d次号池TOKEN", MaxPoolTokenSwitchPerDay-currentCount)
	}

	return result, nil
}

// IncrementPoolTokenSwitchCount 增加用户的号池TOKEN切换次数
func (s *PoolTokenLimitService) IncrementPoolTokenSwitchCount(ctx context.Context, userID int) error {
	cacheKey := s.getPoolTokenSwitchCacheKey(userID)

	// 获取当前次数
	currentCount, err := s.getCurrentSwitchCount(ctx, cacheKey)
	if err != nil {
		logger.Infof("[号池TOKEN限制] 获取用户%d当前切换次数失败: %v\n", userID, err)
		currentCount = 0
	}

	// 增加次数
	newCount := currentCount + 1

	// 设置缓存过期时间到当天结束
	expiration := s.getTimeUntilEndOfDay()

	// 更新缓存
	if err := s.cache.SetString(ctx, cacheKey, strconv.Itoa(newCount), expiration); err != nil {
		logger.Infof("[号池TOKEN限制] 设置用户%d切换次数缓存失败: %v\n", userID, err)
		return err
	}

	logger.Infof("[号池TOKEN限制] 用户%d号池TOKEN切换次数已更新: %d/%d\n",
		userID, newCount, MaxPoolTokenSwitchPerDay)

	return nil
}

// getPoolTokenSwitchCacheKey 生成号池TOKEN切换次数的缓存键
func (s *PoolTokenLimitService) getPoolTokenSwitchCacheKey(userID int) string {
	today := time.Now().Format("2006-01-02")
	return fmt.Sprintf("AUGMENT-GATEWAY:pool_token_switch_count:%d:%s", userID, today)
}

// getCurrentSwitchCount 获取当前切换次数
func (s *PoolTokenLimitService) getCurrentSwitchCount(ctx context.Context, cacheKey string) (int, error) {
	countStr, err := s.cache.GetString(ctx, cacheKey)
	if err != nil {
		// 缓存未命中或错误，返回0
		return 0, nil
	}

	if countStr == "" {
		return 0, nil
	}

	count, err := strconv.Atoi(countStr)
	if err != nil {
		logger.Infof("[号池TOKEN限制] 解析切换次数失败: %v\n", err)
		return 0, nil
	}

	return count, nil
}

// getTimeUntilEndOfDay 获取到当天结束的时间间隔
func (s *PoolTokenLimitService) getTimeUntilEndOfDay() time.Duration {
	now := time.Now()
	tomorrow := now.AddDate(0, 0, 1)
	startOfTomorrow := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 0, 0, 0, 0, tomorrow.Location())
	return time.Until(startOfTomorrow)
}
