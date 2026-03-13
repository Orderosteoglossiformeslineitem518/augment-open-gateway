package repository

import (
	"context"
	"fmt"
	"time"

	"augment-gateway/internal/database"

	"gorm.io/gorm"
)

// TokenRepository Token仓库接口
type TokenRepository interface {
	BaseRepository[database.Token]

	// Token特有方法
	GetByToken(ctx context.Context, token string) (*database.Token, error)
	GetByEmail(ctx context.Context, email string) (*database.Token, error)
	GetActiveTokens(ctx context.Context) ([]*database.Token, error)
	GetExpiredTokens(ctx context.Context) ([]*database.Token, error)
	GetTokensNeedingExpiration(ctx context.Context) ([]*database.Token, error)
	UpdateExpiredTokensStatus(ctx context.Context) (int64, error)
	CountByStatus(ctx context.Context, status string) (int64, error)
	CountTotal(ctx context.Context) (int64, error)
	CountActive(ctx context.Context) (int64, error)
	CountExpired(ctx context.Context) (int64, error)
	CountDisabled(ctx context.Context) (int64, error)
	UpdateUsage(ctx context.Context, tokenID string, increment int) error
	ListWithPagination(ctx context.Context, page, pageSize int, status, search string) ([]*database.Token, int64, error)
	ListWithPaginationAndBanReason(ctx context.Context, page, pageSize int, status, search, submitterUsername, isShared string) ([]*TokenWithBanReason, int64, error)
	SearchTokens(ctx context.Context, keyword string) ([]*database.Token, error)
	GetTokensByTenantAddress(ctx context.Context, tenantAddress string) ([]*database.Token, error)
	GetTokensExpiringSoon(ctx context.Context, days int) ([]*database.Token, error)
	UpdateStatus(ctx context.Context, tokenID string, status string) error
	BatchUpdateStatus(ctx context.Context, tokenIDs []string, status string) error
	GetUsageStats(ctx context.Context, tokenID string) (*TokenUsageStats, error)
}

// TokenUsageStats Token使用统计
type TokenUsageStats struct {
	TokenID         string     `json:"token_id"`
	TotalRequests   int64      `json:"total_requests"`
	SuccessRequests int64      `json:"success_requests"`
	ErrorRequests   int64      `json:"error_requests"`
	TotalBytes      int64      `json:"total_bytes"`
	LastRequest     *time.Time `json:"last_request"`
}

// tokenRepository Token仓库实现
type tokenRepository struct {
	BaseRepository[database.Token]
	db *gorm.DB
}

// NewTokenRepository 创建Token仓库
func NewTokenRepository(db *gorm.DB) TokenRepository {
	return &tokenRepository{
		BaseRepository: NewBaseRepository[database.Token](db),
		db:             db,
	}
}

// GetByToken 根据token字符串获取Token
func (r *tokenRepository) GetByToken(ctx context.Context, token string) (*database.Token, error) {
	query := NewQueryBuilder().WhereEq("token", token)
	return r.First(ctx, query)
}

// GetByEmail 根据邮箱获取Token
func (r *tokenRepository) GetByEmail(ctx context.Context, email string) (*database.Token, error) {
	query := NewQueryBuilder().WhereEq("email", email).IsNotNull("email")
	return r.First(ctx, query)
}

// GetActiveTokens 获取所有活跃的Token
func (r *tokenRepository) GetActiveTokens(ctx context.Context) ([]*database.Token, error) {
	query := NewQueryBuilder().
		WhereEq("status", "active").
		Where("(expires_at IS NULL OR expires_at > ?)", time.Now())

	return r.List(ctx, query)
}

// GetExpiredTokens 获取已过期的Token（状态为expired的）
func (r *tokenRepository) GetExpiredTokens(ctx context.Context) ([]*database.Token, error) {
	query := NewQueryBuilder().WhereEq("status", "expired")
	return r.List(ctx, query)
}

// GetTokensNeedingExpiration 获取需要标记为过期的Token（状态为active但已过期）
func (r *tokenRepository) GetTokensNeedingExpiration(ctx context.Context) ([]*database.Token, error) {
	query := NewQueryBuilder().
		WhereEq("status", "active").
		WhereLt("expires_at", time.Now()).
		IsNotNull("expires_at")

	return r.List(ctx, query)
}

// UpdateExpiredTokensStatus 批量更新过期TOKEN的状态
func (r *tokenRepository) UpdateExpiredTokensStatus(ctx context.Context) (int64, error) {
	result := r.db.WithContext(ctx).
		Model(&database.Token{}).
		Where("status = ? AND expires_at IS NOT NULL AND expires_at < ?", "active", time.Now()).
		Update("status", "expired")

	if result.Error != nil {
		return 0, fmt.Errorf("更新过期TOKEN状态失败: %w", result.Error)
	}

	return result.RowsAffected, nil
}

// CountByStatus 根据状态统计Token数量
func (r *tokenRepository) CountByStatus(ctx context.Context, status string) (int64, error) {
	query := NewQueryBuilder().WhereEq("status", status)
	return r.Count(ctx, query)
}

// CountExpired 统计过期Token数量
func (r *tokenRepository) CountExpired(ctx context.Context) (int64, error) {
	return r.CountByStatus(ctx, "expired")
}

// CountDisabled 统计禁用Token数量
func (r *tokenRepository) CountDisabled(ctx context.Context) (int64, error) {
	return r.CountByStatus(ctx, "disabled")
}

// CountTotal 统计Token总数
func (r *tokenRepository) CountTotal(ctx context.Context) (int64, error) {
	return r.Count(ctx, nil)
}

// CountActive 统计活跃Token数量
func (r *tokenRepository) CountActive(ctx context.Context) (int64, error) {
	return r.CountByStatus(ctx, "active")
}

// UpdateUsage 更新Token使用次数
func (r *tokenRepository) UpdateUsage(ctx context.Context, tokenID string, increment int) error {
	return r.db.WithContext(ctx).Model(&database.Token{}).
		Where("id = ?", tokenID).
		UpdateColumn("used_requests", gorm.Expr("used_requests + ?", increment)).Error
}

// TokenWithBanReason Token信息包含封禁原因
type TokenWithBanReason struct {
	database.Token
	BanReason *string `json:"ban_reason"`
}

// ListWithPagination 分页列表查询
func (r *tokenRepository) ListWithPagination(ctx context.Context, page, pageSize int, status, search string) ([]*database.Token, int64, error) {
	query := NewQueryBuilder()

	// 状态过滤
	if status != "" {
		query.WhereEq("status", status)
	}

	// 搜索过滤
	if search != "" {
		query.Where("token LIKE ? OR tenant_address LIKE ?", "%"+search+"%", "%"+search+"%")
	}

	// 获取总数
	total, err := r.Count(ctx, query)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count tokens: %w", err)
	}

	// 分页查询
	query.Page(page, pageSize).OrderByDesc("created_at")
	tokens, err := r.List(ctx, query)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list tokens: %w", err)
	}

	return tokens, total, nil
}

// ListWithPaginationAndBanReason 分页列表查询，包含封禁原因
// isShared参数：空字符串=全部，"true"=已共享，"false"=未共享
func (r *tokenRepository) ListWithPaginationAndBanReason(ctx context.Context, page, pageSize int, status, search, submitterUsername, isShared string) ([]*TokenWithBanReason, int64, error) {
	// 构建基础查询条件，使用Model确保逻辑删除生效
	db := r.db.WithContext(ctx).Model(&database.Token{})

	// 状态过滤
	if status != "" {
		db = db.Where("status = ?", status)
	}

	// 搜索过滤（支持邮箱或TOKEN模糊查询）
	if search != "" {
		db = db.Where("token LIKE ? OR email LIKE ?", "%"+search+"%", "%"+search+"%")
	}

	// 提交用户名过滤
	if submitterUsername != "" {
		db = db.Where("submitter_username LIKE ?", "%"+submitterUsername+"%")
	}

	// 共享状态过滤
	if isShared == "true" {
		db = db.Where("is_shared = ?", true)
	} else if isShared == "false" {
		db = db.Where("is_shared = ? OR is_shared IS NULL", false)
	}

	// 获取总数
	var total int64
	countQuery := db
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count tokens: %w", err)
	}

	// 分页查询，左连接ban_records表获取封禁原因
	var results []*TokenWithBanReason
	offset := (page - 1) * pageSize

	// 重新构建查询，包含JOIN和分页，使用Model确保软删除生效
	query := r.db.WithContext(ctx).
		Model(&database.Token{}).
		Select("tokens.*, br.ban_reason").
		Joins("LEFT JOIN ban_records br ON tokens.id = br.token_id")

	// 应用相同的过滤条件
	if status != "" {
		query = query.Where("tokens.status = ?", status)
	}
	if search != "" {
		query = query.Where("tokens.token LIKE ? OR tokens.email LIKE ?", "%"+search+"%", "%"+search+"%")
	}
	if submitterUsername != "" {
		query = query.Where("tokens.submitter_username LIKE ?", "%"+submitterUsername+"%")
	}
	// 共享状态过滤
	if isShared == "true" {
		query = query.Where("tokens.is_shared = ?", true)
	} else if isShared == "false" {
		query = query.Where("tokens.is_shared = ? OR tokens.is_shared IS NULL", false)
	}

	err := query.
		Order("tokens.created_at DESC").
		Limit(pageSize).
		Offset(offset).
		Scan(&results).Error

	if err != nil {
		return nil, 0, fmt.Errorf("failed to list tokens with ban reason: %w", err)
	}

	return results, total, nil
}

// SearchTokens 搜索Token
func (r *tokenRepository) SearchTokens(ctx context.Context, keyword string) ([]*database.Token, error) {
	query := NewQueryBuilder().
		Where("token LIKE ? OR name LIKE ? OR description LIKE ? OR tenant_address LIKE ?",
			"%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%").
		OrderByDesc("created_at")

	return r.List(ctx, query)
}

// GetTokensByTenantAddress 根据租户地址获取Token
func (r *tokenRepository) GetTokensByTenantAddress(ctx context.Context, tenantAddress string) ([]*database.Token, error) {
	query := NewQueryBuilder().
		WhereEq("tenant_address", tenantAddress).
		OrderByDesc("created_at")

	return r.List(ctx, query)
}

// GetTokensExpiringSoon 获取即将过期的Token
func (r *tokenRepository) GetTokensExpiringSoon(ctx context.Context, days int) ([]*database.Token, error) {
	expireTime := time.Now().AddDate(0, 0, days)
	query := NewQueryBuilder().
		WhereEq("status", "active").
		WhereLte("expires_at", expireTime).
		IsNotNull("expires_at").
		OrderByAsc("expires_at")

	return r.List(ctx, query)
}

// UpdateStatus 更新Token状态
func (r *tokenRepository) UpdateStatus(ctx context.Context, tokenID string, status string) error {
	return r.UpdateFields(ctx, tokenID, map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	})
}

// BatchUpdateStatus 批量更新Token状态
func (r *tokenRepository) BatchUpdateStatus(ctx context.Context, tokenIDs []string, status string) error {
	if len(tokenIDs) == 0 {
		return nil
	}

	return r.db.WithContext(ctx).Model(&database.Token{}).
		Where("id IN ?", tokenIDs).
		Updates(map[string]interface{}{
			"status":     status,
			"updated_at": time.Now(),
		}).Error
}

// GetUsageStats 获取Token使用统计
func (r *tokenRepository) GetUsageStats(ctx context.Context, tokenID string) (*TokenUsageStats, error) {
	stats := &TokenUsageStats{
		TokenID: tokenID,
	}

	// 从request_logs表获取统计数据
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
		return nil, fmt.Errorf("failed to get usage stats: %w", err)
	}

	stats.TotalRequests = result.TotalRequests
	stats.SuccessRequests = result.SuccessRequests
	stats.ErrorRequests = result.TotalRequests - result.SuccessRequests
	stats.TotalBytes = result.TotalBytes
	stats.LastRequest = result.LastRequest

	return stats, nil
}

// 便捷查询方法

// GetValidToken 获取有效的Token（活跃且未过期）
func (r *tokenRepository) GetValidToken(ctx context.Context, token string) (*database.Token, error) {
	query := NewQueryBuilder().
		WhereEq("token", token).
		WhereEq("status", "active").
		Where("(expires_at IS NULL OR expires_at > ?)", time.Now())

	return r.First(ctx, query)
}

// GetTokensCreatedToday 获取今天创建的Token
func (r *tokenRepository) GetTokensCreatedToday(ctx context.Context) ([]*database.Token, error) {
	query := NewQueryBuilder().
		WhereToday("created_at").
		OrderByDesc("created_at")

	return r.List(ctx, query)
}

// GetTokensCreatedThisWeek 获取本周创建的Token
func (r *tokenRepository) GetTokensCreatedThisWeek(ctx context.Context) ([]*database.Token, error) {
	query := NewQueryBuilder().
		WhereThisWeek("created_at").
		OrderByDesc("created_at")

	return r.List(ctx, query)
}

// GetTokensCreatedThisMonth 获取本月创建的Token
func (r *tokenRepository) GetTokensCreatedThisMonth(ctx context.Context) ([]*database.Token, error) {
	query := NewQueryBuilder().
		WhereThisMonth("created_at").
		OrderByDesc("created_at")

	return r.List(ctx, query)
}

// GetTopUsedTokens 获取使用次数最多的Token
func (r *tokenRepository) GetTopUsedTokens(ctx context.Context, limit int) ([]*database.Token, error) {
	query := NewQueryBuilder().
		OrderByDesc("used_requests").
		Limit(limit)

	return r.List(ctx, query)
}
