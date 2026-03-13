package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"augment-gateway/internal/database"

	"gorm.io/gorm"
)

// parseInt 解析字符串为整数
func parseInt(s string) int {
	v, _ := strconv.Atoi(s)
	return v
}

// UserUsageStatsService 用户使用统计服务
type UserUsageStatsService struct {
	db    *gorm.DB
	cache *CacheService
}

// NewUserUsageStatsService 创建用户使用统计服务
func NewUserUsageStatsService(db *gorm.DB) *UserUsageStatsService {
	return &UserUsageStatsService{
		db: db,
	}
}

// SetCacheService 设置缓存服务
func (s *UserUsageStatsService) SetCacheService(cache *CacheService) {
	s.cache = cache
}

// UserDailyStatsRequest 用户每日统计请求
type UserDailyStatsRequest struct {
	StartDate string `form:"start_date" json:"start_date"` // 开始日期 YYYY-MM-DD
	EndDate   string `form:"end_date" json:"end_date"`     // 结束日期 YYYY-MM-DD
	Days      int    `form:"days" json:"days"`             // 最近N天（与日期范围二选一）
}

// UserDailyStatsResponse 用户每日统计响应
type UserDailyStatsResponse struct {
	Stats []DailyStats `json:"stats"`
	Total StatsTotal   `json:"total"`
}

// DailyStats 每日统计
type DailyStats struct {
	Date          string `json:"date"`           // 日期 YYYY-MM-DD
	RequestCount  int    `json:"request_count"`  // 请求次数（总数）
	OfficialCount int    `json:"official_count"` // 官方请求次数
	ExternalCount int    `json:"external_count"` // 外部渠道请求次数
}

// StatsTotal 统计汇总
type StatsTotal struct {
	RequestCount  int `json:"request_count"`  // 请求次数（总数）
	OfficialCount int `json:"official_count"` // 官方请求次数
	ExternalCount int `json:"external_count"` // 外部渠道请求次数
}

// RequestLogDailyStats 请求日志每日统计结构（用于SQL查询结果）
type RequestLogDailyStats struct {
	Date          string `gorm:"column:date"`
	RequestCount  int    `gorm:"column:request_count"`
	OfficialCount int    `gorm:"column:official_count"`
	ExternalCount int    `gorm:"column:external_count"`
}

// usageStatsCacheKey 生成使用统计缓存键
func (s *UserUsageStatsService) usageStatsCacheKey(userID uint, days int, startDate, endDate string) string {
	if days > 0 {
		return fmt.Sprintf("AUGMENT-GATEWAY:user_usage_stats:%d:days:%d", userID, days)
	}
	return fmt.Sprintf("AUGMENT-GATEWAY:user_usage_stats:%d:%s:%s", userID, startDate, endDate)
}

// GetUserDailyStats 获取用户每日统计
// 从 request_logs 表查询，按官方请求和外部渠道请求区分统计
// 支持5分钟缓存
func (s *UserUsageStatsService) GetUserDailyStats(userID uint, req *UserDailyStatsRequest) (*UserDailyStatsResponse, error) {
	var startDate, endDate time.Time
	var err error

	// 处理日期范围
	if req.Days > 0 {
		endDate = time.Now()
		startDate = endDate.AddDate(0, 0, -req.Days+1)
	} else if req.StartDate != "" && req.EndDate != "" {
		startDate, err = time.Parse("2006-01-02", req.StartDate)
		if err != nil {
			return nil, err
		}
		endDate, err = time.Parse("2006-01-02", req.EndDate)
		if err != nil {
			return nil, err
		}
	} else {
		// 默认最近7天
		endDate = time.Now()
		startDate = endDate.AddDate(0, 0, -6)
	}

	// 截取日期部分
	startDate = time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, startDate.Location())
	endDate = time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 23, 59, 59, 0, endDate.Location())

	// 尝试从缓存获取
	cacheKey := s.usageStatsCacheKey(userID, req.Days, req.StartDate, req.EndDate)
	if s.cache != nil {
		ctx := context.Background()
		cachedData, err := s.cache.GetClient().Get(ctx, cacheKey).Result()
		if err == nil && cachedData != "" {
			var response UserDailyStatsResponse
			if json.Unmarshal([]byte(cachedData), &response) == nil {
				return &response, nil
			}
		}
	}

	// 格式化日期字符串用于查询
	startDateStr := startDate.Format("2006-01-02") + " 00:00:00"
	endDateStr := endDate.Format("2006-01-02") + " 23:59:59"
	userIDStr := fmt.Sprintf("%d", userID)

	// 优化SQL：使用索引友好的查询
	// request_logs 表应有 (user_token_id, created_at) 复合索引
	sql := `
		SELECT 
			DATE_FORMAT(created_at, '%Y-%m-%d') as date,
			COUNT(*) as request_count,
			CAST(SUM(CASE WHEN error_message NOT LIKE '%外部渠道%' OR error_message IS NULL OR error_message = '' THEN 1 ELSE 0 END) AS SIGNED) as official_count,
			CAST(SUM(CASE WHEN error_message LIKE '%外部渠道%' THEN 1 ELSE 0 END) AS SIGNED) as external_count
		FROM request_logs
		WHERE user_token_id = ? AND created_at BETWEEN ? AND ?
		GROUP BY DATE_FORMAT(created_at, '%Y-%m-%d')
		ORDER BY date ASC
	`

	var results []RequestLogDailyStats
	if err := s.db.Raw(sql, userIDStr, startDateStr, endDateStr).Scan(&results).Error; err != nil {
		return nil, err
	}

	// 构建日期映射
	statsMap := make(map[string]RequestLogDailyStats)
	for _, stat := range results {
		statsMap[stat.Date] = stat
	}

	// 生成完整的日期范围数据
	dailyStats := make([]DailyStats, 0)
	total := StatsTotal{}

	for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
		dateStr := d.Format("2006-01-02")
		daily := DailyStats{Date: dateStr}

		if stat, exists := statsMap[dateStr]; exists {
			daily.RequestCount = stat.RequestCount
			daily.OfficialCount = stat.OfficialCount
			daily.ExternalCount = stat.ExternalCount

			total.RequestCount += stat.RequestCount
			total.OfficialCount += stat.OfficialCount
			total.ExternalCount += stat.ExternalCount
		}

		dailyStats = append(dailyStats, daily)
	}

	response := &UserDailyStatsResponse{
		Stats: dailyStats,
		Total: total,
	}

	// 缓存结果（5分钟）
	if s.cache != nil {
		ctx := context.Background()
		if data, err := json.Marshal(response); err == nil {
			s.cache.GetClient().Set(ctx, cacheKey, string(data), 5*time.Minute)
		}
	}

	return response, nil
}

// IncrementUserStats 增加用户统计（记录请求）
func (s *UserUsageStatsService) IncrementUserStats(userID uint, success bool, credits int) error {
	today := time.Now()
	dateOnly := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())

	// 尝试更新现有记录
	var stats database.UserUsageStats
	err := s.db.Where("user_id = ? AND date = ?", userID, dateOnly).First(&stats).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// 创建新记录
			stats = database.UserUsageStats{
				UserID:       userID,
				Date:         dateOnly,
				RequestCount: 1,
				TotalCredits: credits,
			}
			if success {
				stats.SuccessCount = 1
			} else {
				stats.ErrorCount = 1
			}
			return s.db.Create(&stats).Error
		}
		return err
	}

	// 更新现有记录
	updates := map[string]interface{}{
		"request_count": gorm.Expr("request_count + 1"),
		"total_credits": gorm.Expr("total_credits + ?", credits),
	}
	if success {
		updates["success_count"] = gorm.Expr("success_count + 1")
	} else {
		updates["error_count"] = gorm.Expr("error_count + 1")
	}

	return s.db.Model(&stats).Updates(updates).Error
}

// GetUserStatsOverview 获取用户统计概览
func (s *UserUsageStatsService) GetUserStatsOverview(userID uint) (map[string]interface{}, error) {
	// 今日统计
	today := time.Now()
	todayDate := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())

	var todayStats database.UserUsageStats
	s.db.Where("user_id = ? AND date = ?", userID, todayDate).First(&todayStats)

	// 本周统计
	weekStart := todayDate.AddDate(0, 0, -int(today.Weekday()))
	var weekStats struct {
		RequestCount int
		SuccessCount int
		ErrorCount   int
		TotalCredits int
	}
	s.db.Model(&database.UserUsageStats{}).
		Where("user_id = ? AND date >= ?", userID, weekStart).
		Select("COALESCE(SUM(request_count), 0) as request_count, COALESCE(SUM(success_count), 0) as success_count, COALESCE(SUM(error_count), 0) as error_count, COALESCE(SUM(total_credits), 0) as total_credits").
		Scan(&weekStats)

	// 本月统计
	monthStart := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, today.Location())
	var monthStats struct {
		RequestCount int
		SuccessCount int
		ErrorCount   int
		TotalCredits int
	}
	s.db.Model(&database.UserUsageStats{}).
		Where("user_id = ? AND date >= ?", userID, monthStart).
		Select("COALESCE(SUM(request_count), 0) as request_count, COALESCE(SUM(success_count), 0) as success_count, COALESCE(SUM(error_count), 0) as error_count, COALESCE(SUM(total_credits), 0) as total_credits").
		Scan(&monthStats)

	// 总计
	var totalStats struct {
		RequestCount int
		SuccessCount int
		ErrorCount   int
		TotalCredits int
	}
	s.db.Model(&database.UserUsageStats{}).
		Where("user_id = ?", userID).
		Select("COALESCE(SUM(request_count), 0) as request_count, COALESCE(SUM(success_count), 0) as success_count, COALESCE(SUM(error_count), 0) as error_count, COALESCE(SUM(total_credits), 0) as total_credits").
		Scan(&totalStats)

	return map[string]interface{}{
		"today": map[string]int{
			"request_count": todayStats.RequestCount,
			"success_count": todayStats.SuccessCount,
			"error_count":   todayStats.ErrorCount,
			"total_credits": todayStats.TotalCredits,
		},
		"week": map[string]int{
			"request_count": weekStats.RequestCount,
			"success_count": weekStats.SuccessCount,
			"error_count":   weekStats.ErrorCount,
			"total_credits": weekStats.TotalCredits,
		},
		"month": map[string]int{
			"request_count": monthStats.RequestCount,
			"success_count": monthStats.SuccessCount,
			"error_count":   monthStats.ErrorCount,
			"total_credits": monthStats.TotalCredits,
		},
		"total": map[string]int{
			"request_count": totalStats.RequestCount,
			"success_count": totalStats.SuccessCount,
			"error_count":   totalStats.ErrorCount,
			"total_credits": totalStats.TotalCredits,
		},
	}, nil
}

// CleanupOldStats 清理旧的统计数据
func (s *UserUsageStatsService) CleanupOldStats(days int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -days)
	result := s.db.Where("date < ?", cutoff).Delete(&database.UserUsageStats{})
	return result.RowsAffected, result.Error
}

// GetDB 获取数据库连接
func (s *UserUsageStatsService) GetDB() *gorm.DB {
	return s.db
}

// IncrementUserStatsByToken 通过API令牌增加用户统计
// 用于在代理请求中记录统计，通过令牌查询用户ID
func (s *UserUsageStatsService) IncrementUserStatsByToken(apiToken string, success bool, credits int) error {
	// 通过API令牌查询用户
	var user database.User
	if err := s.db.Where("api_token = ?", apiToken).First(&user).Error; err != nil {
		// 用户不存在或查询失败，跳过统计
		return nil
	}

	// 记录用户统计
	return s.IncrementUserStats(user.ID, success, credits)
}
