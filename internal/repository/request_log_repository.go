package repository

import (
	"context"
	"fmt"
	"time"

	"augment-gateway/internal/database"

	"gorm.io/gorm"
)

// RequestLogRepository 请求日志仓库接口
type RequestLogRepository interface {
	BaseRepository[database.RequestLog]

	// 日志记录
	LogRequest(ctx context.Context, log *database.RequestLog) error
	BatchLogRequests(ctx context.Context, logs []*database.RequestLog) error

	// 查询方法
	GetLogsByToken(ctx context.Context, tokenID string, limit int) ([]*database.RequestLog, error)
	GetLogsByTokenInDateRange(ctx context.Context, tokenID string, startDate, endDate time.Time) ([]*database.RequestLog, error)
	GetRecentLogs(ctx context.Context, limit int) ([]*database.RequestLog, error)
	GetErrorLogs(ctx context.Context, limit int) ([]*database.RequestLog, error)
	GetLogsByStatusCode(ctx context.Context, statusCode int, limit int) ([]*database.RequestLog, error)
	GetLogsByMethod(ctx context.Context, method string, limit int) ([]*database.RequestLog, error)
	GetLogsByClientIP(ctx context.Context, clientIP string, limit int) ([]*database.RequestLog, error)

	// 统计方法
	CountLogsByToken(ctx context.Context, tokenID string) (int64, error)
	CountLogsByTokenInDateRange(ctx context.Context, tokenID string, startDate, endDate time.Time) (int64, error)
	CountSuccessLogsByToken(ctx context.Context, tokenID string) (int64, error)
	CountErrorLogsByToken(ctx context.Context, tokenID string) (int64, error)
	GetLatencyStats(ctx context.Context, tokenID string) (*LatencyStats, error)
	GetRequestSizeStats(ctx context.Context, tokenID string) (*SizeStats, error)
	GetResponseSizeStats(ctx context.Context, tokenID string) (*SizeStats, error)

	// 清理方法
	CleanupOldLogs(ctx context.Context, days int) error
	CleanupLogsByToken(ctx context.Context, tokenID string) error

	// 分析方法
	GetTopUserAgents(ctx context.Context, limit int) ([]*UserAgentStats, error)
	GetTopClientIPs(ctx context.Context, limit int) ([]*ClientIPStats, error)
	GetTopPaths(ctx context.Context, limit int) ([]*PathStats, error)
	GetHourlyRequestDistribution(ctx context.Context, tokenID string, date time.Time) ([]*HourlyStats, error)
}

// LatencyStats 延迟统计
type LatencyStats struct {
	MinLatency int64   `json:"min_latency"`
	MaxLatency int64   `json:"max_latency"`
	AvgLatency float64 `json:"avg_latency"`
	P50Latency float64 `json:"p50_latency"`
	P95Latency float64 `json:"p95_latency"`
	P99Latency float64 `json:"p99_latency"`
}

// SizeStats 大小统计
type SizeStats struct {
	MinSize   int64   `json:"min_size"`
	MaxSize   int64   `json:"max_size"`
	AvgSize   float64 `json:"avg_size"`
	TotalSize int64   `json:"total_size"`
}

// UserAgentStats 用户代理统计
type UserAgentStats struct {
	UserAgent string `json:"user_agent"`
	Count     int64  `json:"count"`
}

// ClientIPStats 客户端IP统计
type ClientIPStats struct {
	ClientIP string `json:"client_ip"`
	Count    int64  `json:"count"`
}

// PathStats 路径统计
type PathStats struct {
	Path  string `json:"path"`
	Count int64  `json:"count"`
}

// HourlyStats 小时统计
type HourlyStats struct {
	Hour  int   `json:"hour"`
	Count int64 `json:"count"`
}

// requestLogRepository 请求日志仓库实现
type requestLogRepository struct {
	BaseRepository[database.RequestLog]
	db *gorm.DB
}

// NewRequestLogRepository 创建请求日志仓库
func NewRequestLogRepository(db *gorm.DB) RequestLogRepository {
	return &requestLogRepository{
		BaseRepository: NewBaseRepository[database.RequestLog](db),
		db:             db,
	}
}

// LogRequest 记录请求日志
func (r *requestLogRepository) LogRequest(ctx context.Context, log *database.RequestLog) error {
	return r.Create(ctx, log)
}

// BatchLogRequests 批量记录请求日志
func (r *requestLogRepository) BatchLogRequests(ctx context.Context, logs []*database.RequestLog) error {
	if len(logs) == 0 {
		return nil
	}

	batchSize := 100
	for i := 0; i < len(logs); i += batchSize {
		end := i + batchSize
		if end > len(logs) {
			end = len(logs)
		}

		batch := logs[i:end]
		if err := r.db.WithContext(ctx).Create(&batch).Error; err != nil {
			return fmt.Errorf("batch create failed at index %d: %w", i, err)
		}
	}

	return nil
}

// GetLogsByToken 获取指定Token的日志（支持系统TOKEN和用户令牌）
func (r *requestLogRepository) GetLogsByToken(ctx context.Context, tokenID string, limit int) ([]*database.RequestLog, error) {
	query := NewQueryBuilder().
		Where("token_id = ? OR user_token_id = ?", tokenID, tokenID).
		OrderByDesc("created_at").
		Limit(limit)

	return r.List(ctx, query)
}

// GetLogsByTokenInDateRange 获取指定Token在日期范围内的日志（支持系统TOKEN和用户令牌）
func (r *requestLogRepository) GetLogsByTokenInDateRange(ctx context.Context, tokenID string, startDate, endDate time.Time) ([]*database.RequestLog, error) {
	query := NewQueryBuilder().
		Where("token_id = ? OR user_token_id = ?", tokenID, tokenID).
		Between("created_at", startDate, endDate).
		OrderByDesc("created_at")

	return r.List(ctx, query)
}

// GetRecentLogs 获取最近的日志
func (r *requestLogRepository) GetRecentLogs(ctx context.Context, limit int) ([]*database.RequestLog, error) {
	query := NewQueryBuilder().
		OrderByDesc("created_at").
		Limit(limit)

	return r.List(ctx, query)
}

// GetErrorLogs 获取错误日志
func (r *requestLogRepository) GetErrorLogs(ctx context.Context, limit int) ([]*database.RequestLog, error) {
	query := NewQueryBuilder().
		Where("status_code >= 400").
		OrderByDesc("created_at").
		Limit(limit)

	return r.List(ctx, query)
}

// GetLogsByStatusCode 根据状态码获取日志
func (r *requestLogRepository) GetLogsByStatusCode(ctx context.Context, statusCode int, limit int) ([]*database.RequestLog, error) {
	query := NewQueryBuilder().
		WhereEq("status_code", statusCode).
		OrderByDesc("created_at").
		Limit(limit)

	return r.List(ctx, query)
}

// GetLogsByMethod 根据请求方法获取日志
func (r *requestLogRepository) GetLogsByMethod(ctx context.Context, method string, limit int) ([]*database.RequestLog, error) {
	query := NewQueryBuilder().
		WhereEq("method", method).
		OrderByDesc("created_at").
		Limit(limit)

	return r.List(ctx, query)
}

// GetLogsByClientIP 根据客户端IP获取日志
func (r *requestLogRepository) GetLogsByClientIP(ctx context.Context, clientIP string, limit int) ([]*database.RequestLog, error) {
	query := NewQueryBuilder().
		WhereEq("client_ip", clientIP).
		OrderByDesc("created_at").
		Limit(limit)

	return r.List(ctx, query)
}

// CountLogsByToken 统计指定Token的日志数量（支持系统TOKEN和用户令牌）
func (r *requestLogRepository) CountLogsByToken(ctx context.Context, tokenID string) (int64, error) {
	query := NewQueryBuilder().Where("token_id = ? OR user_token_id = ?", tokenID, tokenID)
	return r.Count(ctx, query)
}

// CountLogsByTokenInDateRange 统计指定Token在日期范围内的日志数量
func (r *requestLogRepository) CountLogsByTokenInDateRange(ctx context.Context, tokenID string, startDate, endDate time.Time) (int64, error) {
	query := NewQueryBuilder().
		WhereEq("token_id", tokenID).
		Between("created_at", startDate, endDate)

	return r.Count(ctx, query)
}

// CountSuccessLogsByToken 统计指定Token的成功请求数量
func (r *requestLogRepository) CountSuccessLogsByToken(ctx context.Context, tokenID string) (int64, error) {
	query := NewQueryBuilder().
		WhereEq("token_id", tokenID).
		Where("status_code >= 200 AND status_code < 400")

	return r.Count(ctx, query)
}

// CountErrorLogsByToken 统计指定Token的错误请求数量
func (r *requestLogRepository) CountErrorLogsByToken(ctx context.Context, tokenID string) (int64, error) {
	query := NewQueryBuilder().
		WhereEq("token_id", tokenID).
		Where("status_code >= 400")

	return r.Count(ctx, query)
}

// GetLatencyStats 获取延迟统计
func (r *requestLogRepository) GetLatencyStats(ctx context.Context, tokenID string) (*LatencyStats, error) {
	var stats LatencyStats

	err := r.db.WithContext(ctx).
		Model(&database.RequestLog{}).
		Where("token_id = ? OR user_token_id = ?", tokenID, tokenID).
		Select(`
			MIN(latency) as min_latency,
			MAX(latency) as max_latency,
			AVG(latency) as avg_latency
		`).
		Scan(&stats).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get latency stats: %w", err)
	}

	// 获取百分位数（简化实现）
	var latencies []int64
	err = r.db.WithContext(ctx).
		Model(&database.RequestLog{}).
		Where("token_id = ? OR user_token_id = ?", tokenID, tokenID).
		Order("latency ASC").
		Pluck("latency", &latencies).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get latency percentiles: %w", err)
	}

	if len(latencies) > 0 {
		stats.P50Latency = float64(latencies[len(latencies)*50/100])
		stats.P95Latency = float64(latencies[len(latencies)*95/100])
		stats.P99Latency = float64(latencies[len(latencies)*99/100])
	}

	return &stats, nil
}

// GetRequestSizeStats 获取请求大小统计
func (r *requestLogRepository) GetRequestSizeStats(ctx context.Context, tokenID string) (*SizeStats, error) {
	var stats SizeStats

	err := r.db.WithContext(ctx).
		Model(&database.RequestLog{}).
		Where("token_id = ?", tokenID).
		Select(`
			MIN(request_size) as min_size,
			MAX(request_size) as max_size,
			AVG(request_size) as avg_size,
			SUM(request_size) as total_size
		`).
		Scan(&stats).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get request size stats: %w", err)
	}

	return &stats, nil
}

// GetResponseSizeStats 获取响应大小统计
func (r *requestLogRepository) GetResponseSizeStats(ctx context.Context, tokenID string) (*SizeStats, error) {
	var stats SizeStats

	err := r.db.WithContext(ctx).
		Model(&database.RequestLog{}).
		Where("token_id = ?", tokenID).
		Select(`
			MIN(response_size) as min_size,
			MAX(response_size) as max_size,
			AVG(response_size) as avg_size,
			SUM(response_size) as total_size
		`).
		Scan(&stats).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get response size stats: %w", err)
	}

	return &stats, nil
}

// CleanupOldLogs 清理旧日志
func (r *requestLogRepository) CleanupOldLogs(ctx context.Context, days int) error {
	cutoff := time.Now().AddDate(0, 0, -days)
	return r.db.WithContext(ctx).Where("created_at < ?", cutoff).Delete(&database.RequestLog{}).Error
}

// CleanupLogsByToken 清理指定Token的日志
func (r *requestLogRepository) CleanupLogsByToken(ctx context.Context, tokenID string) error {
	return r.db.WithContext(ctx).Where("token_id = ?", tokenID).Delete(&database.RequestLog{}).Error
}

// GetTopUserAgents 获取热门用户代理
func (r *requestLogRepository) GetTopUserAgents(ctx context.Context, limit int) ([]*UserAgentStats, error) {
	var results []*UserAgentStats

	err := r.db.WithContext(ctx).
		Model(&database.RequestLog{}).
		Select("user_agent, COUNT(*) as count").
		Group("user_agent").
		Order("count DESC").
		Limit(limit).
		Scan(&results).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get top user agents: %w", err)
	}

	return results, nil
}

// GetTopClientIPs 获取热门客户端IP
func (r *requestLogRepository) GetTopClientIPs(ctx context.Context, limit int) ([]*ClientIPStats, error) {
	var results []*ClientIPStats

	err := r.db.WithContext(ctx).
		Model(&database.RequestLog{}).
		Select("client_ip, COUNT(*) as count").
		Group("client_ip").
		Order("count DESC").
		Limit(limit).
		Scan(&results).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get top client IPs: %w", err)
	}

	return results, nil
}

// GetTopPaths 获取热门路径
func (r *requestLogRepository) GetTopPaths(ctx context.Context, limit int) ([]*PathStats, error) {
	var results []*PathStats

	err := r.db.WithContext(ctx).
		Model(&database.RequestLog{}).
		Select("path, COUNT(*) as count").
		Group("path").
		Order("count DESC").
		Limit(limit).
		Scan(&results).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get top paths: %w", err)
	}

	return results, nil
}

// GetHourlyRequestDistribution 获取小时请求分布
func (r *requestLogRepository) GetHourlyRequestDistribution(ctx context.Context, tokenID string, date time.Time) ([]*HourlyStats, error) {
	var results []*HourlyStats

	dateStr := date.Format("2006-01-02")

	err := r.db.WithContext(ctx).
		Model(&database.RequestLog{}).
		Select("HOUR(created_at) as hour, COUNT(*) as count").
		Where("token_id = ? AND DATE(created_at) = ?", tokenID, dateStr).
		Group("HOUR(created_at)").
		Order("hour ASC").
		Scan(&results).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get hourly request distribution: %w", err)
	}

	return results, nil
}

// 便捷查询方法

// GetTodayLogs 获取今天的日志
func (r *requestLogRepository) GetTodayLogs(ctx context.Context, tokenID string) ([]*database.RequestLog, error) {
	query := NewQueryBuilder().
		WhereEq("token_id", tokenID).
		WhereToday("created_at").
		OrderByDesc("created_at")

	return r.List(ctx, query)
}

// GetThisWeekLogs 获取本周的日志
func (r *requestLogRepository) GetThisWeekLogs(ctx context.Context, tokenID string) ([]*database.RequestLog, error) {
	query := NewQueryBuilder().
		WhereEq("token_id", tokenID).
		WhereThisWeek("created_at").
		OrderByDesc("created_at")

	return r.List(ctx, query)
}

// GetThisMonthLogs 获取本月的日志
func (r *requestLogRepository) GetThisMonthLogs(ctx context.Context, tokenID string) ([]*database.RequestLog, error) {
	query := NewQueryBuilder().
		WhereEq("token_id", tokenID).
		WhereThisMonth("created_at").
		OrderByDesc("created_at")

	return r.List(ctx, query)
}

// GetSlowRequests 获取慢请求
func (r *requestLogRepository) GetSlowRequests(ctx context.Context, thresholdMs int64, limit int) ([]*database.RequestLog, error) {
	query := NewQueryBuilder().
		WhereGt("latency", thresholdMs*1000). // 转换为微秒
		OrderByDesc("latency").
		Limit(limit)

	return r.List(ctx, query)
}

// GetLargeRequests 获取大请求
func (r *requestLogRepository) GetLargeRequests(ctx context.Context, thresholdBytes int64, limit int) ([]*database.RequestLog, error) {
	query := NewQueryBuilder().
		WhereGt("request_size", thresholdBytes).
		OrderByDesc("request_size").
		Limit(limit)

	return r.List(ctx, query)
}

// GetLargeResponses 获取大响应
func (r *requestLogRepository) GetLargeResponses(ctx context.Context, thresholdBytes int64, limit int) ([]*database.RequestLog, error) {
	query := NewQueryBuilder().
		WhereGt("response_size", thresholdBytes).
		OrderByDesc("response_size").
		Limit(limit)

	return r.List(ctx, query)
}
