package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"augment-gateway/internal/database"
	"augment-gateway/internal/logger"
)

// BanRecordService 封号记录服务
type BanRecordService struct {
	db *gorm.DB
}

// NewBanRecordService 创建封号记录服务
func NewBanRecordService(db *gorm.DB) *BanRecordService {
	return &BanRecordService{
		db: db,
	}
}

// BanRecordInfo 封号记录信息
type BanRecordInfo struct {
	TokenID        *string   `json:"token_id"`
	UserToken      *string   `json:"user_token"`
	RequestPath    string    `json:"request_path"`
	RequestMethod  string    `json:"request_method"`
	RequestHeaders string    `json:"request_headers"`
	ClientIP       string    `json:"client_ip"`
	UserAgent      string    `json:"user_agent"`
	BanReason      string    `json:"ban_reason"`
	ResponseBody   string    `json:"response_body"`
	BannedAt       time.Time `json:"banned_at"`
}

// CreateBanRecord 创建封号记录
func (s *BanRecordService) CreateBanRecord(ctx context.Context, info *BanRecordInfo) error {
	// 如果有token_id，先检查是否已经存在该token的封号记录
	if info.TokenID != nil && *info.TokenID != "" {
		var existingRecord database.BanRecord
		err := s.db.WithContext(ctx).
			Where("token_id = ?", *info.TokenID).
			First(&existingRecord).Error

		if err == nil {
			// 已存在该token的封号记录，不再重复记录
			logger.Infof("[封号记录] ⚠️ TOKEN %s 已存在封号记录，跳过重复记录\n",
				getTokenDisplay(info.TokenID))
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			// 查询出错，记录警告但继续执行
			logger.Infof("[封号记录] 警告: 检查重复记录时出错: %v\n", err)
		}
	}

	// 截取响应内容，避免过长，并处理字符编码问题
	responseBody := info.ResponseBody
	if len(responseBody) > 500 {
		// 安全截取，避免截断UTF-8字符
		responseBody = s.safeSubstring(responseBody, 500) + "..."
	}

	// 清理可能导致MySQL编码错误的字符
	responseBody = s.cleanResponseBody(responseBody)

	banRecord := &database.BanRecord{
		TokenID:        info.TokenID,
		UserToken:      info.UserToken,
		RequestPath:    info.RequestPath,
		RequestMethod:  info.RequestMethod,
		RequestHeaders: info.RequestHeaders,
		ClientIP:       info.ClientIP,
		UserAgent:      info.UserAgent,
		BanReason:      info.BanReason,
		ResponseBody:   responseBody,
		BannedAt:       info.BannedAt,
		CreatedAt:      time.Now(),
	}

	if err := s.db.WithContext(ctx).Create(banRecord).Error; err != nil {
		return fmt.Errorf("创建封号记录失败: %w", err)
	}

	logger.Infof("[封号记录] ✅ 成功记录封号信息 - TOKEN: %s, 用户令牌: %s, 原因: %s\n",
		getTokenDisplay(info.TokenID),
		getTokenDisplay(info.UserToken),
		info.BanReason)

	return nil
}

// CreateBanRecordFromRequest 从请求上下文创建封号记录
func (s *BanRecordService) CreateBanRecordFromRequest(
	ctx context.Context,
	c *gin.Context,
	tokenID *string,
	userToken *string,
	banReason string,
	responseBody string,
) error {
	// 获取请求头信息
	headers := make(map[string]string)
	for key, values := range c.Request.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}
	headersJSON, _ := json.Marshal(headers)

	// 获取客户端IP
	clientIP := c.ClientIP()

	// 获取用户代理
	userAgent := c.GetHeader("User-Agent")

	// 构建请求路径
	requestPath := c.Request.URL.Path
	if c.Request.URL.RawQuery != "" {
		requestPath += "?" + c.Request.URL.RawQuery
	}

	info := &BanRecordInfo{
		TokenID:        tokenID,
		UserToken:      userToken,
		RequestPath:    requestPath,
		RequestMethod:  c.Request.Method,
		RequestHeaders: string(headersJSON),
		ClientIP:       clientIP,
		UserAgent:      userAgent,
		BanReason:      banReason,
		ResponseBody:   responseBody,
		BannedAt:       time.Now(),
	}

	return s.CreateBanRecord(ctx, info)
}

// GetBanRecords 获取封号记录列表
func (s *BanRecordService) GetBanRecords(ctx context.Context, limit, offset int) ([]*database.BanRecord, int64, error) {
	var records []*database.BanRecord
	var total int64

	// 获取总数
	if err := s.db.WithContext(ctx).Model(&database.BanRecord{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("获取封号记录总数失败: %w", err)
	}

	// 获取记录列表
	if err := s.db.WithContext(ctx).
		Order("banned_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&records).Error; err != nil {
		return nil, 0, fmt.Errorf("获取封号记录列表失败: %w", err)
	}

	return records, total, nil
}

// GetBanRecordsByToken 根据TOKEN获取封号记录
func (s *BanRecordService) GetBanRecordsByToken(ctx context.Context, tokenID string, limit int) ([]*database.BanRecord, error) {
	var records []*database.BanRecord

	if err := s.db.WithContext(ctx).
		Where("token_id = ?", tokenID).
		Order("banned_at DESC").
		Limit(limit).
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("获取TOKEN封号记录失败: %w", err)
	}

	return records, nil
}

// GetBanRecordsByUserToken 根据用户令牌获取封号记录
func (s *BanRecordService) GetBanRecordsByUserToken(ctx context.Context, userToken string, limit int) ([]*database.BanRecord, error) {
	var records []*database.BanRecord

	if err := s.db.WithContext(ctx).
		Where("user_token = ?", userToken).
		Order("banned_at DESC").
		Limit(limit).
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("获取用户令牌封号记录失败: %w", err)
	}

	return records, nil
}

// DeleteOldBanRecords 删除旧的封号记录（保留最近30天）
func (s *BanRecordService) DeleteOldBanRecords(ctx context.Context) (int64, error) {
	cutoffTime := time.Now().AddDate(0, 0, -30) // 30天前

	result := s.db.WithContext(ctx).
		Where("banned_at < ?", cutoffTime).
		Delete(&database.BanRecord{})

	if result.Error != nil {
		return 0, fmt.Errorf("删除旧封号记录失败: %w", result.Error)
	}

	if result.RowsAffected > 0 {
		logger.Infof("[封号记录] 🗑️ 清理了 %d 条30天前的封号记录\n", result.RowsAffected)
	}

	return result.RowsAffected, nil
}

// getTokenDisplay 获取TOKEN显示文本（前8位）
func getTokenDisplay(token *string) string {
	if token == nil || *token == "" {
		return "无"
	}
	if len(*token) > 8 {
		return (*token)[:8] + "..."
	}
	return *token
}

// safeSubstring 安全截取字符串，避免截断UTF-8字符
func (s *BanRecordService) safeSubstring(str string, maxLen int) string {
	if len(str) <= maxLen {
		return str
	}

	// 转换为rune切片以正确处理UTF-8字符
	runes := []rune(str)
	if len(runes) <= maxLen {
		return str
	}

	return string(runes[:maxLen])
}

// cleanResponseBody 清理响应体中可能导致MySQL编码错误的字符
func (s *BanRecordService) cleanResponseBody(body string) string {
	// 移除或替换可能导致MySQL编码错误的字符
	// 保留基本的ASCII字符和常见的UTF-8字符
	var result strings.Builder

	for _, r := range body {
		// 保留可打印的ASCII字符和基本的UTF-8字符
		if r < 32 && r != '\n' && r != '\r' && r != '\t' {
			// 跳过控制字符（除了换行、回车、制表符）
			continue
		}

		// 跳过可能有问题的字符范围
		if r == 0xFFFD { // Unicode替换字符
			continue
		}

		result.WriteRune(r)
	}

	return result.String()
}

// GetBanRecordStats 获取封号记录统计
func (s *BanRecordService) GetBanRecordStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// 总封号记录数
	var totalCount int64
	if err := s.db.WithContext(ctx).Model(&database.BanRecord{}).Count(&totalCount).Error; err != nil {
		return nil, fmt.Errorf("获取总封号记录数失败: %w", err)
	}
	stats["total_count"] = totalCount

	// 今日封号记录数
	today := time.Now().Truncate(24 * time.Hour)
	var todayCount int64
	if err := s.db.WithContext(ctx).Model(&database.BanRecord{}).
		Where("banned_at >= ?", today).
		Count(&todayCount).Error; err != nil {
		return nil, fmt.Errorf("获取今日封号记录数失败: %w", err)
	}
	stats["today_count"] = todayCount

	// 最近7天封号记录数
	weekAgo := time.Now().AddDate(0, 0, -7)
	var weekCount int64
	if err := s.db.WithContext(ctx).Model(&database.BanRecord{}).
		Where("banned_at >= ?", weekAgo).
		Count(&weekCount).Error; err != nil {
		return nil, fmt.Errorf("获取最近7天封号记录数失败: %w", err)
	}
	stats["week_count"] = weekCount

	// 最近30天封号记录数
	monthAgo := time.Now().AddDate(0, 0, -30)
	var monthCount int64
	if err := s.db.WithContext(ctx).Model(&database.BanRecord{}).
		Where("banned_at >= ?", monthAgo).
		Count(&monthCount).Error; err != nil {
		return nil, fmt.Errorf("获取最近30天封号记录数失败: %w", err)
	}
	stats["month_count"] = monthCount

	return stats, nil
}

// GetTopBanReasons 获取最常见的封禁原因
func (s *BanRecordService) GetTopBanReasons(ctx context.Context, limit int) ([]map[string]interface{}, error) {
	var results []struct {
		BanReason string `json:"ban_reason"`
		Count     int64  `json:"count"`
	}

	if err := s.db.WithContext(ctx).
		Model(&database.BanRecord{}).
		Select("ban_reason, COUNT(*) as count").
		Group("ban_reason").
		Order("count DESC").
		Limit(limit).
		Find(&results).Error; err != nil {
		return nil, fmt.Errorf("获取封禁原因统计失败: %w", err)
	}

	// 转换为通用格式
	var reasons []map[string]interface{}
	for _, result := range results {
		reasons = append(reasons, map[string]interface{}{
			"reason": result.BanReason,
			"count":  result.Count,
		})
	}

	return reasons, nil
}
