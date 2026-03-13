package service

import (
	"context"
	"fmt"
	"time"

	"augment-gateway/internal/database"
	"augment-gateway/internal/logger"
	"augment-gateway/internal/repository"

	"gorm.io/gorm"
)

// StatsService 统计服务
type StatsService struct {
	db             *gorm.DB
	cache          *CacheService
	repository     repository.StatsRepository
	requestLogRepo repository.RequestLogRepository
}

// NewStatsService 创建统计服务
func NewStatsService(db *gorm.DB, cache *CacheService) *StatsService {
	return &StatsService{
		db:             db,
		cache:          cache,
		repository:     repository.NewStatsRepository(db),
		requestLogRepo: repository.NewRequestLogRepository(db),
	}
}

// TokenStats 获取指定token的统计
type TokenStats struct {
	TokenID         string     `json:"token_id"`
	Token           string     `json:"token"` // 实际的token值
	TokenName       string     `json:"token_name"`
	TenantURL       string     `json:"tenant_url"`
	TotalRequests   int64      `json:"total_requests"`
	SuccessRequests int64      `json:"success_requests"`
	ErrorRequests   int64      `json:"error_requests"`
	SuccessRate     float64    `json:"success_rate"`
	TotalBytes      int64      `json:"total_bytes"`
	LastRequest     *time.Time `json:"last_request"`
}

// UsageStats 使用统计
type UsageStats struct {
	Date     string `json:"date"`
	Requests int64  `json:"requests"`
	Success  int64  `json:"success"`
	Errors   int64  `json:"errors"`
	Bytes    int64  `json:"bytes"`
}

// OverviewStats 概览统计
type OverviewStats struct {
	TotalTokens     int64   `json:"total_tokens"`
	ActiveTokens    int64   `json:"active_tokens"`
	ExpiredTokens   int64   `json:"expired_tokens"`
	DisabledTokens  int64   `json:"disabled_tokens"`
	TotalRequests   int64   `json:"total_requests"`
	SuccessRequests int64   `json:"success_requests"`
	ErrorRequests   int64   `json:"error_requests"`
	SuccessRate     float64 `json:"success_rate"`
	TotalBytes      int64   `json:"total_bytes"`
}

// GetOverview 获取统计概览
func (s *StatsService) GetOverview(ctx context.Context) (*OverviewStats, error) {
	// 先尝试从缓存获取
	var cachedStats OverviewStats
	err := s.cache.GetCachedOverviewStats(ctx, &cachedStats)
	if err == nil {
		logger.Infof("[统计] 从缓存加载概览统计\n")
		return &cachedStats, nil
	}

	logger.Infof("[统计] 缓存未命中，从数据库加载概览统计\n")

	// 缓存未命中，从数据库获取
	overviewStats, err := s.repository.GetOverviewStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取概览统计失败: %w", err)
	}

	// 转换为服务层的OverviewStats结构
	result := &OverviewStats{
		TotalTokens:     overviewStats.TotalTokens,
		ActiveTokens:    overviewStats.ActiveTokens,
		ExpiredTokens:   overviewStats.ExpiredTokens,
		DisabledTokens:  overviewStats.DisabledTokens,
		TotalRequests:   overviewStats.TotalRequests,
		SuccessRequests: overviewStats.SuccessRequests,
		ErrorRequests:   overviewStats.ErrorRequests,
		SuccessRate:     overviewStats.SuccessRate,
	}

	// 异步缓存结果
	go func() {
		cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.cache.CacheOverviewStats(cacheCtx, result); err != nil {
			logger.Warnf("警告: 缓存概览统计失败: %v\n", err)
		}
	}()

	return result, nil
}

// LogRequest 记录请求日志
func (s *StatsService) LogRequest(ctx context.Context, log *database.RequestLog) error {
	// 保存请求日志
	if err := s.requestLogRepo.LogRequest(ctx, log); err != nil {
		return fmt.Errorf("保存请求日志失败: %w", err)
	}

	// 异步更新统计和缓存
	go func() {
		// 更新使用统计
		s.updateUsageStats(context.Background(), log)

		// 使概览统计缓存失效，确保下次获取时重新计算
		cacheCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := s.cache.InvalidateOverviewStats(cacheCtx); err != nil {
			logger.Warnf("警告: 使概览统计缓存失效失败: %v\n", err)
		}
	}()

	return nil
}

// updateUsageStats 更新使用统计
func (s *StatsService) updateUsageStats(ctx context.Context, log *database.RequestLog) {
	// 根据日志类型更新统计：优先使用系统TOKEN ID，如果没有则使用用户令牌ID
	var tokenID string
	if log.TokenID != nil {
		tokenID = *log.TokenID
	} else if log.UserTokenID != nil {
		tokenID = *log.UserTokenID
	} else {
		// 如果两个ID都为空，记录警告并跳过
		logger.Warnf("警告: 请求日志中TokenID和UserTokenID都为空\n")
		return
	}

	// 使用repository的UpdateOrCreateUsageStats方法
	if err := s.repository.UpdateOrCreateUsageStats(ctx, tokenID, time.Now(), log); err != nil {
		// 记录错误但不影响主流程
		logger.Warnf("警告: 更新使用统计失败: %v\n", err)
	}
}

// GetTokenStats 获取指定token的统计
func (s *StatsService) GetTokenStats(ctx context.Context, tokenID string) (*TokenStats, error) {
	tokenStatsResult, err := s.repository.GetTokenStats(ctx, tokenID)
	if err != nil {
		return nil, fmt.Errorf("获取令牌统计失败: %w", err)
	}

	// 转换为服务层的TokenStats结构
	stats := &TokenStats{
		TokenID:         tokenStatsResult.TokenID,
		TokenName:       tokenStatsResult.TokenName,
		TotalRequests:   tokenStatsResult.TotalRequests,
		SuccessRequests: tokenStatsResult.SuccessRequests,
		ErrorRequests:   tokenStatsResult.ErrorRequests,
		SuccessRate:     tokenStatsResult.SuccessRate,
		TotalBytes:      tokenStatsResult.TotalBytes,
		LastRequest:     tokenStatsResult.LastRequest,
	}

	return stats, nil
}

// GetUsageHistory 获取使用历史
func (s *StatsService) GetUsageHistory(ctx context.Context, tokenID string, days int) ([]*UsageStats, error) {
	historyResults, err := s.repository.GetUsageHistory(ctx, tokenID, days)
	if err != nil {
		return nil, fmt.Errorf("failed to get usage history: %w", err)
	}

	// 转换为服务层的UsageStats结构
	var stats []*UsageStats
	for _, result := range historyResults {
		stats = append(stats, &UsageStats{
			Date:     result.Date,
			Requests: result.Requests,
			Success:  result.Success,
			Errors:   result.Errors,
			Bytes:    result.Bytes,
		})
	}

	return stats, nil
}

// GetTopTokens 获取使用量最高的tokens
func (s *StatsService) GetTopTokens(ctx context.Context, limit int) ([]*TokenStats, error) {
	topTokenResults, err := s.repository.GetTopTokens(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get top tokens: %w", err)
	}

	// 转换为服务层的TokenStats结构
	var stats []*TokenStats
	for _, result := range topTokenResults {
		stats = append(stats, &TokenStats{
			TokenID:         result.TokenID,
			Token:           result.Token, // 实际的token值
			TokenName:       result.TokenName,
			TenantURL:       result.TenantAddress, // 注意字段名映射
			TotalRequests:   result.RequestCount,
			SuccessRequests: result.SuccessCount,
			ErrorRequests:   result.ErrorCount,
			SuccessRate:     result.SuccessRate,
			TotalBytes:      result.TotalBytes,
		})
	}

	return stats, nil
}

// GetRequestTrend 获取请求趋势数据
func (s *StatsService) GetRequestTrend(ctx context.Context, days int) ([]*UsageStats, error) {
	trendResults, err := s.repository.GetUsageTrend(ctx, days)
	if err != nil {
		return nil, fmt.Errorf("failed to get request trend: %w", err)
	}

	// 转换为服务层的UsageStats结构
	var stats []*UsageStats
	for _, result := range trendResults {
		stats = append(stats, &UsageStats{
			Date:     result.Date,
			Requests: result.Requests,
			Success:  result.Success,
			Errors:   result.Errors,
			Bytes:    0, // TrendResult中没有Bytes字段，设为0
		})
	}

	return stats, nil
}

// CleanupOldLogs 清理旧日志
func (s *StatsService) CleanupOldLogs(ctx context.Context, days int) (int64, error) {
	// 清理请求日志
	if err := s.requestLogRepo.CleanupOldLogs(ctx, days); err != nil {
		return 0, fmt.Errorf("清理旧请求日志失败: %w", err)
	}

	// 清理统计数据
	if err := s.repository.CleanupOldStats(ctx, days); err != nil {
		return 0, fmt.Errorf("清理旧统计数据失败: %w", err)
	}

	// 注意：由于使用了repository，我们无法直接获取删除的行数
	// 如果需要返回删除的行数，可以在repository方法中添加相应的返回值
	return 0, nil
}
