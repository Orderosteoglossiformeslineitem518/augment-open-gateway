package repository

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"augment-gateway/internal/database"

	"gorm.io/gorm"
)

// StatsRepository 统计仓库接口
type StatsRepository interface {
	BaseRepository[database.UsageStats]

	// 概览统计
	GetOverviewStats(ctx context.Context) (*OverviewStats, error)

	// Token统计
	GetTokenStats(ctx context.Context, tokenID string) (*TokenStatsResult, error)
	GetTopTokens(ctx context.Context, limit int) ([]*TopTokenResult, error)

	// 使用历史
	GetUsageHistory(ctx context.Context, tokenID string, days int) ([]*UsageHistoryResult, error)
	GetUsageTrend(ctx context.Context, days int) ([]*TrendResult, error)

	// 请求统计
	GetRequestStats(ctx context.Context, tokenID string) (*RequestStats, error)
	GetRequestStatsInDateRange(ctx context.Context, tokenID string, startDate, endDate time.Time) (*RequestStats, error)

	// 日期统计
	GetDailyStats(ctx context.Context, date time.Time) (*DailyStats, error)
	GetWeeklyStats(ctx context.Context, year, week int) (*WeeklyStats, error)
	GetMonthlyStats(ctx context.Context, year, month int) (*MonthlyStats, error)

	// 更新统计
	UpdateOrCreateUsageStats(ctx context.Context, tokenID string, date time.Time, requestLog *database.RequestLog) error

	// 清理统计
	CleanupOldStats(ctx context.Context, days int) error
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
}

// TokenStatsResult Token统计结果
type TokenStatsResult struct {
	TokenID         string     `json:"token_id"`
	TokenName       string     `json:"token_name"`
	TotalRequests   int64      `json:"total_requests"`
	SuccessRequests int64      `json:"success_requests"`
	ErrorRequests   int64      `json:"error_requests"`
	SuccessRate     float64    `json:"success_rate"`
	TotalBytes      int64      `json:"total_bytes"`
	LastRequest     *time.Time `json:"last_request"`
}

// TopTokenResult 热门Token结果
type TopTokenResult struct {
	TokenID       string  `json:"token_id"`
	Token         string  `json:"token"` // 实际的token值
	TokenName     string  `json:"token_name"`
	TenantAddress string  `json:"tenant_address"`
	RequestCount  int64   `json:"request_count"`
	SuccessCount  int64   `json:"success_count"`
	ErrorCount    int64   `json:"error_count"`
	TotalBytes    int64   `json:"total_bytes"`
	SuccessRate   float64 `json:"success_rate"`
}

// UsageHistoryResult 使用历史结果
type UsageHistoryResult struct {
	Date     string `json:"date"`
	Requests int64  `json:"requests"`
	Success  int64  `json:"success"`
	Errors   int64  `json:"errors"`
	Bytes    int64  `json:"bytes"`
}

// TrendResult 趋势结果
type TrendResult struct {
	Date     string `json:"date"`
	Requests int64  `json:"requests"`
	Success  int64  `json:"success"`
	Errors   int64  `json:"errors"`
}

// RequestStats 请求统计
type RequestStats struct {
	TotalRequests   int64   `json:"total_requests"`
	SuccessRequests int64   `json:"success_requests"`
	ErrorRequests   int64   `json:"error_requests"`
	SuccessRate     float64 `json:"success_rate"`
	TotalBytes      int64   `json:"total_bytes"`
	AvgLatency      float64 `json:"avg_latency"`
}

// DailyStats 日统计
type DailyStats struct {
	Date            time.Time `json:"date"`
	TotalRequests   int64     `json:"total_requests"`
	SuccessRequests int64     `json:"success_requests"`
	ErrorRequests   int64     `json:"error_requests"`
	TotalBytes      int64     `json:"total_bytes"`
	UniqueTokens    int64     `json:"unique_tokens"`
}

// WeeklyStats 周统计
type WeeklyStats struct {
	Year            int   `json:"year"`
	Week            int   `json:"week"`
	TotalRequests   int64 `json:"total_requests"`
	SuccessRequests int64 `json:"success_requests"`
	ErrorRequests   int64 `json:"error_requests"`
	TotalBytes      int64 `json:"total_bytes"`
	UniqueTokens    int64 `json:"unique_tokens"`
}

// MonthlyStats 月统计
type MonthlyStats struct {
	Year            int   `json:"year"`
	Month           int   `json:"month"`
	TotalRequests   int64 `json:"total_requests"`
	SuccessRequests int64 `json:"success_requests"`
	ErrorRequests   int64 `json:"error_requests"`
	TotalBytes      int64 `json:"total_bytes"`
	UniqueTokens    int64 `json:"unique_tokens"`
}

// statsRepository 统计仓库实现
type statsRepository struct {
	BaseRepository[database.UsageStats]
	db *gorm.DB
}

// NewStatsRepository 创建统计仓库
func NewStatsRepository(db *gorm.DB) StatsRepository {
	return &statsRepository{
		BaseRepository: NewBaseRepository[database.UsageStats](db),
		db:             db,
	}
}

// GetOverviewStats 获取概览统计
func (r *statsRepository) GetOverviewStats(ctx context.Context) (*OverviewStats, error) {
	stats := &OverviewStats{}

	// 获取Token统计
	if err := r.db.WithContext(ctx).Model(&database.Token{}).Count(&stats.TotalTokens).Error; err != nil {
		return nil, fmt.Errorf("failed to get total tokens: %w", err)
	}

	if err := r.db.WithContext(ctx).Model(&database.Token{}).Where("status = ?", "active").Count(&stats.ActiveTokens).Error; err != nil {
		return nil, fmt.Errorf("failed to get active tokens: %w", err)
	}

	if err := r.db.WithContext(ctx).Model(&database.Token{}).Where("status = ?", "expired").Count(&stats.ExpiredTokens).Error; err != nil {
		return nil, fmt.Errorf("failed to get expired tokens: %w", err)
	}

	if err := r.db.WithContext(ctx).Model(&database.Token{}).Where("status = ?", "disabled").Count(&stats.DisabledTokens).Error; err != nil {
		return nil, fmt.Errorf("failed to get disabled tokens: %w", err)
	}

	// 获取请求统计
	if err := r.db.WithContext(ctx).Model(&database.RequestLog{}).Count(&stats.TotalRequests).Error; err != nil {
		return nil, fmt.Errorf("failed to get total requests: %w", err)
	}

	if err := r.db.WithContext(ctx).Model(&database.RequestLog{}).
		Where("status_code = 200").
		Count(&stats.SuccessRequests).Error; err != nil {
		return nil, fmt.Errorf("获取成功请求数失败: %w", err)
	}

	stats.ErrorRequests = stats.TotalRequests - stats.SuccessRequests
	if stats.TotalRequests > 0 {
		// 计算成功率并保留两位小数
		successRate := float64(stats.SuccessRequests) / float64(stats.TotalRequests) * 100
		stats.SuccessRate = math.Round(successRate*100) / 100
	}

	return stats, nil
}

// GetTokenStats 获取Token统计
func (r *statsRepository) GetTokenStats(ctx context.Context, tokenID string) (*TokenStatsResult, error) {
	// 获取Token信息
	var token database.Token
	if err := r.db.WithContext(ctx).Where("id = ?", tokenID).First(&token).Error; err != nil {
		return nil, fmt.Errorf("token not found: %w", err)
	}

	stats := &TokenStatsResult{
		TokenID:   tokenID,
		TokenName: token.Name,
	}

	// 获取请求统计
	var result struct {
		TotalRequests   int64      `json:"total_requests"`
		SuccessRequests int64      `json:"success_requests"`
		TotalBytes      int64      `json:"total_bytes"`
		LastRequest     *time.Time `json:"last_request"`
	}

	err := r.db.WithContext(ctx).
		Model(&database.RequestLog{}).
		Where("token_id = ? OR user_token_id = ?", tokenID, tokenID).
		Select(`
			COUNT(*) as total_requests,
			SUM(CASE WHEN status_code = 200 THEN 1 ELSE 0 END) as success_requests,
			SUM(request_size + response_size) as total_bytes,
			MAX(created_at) as last_request
		`).
		Scan(&result).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get token stats: %w", err)
	}

	stats.TotalRequests = result.TotalRequests
	stats.SuccessRequests = result.SuccessRequests
	stats.ErrorRequests = result.TotalRequests - result.SuccessRequests
	stats.TotalBytes = result.TotalBytes
	stats.LastRequest = result.LastRequest

	if stats.TotalRequests > 0 {
		// 计算成功率并保留两位小数
		successRate := float64(stats.SuccessRequests) / float64(stats.TotalRequests) * 100
		stats.SuccessRate = math.Round(successRate*100) / 100
	}

	return stats, nil
}

// GetTopTokens 获取热门Token - 基于tokens表的used_requests字段排序，从request_logs表计算成功率
func (r *statsRepository) GetTopTokens(ctx context.Context, limit int) ([]*TopTokenResult, error) {
	var results []*TopTokenResult

	// 首先从tokens表获取使用量最多的前N个token，使用Model确保逻辑删除生效
	err := r.db.WithContext(ctx).
		Model(&database.Token{}).
		Select(`
			id as token_id,
			token as token,
			name as token_name,
			tenant_address,
			used_requests as request_count,
			0 as success_count,
			0 as error_count,
			0 as total_bytes,
			0 as success_rate
		`).
		Where("status = ?", "active").
		Order("used_requests DESC").
		Limit(limit).
		Scan(&results).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get top tokens: %w", err)
	}

	// 如果没有数据，直接返回
	if len(results) == 0 {
		return results, nil
	}

	// 批量查询所有token的request_logs统计数据（性能优化：一次查询）
	var tokenIDs []string
	for _, result := range results {
		tokenIDs = append(tokenIDs, result.TokenID)
	}

	// 批量查询request_logs表中的统计数据
	type TokenStats struct {
		TokenID      string `json:"token_id"`
		TotalCount   int64  `json:"total_count"`
		SuccessCount int64  `json:"success_count"`
		ErrorCount   int64  `json:"error_count"`
		TotalBytes   int64  `json:"total_bytes"`
	}

	var statsResults []TokenStats
	err = r.db.WithContext(ctx).
		Table("request_logs").
		Select(`
			COALESCE(token_id, user_token_id) as token_id,
			COUNT(*) as total_count,
			COUNT(CASE WHEN status_code = 200 THEN 1 END) as success_count,
			COUNT(CASE WHEN status_code != 200 THEN 1 END) as error_count,
			COALESCE(SUM(request_size + response_size), 0) as total_bytes
		`).
		Where("token_id IN ? OR user_token_id IN ?", tokenIDs, tokenIDs).
		Group("COALESCE(token_id, user_token_id)").
		Scan(&statsResults).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get request logs stats: %w", err)
	}

	// 创建统计数据映射表，便于快速查找
	statsMap := make(map[string]TokenStats)
	for _, stats := range statsResults {
		statsMap[stats.TokenID] = stats
	}

	// 更新结果数据，计算成功率
	for i, result := range results {
		if stats, exists := statsMap[result.TokenID]; exists {
			results[i].SuccessCount = stats.SuccessCount
			results[i].ErrorCount = stats.ErrorCount
			results[i].TotalBytes = stats.TotalBytes

			// 计算成功率并保留两位小数
			if stats.TotalCount > 0 {
				successRate := float64(stats.SuccessCount) / float64(stats.TotalCount) * 100
				results[i].SuccessRate = math.Round(successRate*100) / 100
			} else {
				results[i].SuccessRate = 0
			}
		}
		// 如果没有找到对应的统计数据，保持默认值（0）
	}

	return results, nil
}

// GetUsageHistory 获取使用历史 - 基于request_logs表
func (r *statsRepository) GetUsageHistory(ctx context.Context, tokenID string, days int) ([]*UsageHistoryResult, error) {
	var results []*UsageHistoryResult

	startDate := time.Now().AddDate(0, 0, -days)

	err := r.db.WithContext(ctx).
		Table("request_logs").
		Select(`
			DATE(created_at) as date,
			COUNT(CASE WHEN status_code = 200 THEN 1 END) as success,
			COUNT(CASE WHEN status_code != 200 THEN 1 END) as errors,
			COUNT(*) as requests,
			COALESCE(SUM(request_size + response_size), 0) as bytes
		`).
		Where("(token_id = ? OR user_token_id = ?) AND created_at >= ?", tokenID, tokenID, startDate).
		Group("DATE(created_at)").
		Order("date ASC").
		Scan(&results).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get usage history: %w", err)
	}

	return results, nil
}

// GetUsageTrend 获取使用趋势 - 基于request_logs表
func (r *statsRepository) GetUsageTrend(ctx context.Context, days int) ([]*TrendResult, error) {
	var results []*TrendResult

	startDate := time.Now().AddDate(0, 0, -days)

	err := r.db.WithContext(ctx).
		Table("request_logs").
		Select(`
			DATE_FORMAT(created_at, '%m-%d') as date,
			COUNT(CASE WHEN status_code = 200 THEN 1 END) as success,
			COUNT(CASE WHEN status_code != 200 THEN 1 END) as errors,
			COUNT(*) as requests
		`).
		Where("created_at >= ?", startDate).
		Group("DATE_FORMAT(created_at, '%m-%d')").
		Order("MIN(created_at) ASC").
		Scan(&results).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get usage trend: %w", err)
	}

	return results, nil
}

// GetRequestStats 获取请求统计
func (r *statsRepository) GetRequestStats(ctx context.Context, tokenID string) (*RequestStats, error) {
	var result RequestStats

	err := r.db.WithContext(ctx).
		Model(&database.RequestLog{}).
		Where("token_id = ? OR user_token_id = ?", tokenID, tokenID).
		Select(`
			COUNT(*) as total_requests,
			SUM(CASE WHEN status_code = 200 THEN 1 ELSE 0 END) as success_requests,
			SUM(request_size + response_size) as total_bytes,
			AVG(latency) as avg_latency
		`).
		Scan(&result).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get request stats: %w", err)
	}

	result.ErrorRequests = result.TotalRequests - result.SuccessRequests
	if result.TotalRequests > 0 {
		// 计算成功率并保留两位小数
		successRate := float64(result.SuccessRequests) / float64(result.TotalRequests) * 100
		result.SuccessRate = math.Round(successRate*100) / 100
	}

	return &result, nil
}

// GetRequestStatsInDateRange 获取指定日期范围的请求统计
func (r *statsRepository) GetRequestStatsInDateRange(ctx context.Context, tokenID string, startDate, endDate time.Time) (*RequestStats, error) {
	var result RequestStats

	err := r.db.WithContext(ctx).
		Model(&database.RequestLog{}).
		Where("(token_id = ? OR user_token_id = ?) AND created_at BETWEEN ? AND ?", tokenID, tokenID, startDate, endDate).
		Select(`
			COUNT(*) as total_requests,
			SUM(CASE WHEN status_code = 200 THEN 1 ELSE 0 END) as success_requests,
			SUM(request_size + response_size) as total_bytes,
			AVG(latency) as avg_latency
		`).
		Scan(&result).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get request stats in date range: %w", err)
	}

	result.ErrorRequests = result.TotalRequests - result.SuccessRequests
	if result.TotalRequests > 0 {
		// 计算成功率并保留两位小数
		successRate := float64(result.SuccessRequests) / float64(result.TotalRequests) * 100
		result.SuccessRate = math.Round(successRate*100) / 100
	}

	return &result, nil
}

// GetDailyStats 获取日统计
func (r *statsRepository) GetDailyStats(ctx context.Context, date time.Time) (*DailyStats, error) {
	dateStr := date.Format("2006-01-02")
	var result DailyStats

	err := r.db.WithContext(ctx).
		Table("usage_stats").
		Select(`
			? as date,
			SUM(request_count) as total_requests,
			SUM(success_count) as success_requests,
			SUM(error_count) as error_requests,
			SUM(total_bytes) as total_bytes,
			COUNT(DISTINCT token_id) as unique_tokens
		`, date).
		Where("DATE(date) = ?", dateStr).
		Scan(&result).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get daily stats: %w", err)
	}

	return &result, nil
}

// GetWeeklyStats 获取周统计
func (r *statsRepository) GetWeeklyStats(ctx context.Context, year, week int) (*WeeklyStats, error) {
	var result WeeklyStats

	err := r.db.WithContext(ctx).
		Table("usage_stats").
		Select(`
			? as year,
			? as week,
			SUM(request_count) as total_requests,
			SUM(success_count) as success_requests,
			SUM(error_count) as error_requests,
			SUM(total_bytes) as total_bytes,
			COUNT(DISTINCT token_id) as unique_tokens
		`, year, week).
		Where("YEAR(date) = ? AND WEEK(date) = ?", year, week).
		Scan(&result).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get weekly stats: %w", err)
	}

	return &result, nil
}

// GetMonthlyStats 获取月统计
func (r *statsRepository) GetMonthlyStats(ctx context.Context, year, month int) (*MonthlyStats, error) {
	var result MonthlyStats

	err := r.db.WithContext(ctx).
		Table("usage_stats").
		Select(`
			? as year,
			? as month,
			SUM(request_count) as total_requests,
			SUM(success_count) as success_requests,
			SUM(error_count) as error_requests,
			SUM(total_bytes) as total_bytes,
			COUNT(DISTINCT token_id) as unique_tokens
		`, year, month).
		Where("YEAR(date) = ? AND MONTH(date) = ?", year, month).
		Scan(&result).Error

	if err != nil {
		return nil, fmt.Errorf("获取月统计失败: %w", err)
	}

	return &result, nil
}

// UpdateOrCreateUsageStats 更新或创建使用统计
func (r *statsRepository) UpdateOrCreateUsageStats(ctx context.Context, tokenID string, date time.Time, requestLog *database.RequestLog) error {
	dateStr := date.Format("2006-01-02")

	// 检查是否已存在统计记录
	var stats database.UsageStats
	err := r.db.WithContext(ctx).Where("token_id = ? AND DATE(date) = ?", tokenID, dateStr).First(&stats).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 创建新的统计记录
		stats = database.UsageStats{
			TokenID:      tokenID,
			RequestCount: 1,
			Date:         date,
		}

		if requestLog.StatusCode == 200 {
			stats.SuccessCount = 1
		} else {
			stats.ErrorCount = 1
		}

		stats.TotalBytes = requestLog.RequestSize + requestLog.ResponseSize
		stats.AvgLatency = float64(requestLog.Latency) / 1000.0 // 转换为毫秒

		return r.db.WithContext(ctx).Create(&stats).Error
	} else if err != nil {
		return fmt.Errorf("检查现有统计失败: %w", err)
	}

	// 更新现有统计
	updates := map[string]interface{}{
		"request_count": gorm.Expr("request_count + 1"),
		"total_bytes":   gorm.Expr("total_bytes + ?", requestLog.RequestSize+requestLog.ResponseSize),
	}

	if requestLog.StatusCode == 200 {
		updates["success_count"] = gorm.Expr("success_count + 1")
	} else {
		updates["error_count"] = gorm.Expr("error_count + 1")
	}

	// 更新平均延迟
	latencyMs := float64(requestLog.Latency) / 1000.0
	updates["avg_latency"] = gorm.Expr("(avg_latency * (request_count - 1) + ?) / request_count", latencyMs)

	return r.db.WithContext(ctx).Model(&stats).Updates(updates).Error
}

// CleanupOldStats 清理旧统计数据
func (r *statsRepository) CleanupOldStats(ctx context.Context, days int) error {
	cutoff := time.Now().AddDate(0, 0, -days)
	return r.db.WithContext(ctx).Where("date < ?", cutoff).Delete(&database.UsageStats{}).Error
}
