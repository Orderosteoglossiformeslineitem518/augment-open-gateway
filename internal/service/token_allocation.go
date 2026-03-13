package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"augment-gateway/internal/database"
	"augment-gateway/internal/logger"

	"gorm.io/gorm"
)

// TokenAllocationService TOKEN分配服务
type TokenAllocationService struct {
	db           *gorm.DB
	cacheService *CacheService
}

// NewTokenAllocationService 创建TOKEN分配服务
func NewTokenAllocationService(db *gorm.DB) *TokenAllocationService {
	return &TokenAllocationService{
		db: db,
	}
}

// SetCacheService 设置缓存服务（避免循环依赖）
func (s *TokenAllocationService) SetCacheService(cacheService *CacheService) {
	s.cacheService = cacheService
}

// UserTokenAllocationListRequest 用户TOKEN分配列表请求
type UserTokenAllocationListRequest struct {
	Page      int    `form:"page" json:"page"`
	PageSize  int    `form:"page_size" json:"page_size"`
	Status    string `form:"status" json:"status"`         // TOKEN状态筛选：active, disabled, expired
	Search    string `form:"search" json:"search"`         // 搜索：邮箱或TOKEN ID模糊匹配
	TokenType string `form:"token_type" json:"token_type"` // TOKEN类型筛选：own(自有), shared(共享), 空为全部
}

// UserTokenAllocationListResponse 用户TOKEN分配列表响应
type UserTokenAllocationListResponse struct {
	List  []TokenAllocationDetail `json:"list"`
	Total int64                   `json:"total"`
}

// TokenAllocationDetail TOKEN分配详情（包含TOKEN信息）
type TokenAllocationDetail struct {
	ID            uint       `json:"id"`
	TokenID       string     `json:"token_id"`
	TokenValue    string     `json:"token_value"` // TOKEN值（共享账号显示"共享账号"，非共享显示token字段）
	AllocatedAt   time.Time  `json:"allocated_at"`
	ExpiredAt     *time.Time `json:"expired_at,omitempty"`
	Status        string     `json:"status"`
	StatusDisplay string     `json:"status_display"`
	Remark        string     `json:"remark"`

	// TOKEN信息
	TokenEmail      string     `json:"token_email,omitempty"`
	TokenStatus     string     `json:"token_status"`
	TokenExpiresAt  *time.Time `json:"token_expires_at,omitempty"`
	MaxRequests     int        `json:"max_requests"`
	UsedRequests    int        `json:"used_requests"`
	TenantAddress   string     `json:"tenant_address,omitempty"`   // 租户地址
	PortalURL       string     `json:"portal_url,omitempty"`       // 订阅地址
	TokenCreatedAt  *time.Time `json:"token_created_at,omitempty"` // TOKEN创建时间
	BanReason       string     `json:"ban_reason,omitempty"`       // 封禁原因
	IsCurrentUsing  bool       `json:"is_current_using"`           // 是否为当前使用的账号
	EnhancedEnabled bool       `json:"enhanced_enabled"`           // 是否开启增强功能
	IsSharedToken   bool       `json:"is_shared_token"`            // 是否为共享TOKEN

	// 增强渠道信息
	EnhancedChannelID   *uint  `json:"enhanced_channel_id,omitempty"`   // 绑定的外部渠道ID
	EnhancedChannelName string `json:"enhanced_channel_name,omitempty"` // 绑定的外部渠道名称
}

// GetUserAllocations 获取用户的TOKEN列表
// 包含：1. 用户提交的TOKEN（submitter_user_id = 当前用户）
//
//  2. 系统分配给用户的共享TOKEN（通过shared_token_allocations表关联）
//
// 通过TokenType参数筛选：own(自有), shared(共享), 空为全部
// 使用UNION ALL在数据库层面完成分页，避免内存分页性能问题
func (s *TokenAllocationService) GetUserAllocations(userID uint, req *UserTokenAllocationListRequest, currentUsingTokenID string) (*UserTokenAllocationListResponse, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}

	offset := (req.Page - 1) * req.PageSize

	// 根据TokenType决定查询哪些数据
	queryOwn := req.TokenType == "" || req.TokenType == "own"
	queryShared := req.TokenType == "" || req.TokenType == "shared"

	// 单一类型查询：直接使用原有逻辑
	if req.TokenType == "own" {
		return s.getUserOwnTokens(userID, req, offset, currentUsingTokenID)
	}
	if req.TokenType == "shared" {
		return s.getUserSharedTokens(userID, req, offset, currentUsingTokenID)
	}

	// 全部类型查询：使用UNION ALL
	return s.getUserAllTokensWithUnion(userID, req, offset, currentUsingTokenID, queryOwn, queryShared)
}

func (s *TokenAllocationService) getUserSystemSharedAllocationsQuery(userID uint) *gorm.DB {
	return s.db.Model(&database.SharedTokenAllocation{}).
		Joins("JOIN tokens ON tokens.id = shared_token_allocations.token_id").
		Where("shared_token_allocations.user_id = ?", userID).
		Where("tokens.deleted_at IS NULL").
		Where("tokens.submitter_user_id IS NULL").
		Where("tokens.is_shared = ?", true)
}

// getUserOwnTokens 获取用户自有TOKEN列表
func (s *TokenAllocationService) getUserOwnTokens(userID uint, req *UserTokenAllocationListRequest, offset int, currentUsingTokenID string) (*UserTokenAllocationListResponse, error) {
	var userTokens []database.Token
	var total int64

	userTokenQuery := s.db.Model(&database.Token{}).Where("submitter_user_id = ?", userID)
	if req.Search != "" {
		searchPattern := "%" + req.Search + "%"
		userTokenQuery = userTokenQuery.Where("id LIKE ? OR email LIKE ?", searchPattern, searchPattern)
	}
	if req.Status != "" {
		userTokenQuery = userTokenQuery.Where("status = ?", req.Status)
	}

	userTokenQuery.Count(&total)
	userTokenQuery.Order("created_at DESC").Offset(offset).Limit(req.PageSize).Find(&userTokens)

	// 批量获取增强信息
	tokenIDs := make([]string, 0, len(userTokens))
	for _, token := range userTokens {
		tokenIDs = append(tokenIDs, token.ID)
	}
	enhanceInfoMap, _ := s.GetTokenEnhanceInfoBatch(userID, tokenIDs)

	list := make([]TokenAllocationDetail, 0, len(userTokens))
	for _, token := range userTokens {
		detail := s.buildTokenAllocationDetail(token, false, token.CreatedAt, currentUsingTokenID, enhanceInfoMap)
		list = append(list, detail)
	}

	return &UserTokenAllocationListResponse{List: list, Total: total}, nil
}

// getUserSharedTokens 获取用户共享TOKEN列表
func (s *TokenAllocationService) getUserSharedTokens(userID uint, req *UserTokenAllocationListRequest, offset int, currentUsingTokenID string) (*UserTokenAllocationListResponse, error) {
	var sharedAllocations []database.SharedTokenAllocation
	var total int64

	// 统计总数
	countQuery := s.getUserSystemSharedAllocationsQuery(userID)
	if req.Search != "" {
		searchPattern := "%" + req.Search + "%"
		countQuery = countQuery.Where("tokens.id LIKE ? OR tokens.email LIKE ?", searchPattern, searchPattern)
	}
	if req.Status != "" {
		countQuery = countQuery.Where("tokens.status = ?", req.Status)
	}
	countQuery.Count(&total)

	// 查询数据
	preloadQuery := s.getUserSystemSharedAllocationsQuery(userID).Preload("Token")
	if req.Search != "" {
		searchPattern := "%" + req.Search + "%"
		preloadQuery = preloadQuery.Where("tokens.id LIKE ? OR tokens.email LIKE ?", searchPattern, searchPattern)
	}
	if req.Status != "" {
		preloadQuery = preloadQuery.Where("tokens.status = ?", req.Status)
	}
	preloadQuery = preloadQuery.Order("shared_token_allocations.allocated_at DESC").Offset(offset).Limit(req.PageSize)
	preloadQuery.Find(&sharedAllocations)

	// 批量获取增强信息
	tokenIDs := make([]string, 0, len(sharedAllocations))
	for _, alloc := range sharedAllocations {
		if alloc.Token != nil {
			tokenIDs = append(tokenIDs, alloc.Token.ID)
		}
	}
	enhanceInfoMap, _ := s.GetTokenEnhanceInfoBatch(userID, tokenIDs)

	list := make([]TokenAllocationDetail, 0, len(sharedAllocations))
	for _, alloc := range sharedAllocations {
		if alloc.Token != nil {
			detail := s.buildTokenAllocationDetail(*alloc.Token, true, alloc.AllocatedAt, currentUsingTokenID, enhanceInfoMap)
			list = append(list, detail)
		}
	}

	return &UserTokenAllocationListResponse{List: list, Total: total}, nil
}

// CombinedTokenRow UNION ALL查询结果行
type CombinedTokenRow struct {
	TokenID   string    `gorm:"column:token_id"`
	TokenType string    `gorm:"column:token_type"` // 'own' or 'shared'
	SortTime  time.Time `gorm:"column:sort_time"`
}

// getUserAllTokensWithUnion 使用UNION ALL获取用户所有TOKEN列表
func (s *TokenAllocationService) getUserAllTokensWithUnion(userID uint, req *UserTokenAllocationListRequest, offset int, currentUsingTokenID string, queryOwn, queryShared bool) (*UserTokenAllocationListResponse, error) {
	var total int64
	var combinedRows []CombinedTokenRow

	// 构建搜索和状态条件
	searchCondition := ""
	statusCondition := ""
	args := []interface{}{}

	if req.Search != "" {
		searchPattern := "%" + req.Search + "%"
		searchCondition = " AND (t.id LIKE ? OR t.email LIKE ?)"
		args = append(args, searchPattern, searchPattern)
	}
	if req.Status != "" {
		statusCondition = " AND t.status = ?"
		args = append(args, req.Status)
	}

	// 构建UNION ALL查询
	var unionSQL string
	var countSQL string
	unionArgs := []interface{}{}
	countArgs := []interface{}{}

	if queryOwn && queryShared {
		// 查询全部类型
		unionSQL = `
			SELECT token_id, token_type, sort_time FROM (
				SELECT t.id as token_id, 'own' as token_type, t.created_at as sort_time 
				FROM tokens t 
				WHERE t.submitter_user_id = ? AND t.deleted_at IS NULL` + searchCondition + statusCondition + `
				UNION ALL
				SELECT t.id as token_id, 'shared' as token_type, sta.allocated_at as sort_time 
				FROM shared_token_allocations sta
				JOIN tokens t ON t.id = sta.token_id
				WHERE sta.user_id = ? AND sta.deleted_at IS NULL AND t.deleted_at IS NULL AND t.submitter_user_id IS NULL AND t.is_shared = 1` + searchCondition + statusCondition + `
			) AS combined
			ORDER BY sort_time DESC
			LIMIT ? OFFSET ?
		`
		countSQL = `
			SELECT COUNT(*) FROM (
				SELECT t.id FROM tokens t 
				WHERE t.submitter_user_id = ? AND t.deleted_at IS NULL` + searchCondition + statusCondition + `
				UNION ALL
				SELECT t.id FROM shared_token_allocations sta
				JOIN tokens t ON t.id = sta.token_id
				WHERE sta.user_id = ? AND sta.deleted_at IS NULL AND t.deleted_at IS NULL AND t.submitter_user_id IS NULL AND t.is_shared = 1` + searchCondition + statusCondition + `
			) AS combined
		`
		// 构建参数：第一个子查询的参数 + 第二个子查询的参数
		unionArgs = append(unionArgs, userID)
		unionArgs = append(unionArgs, args...)
		unionArgs = append(unionArgs, userID)
		unionArgs = append(unionArgs, args...)
		unionArgs = append(unionArgs, req.PageSize, offset)

		countArgs = append(countArgs, userID)
		countArgs = append(countArgs, args...)
		countArgs = append(countArgs, userID)
		countArgs = append(countArgs, args...)
	}

	// 执行统计查询
	if err := s.db.Raw(countSQL, countArgs...).Scan(&total).Error; err != nil {
		return nil, fmt.Errorf("统计TOKEN总数失败: %w", err)
	}

	// 执行分页查询
	if err := s.db.Raw(unionSQL, unionArgs...).Scan(&combinedRows).Error; err != nil {
		return nil, fmt.Errorf("查询TOKEN列表失败: %w", err)
	}

	if len(combinedRows) == 0 {
		return &UserTokenAllocationListResponse{List: []TokenAllocationDetail{}, Total: total}, nil
	}

	// 收集TOKEN IDs，分类查询
	ownTokenIDs := make([]string, 0)
	sharedTokenIDs := make([]string, 0)
	tokenTypeMap := make(map[string]string)        // token_id -> token_type
	tokenSortTimeMap := make(map[string]time.Time) // token_id -> sort_time

	for _, row := range combinedRows {
		tokenTypeMap[row.TokenID] = row.TokenType
		tokenSortTimeMap[row.TokenID] = row.SortTime
		if row.TokenType == "own" {
			ownTokenIDs = append(ownTokenIDs, row.TokenID)
		} else {
			sharedTokenIDs = append(sharedTokenIDs, row.TokenID)
		}
	}

	// 批量查询TOKEN详情
	tokenMap := make(map[string]database.Token)
	if len(ownTokenIDs) > 0 {
		var ownTokens []database.Token
		s.db.Where("id IN ?", ownTokenIDs).Find(&ownTokens)
		for _, t := range ownTokens {
			tokenMap[t.ID] = t
		}
	}
	if len(sharedTokenIDs) > 0 {
		var sharedTokens []database.Token
		s.db.Where("id IN ?", sharedTokenIDs).Find(&sharedTokens)
		for _, t := range sharedTokens {
			tokenMap[t.ID] = t
		}
	}

	// 批量获取增强信息
	allTokenIDs := make([]string, 0, len(combinedRows))
	for _, row := range combinedRows {
		allTokenIDs = append(allTokenIDs, row.TokenID)
	}
	enhanceInfoMap, _ := s.GetTokenEnhanceInfoBatch(userID, allTokenIDs)

	// 按原始顺序构建结果列表
	list := make([]TokenAllocationDetail, 0, len(combinedRows))
	for _, row := range combinedRows {
		token, exists := tokenMap[row.TokenID]
		if !exists {
			continue
		}
		isShared := row.TokenType == "shared"
		detail := s.buildTokenAllocationDetail(token, isShared, row.SortTime, currentUsingTokenID, enhanceInfoMap)
		list = append(list, detail)
	}

	return &UserTokenAllocationListResponse{List: list, Total: total}, nil
}

// buildTokenAllocationDetail 构建TOKEN分配详情
func (s *TokenAllocationService) buildTokenAllocationDetail(token database.Token, isShared bool, allocatedAt time.Time, currentUsingTokenID string, enhanceInfoMap map[string]*TokenEnhanceInfo) TokenAllocationDetail {
	detail := TokenAllocationDetail{
		TokenID:         token.ID,
		TokenStatus:     token.Status,
		TokenExpiresAt:  token.ExpiresAt,
		MaxRequests:     token.MaxRequests,
		UsedRequests:    token.UsedRequests,
		TokenCreatedAt:  &token.CreatedAt,
		IsCurrentUsing:  token.ID == currentUsingTokenID,
		EnhancedEnabled: token.EnhancedEnabled,
		IsSharedToken:   isShared,
		AllocatedAt:     allocatedAt,
	}

	// 对共享TOKEN进行脱敏处理
	if isShared {
		detail.TokenValue = "共享账号"
		detail.TokenEmail = "augmentgateway@augmentcode.com"
		detail.TenantAddress = "共享账号"
		detail.PortalURL = "" // 订阅地址返回空
	} else {
		// 非共享TOKEN显示真实信息
		detail.TokenValue = token.Token // 显示token字段值
		if token.Email != nil {
			detail.TokenEmail = *token.Email
		}
		detail.TenantAddress = token.TenantAddress
		if token.PortalURL != nil {
			detail.PortalURL = *token.PortalURL
		}
	}

	// 填充增强渠道信息
	if enhanceInfo, ok := enhanceInfoMap[token.ID]; ok && enhanceInfo != nil {
		detail.EnhancedChannelID = &enhanceInfo.ChannelID
		detail.EnhancedChannelName = enhanceInfo.ProviderName
	}

	// 设置状态显示
	if token.Status == "active" {
		detail.Status = "active"
		detail.StatusDisplay = "正常"
	} else if token.Status == "disabled" {
		detail.Status = "disabled"
		detail.StatusDisplay = "已禁用"
	} else if token.Status == "expired" || (token.ExpiresAt != nil && token.ExpiresAt.Before(time.Now())) {
		detail.Status = "expired"
		detail.StatusDisplay = "已过期"
	}

	// 如果TOKEN被禁用，获取封禁原因
	if token.Status == "disabled" {
		var banRecord database.BanRecord
		if err := s.db.Where("token_id = ?", token.ID).Order("banned_at DESC").First(&banRecord).Error; err == nil {
			detail.BanReason = banRecord.BanReason
		}
	}

	return detail
}

// GetDB 获取数据库连接
func (s *TokenAllocationService) GetDB() *gorm.DB {
	return s.db
}

// UserTokenAccountStatsResponse 用户TOKEN账号统计响应
type UserTokenAccountStatsResponse struct {
	TotalCount     int64 `json:"total_count"`     // 总账号数
	AvailableCount int64 `json:"available_count"` // 可用账户数
	DisabledCount  int64 `json:"disabled_count"`  // 封禁账户数
	ExpiredCount   int64 `json:"expired_count"`   // 过期账号数
}

// GetUserTokenAccountStats 获取用户的TOKEN账号统计
// 包含：1. 用户提交的TOKEN 2. 系统分配给用户的共享TOKEN
func (s *TokenAllocationService) GetUserTokenAccountStats(userID uint) (*UserTokenAccountStatsResponse, error) {
	// 1. 统计用户提交的TOKEN
	var userTotalCount, userAvailableCount, userDisabledCount, userExpiredCount int64

	s.db.Model(&database.Token{}).Where("submitter_user_id = ?", userID).Count(&userTotalCount)
	s.db.Model(&database.Token{}).Where("submitter_user_id = ? AND status = ?", userID, "active").Count(&userAvailableCount)
	s.db.Model(&database.Token{}).Where("submitter_user_id = ? AND status = ?", userID, "disabled").Count(&userDisabledCount)
	s.db.Model(&database.Token{}).Where("submitter_user_id = ? AND status = ?", userID, "expired").Count(&userExpiredCount)

	// 2. 统计系统分配给用户的共享TOKEN
	var sharedAllocations []database.SharedTokenAllocation
	if err := s.getUserSystemSharedAllocationsQuery(userID).Preload("Token").Find(&sharedAllocations).Error; err != nil {
		return nil, fmt.Errorf("查询共享TOKEN失败: %w", err)
	}

	var sharedTotalCount, sharedAvailableCount, sharedDisabledCount, sharedExpiredCount int64
	for _, alloc := range sharedAllocations {
		if alloc.Token != nil {
			sharedTotalCount++
			switch alloc.Token.Status {
			case "active":
				sharedAvailableCount++
			case "disabled":
				sharedDisabledCount++
			case "expired":
				sharedExpiredCount++
			}
		}
	}

	return &UserTokenAccountStatsResponse{
		TotalCount:     userTotalCount + sharedTotalCount,
		AvailableCount: userAvailableCount + sharedAvailableCount,
		DisabledCount:  userDisabledCount + sharedDisabledCount,
		ExpiredCount:   userExpiredCount + sharedExpiredCount,
	}, nil
}

// UserDisableTokenRequest 用户禁用TOKEN请求
type UserDisableTokenRequest struct {
	TokenID string `json:"token_id" binding:"required"`
}

// UserSwitchTokenRequest 用户切换TOKEN请求
type UserSwitchTokenRequest struct {
	TokenID string `json:"token_id" binding:"required"`
}

// DisableUserToken 用户禁用自己提交的TOKEN（直接从tokens表查询）
func (s *TokenAllocationService) DisableUserToken(userID uint, tokenID string) error {
	// 验证该TOKEN是否属于该用户（通过submitter_user_id判断）
	var token database.Token
	if err := s.db.Where("id = ? AND submitter_user_id = ?", tokenID, userID).First(&token).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("该账号不属于您或不存在")
		}
		return err
	}

	// 检查TOKEN是否已被禁用
	if token.Status == "disabled" {
		return errors.New("该账号已被禁用")
	}

	// 禁用TOKEN
	if err := s.db.Model(&token).Update("status", "disabled").Error; err != nil {
		return errors.New("禁用账号失败")
	}

	// 记录禁用原因
	banRecord := database.BanRecord{
		TokenID:   &tokenID,
		BanReason: "用户主动禁用",
		BannedAt:  time.Now(),
	}
	s.db.Create(&banRecord)

	return nil
}

// DeleteUserToken 用户删除自己提交的TOKEN（仅自有账号且已禁用状态可删除）
func (s *TokenAllocationService) DeleteUserToken(userID uint, tokenID string) error {
	// 验证该TOKEN是否属于该用户（通过submitter_user_id判断）
	var token database.Token
	if err := s.db.Where("id = ? AND submitter_user_id = ?", tokenID, userID).First(&token).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("仅可删除自有账号")
		}
		return err
	}

	// 共享账号不可删除（双重检查，确保安全）
	if token.SubmitterUserID == nil || *token.SubmitterUserID != userID {
		return errors.New("共享账号不可删除")
	}

	// 仅允许删除已禁用的账号
	if token.Status != "disabled" {
		return errors.New("仅可删除已禁用的账号")
	}

	// 软删除TOKEN
	if err := s.db.Delete(&token).Error; err != nil {
		return errors.New("删除账号失败")
	}

	// 清理相关的增强绑定记录
	s.db.Where("token_id = ?", tokenID).Delete(&database.TokenChannelBinding{})

	return nil
}

// GetUserAvailableTokensForSwitch 获取用户可切换的TOKEN列表
// 包含：1. 用户提交的活跃TOKEN 2. 系统分配给用户的活跃共享TOKEN
func (s *TokenAllocationService) GetUserAvailableTokensForSwitch(userID uint, excludeTokenID string) ([]TokenAllocationDetail, error) {
	// 1. 查询用户提交的活跃TOKEN
	query := s.db.Model(&database.Token{}).Where("submitter_user_id = ? AND status = ?", userID, "active")
	if excludeTokenID != "" {
		query = query.Where("id != ?", excludeTokenID)
	}

	var userTokens []database.Token
	if err := query.Order("created_at DESC").Find(&userTokens).Error; err != nil {
		return nil, err
	}

	// 2. 查询系统分配给用户的活跃共享TOKEN
	var sharedAllocations []database.SharedTokenAllocation
	sharedQuery := s.getUserSystemSharedAllocationsQuery(userID).Preload("Token")
	if err := sharedQuery.Find(&sharedAllocations).Error; err != nil {
		return nil, err
	}

	// 合并TOKEN列表
	tokenMap := make(map[string]bool)
	list := make([]TokenAllocationDetail, 0)

	// 添加用户提交的TOKEN
	for _, token := range userTokens {
		tokenMap[token.ID] = false // false表示非共享TOKEN
		detail := TokenAllocationDetail{
			TokenID:        token.ID,
			AllocatedAt:    token.CreatedAt,
			Status:         "active",
			StatusDisplay:  "正常",
			TokenStatus:    token.Status,
			MaxRequests:    token.MaxRequests,
			UsedRequests:   token.UsedRequests,
			TenantAddress:  token.TenantAddress,
			TokenCreatedAt: &token.CreatedAt,
			IsCurrentUsing: false,
			IsSharedToken:  false,
		}
		if token.Email != nil {
			detail.TokenEmail = *token.Email
		}
		list = append(list, detail)
	}

	// 添加共享TOKEN（跳过已存在的和排除的）
	for _, alloc := range sharedAllocations {
		if alloc.Token != nil && alloc.Token.Status == "active" {
			if _, exists := tokenMap[alloc.Token.ID]; !exists {
				if excludeTokenID != "" && alloc.Token.ID == excludeTokenID {
					continue
				}
				tokenMap[alloc.Token.ID] = true
				// 对共享TOKEN进行脱敏处理
				detail := TokenAllocationDetail{
					TokenID:        alloc.Token.ID,
					AllocatedAt:    alloc.AllocatedAt,
					Status:         "active",
					StatusDisplay:  "正常",
					TokenStatus:    alloc.Token.Status,
					MaxRequests:    alloc.Token.MaxRequests,
					UsedRequests:   alloc.Token.UsedRequests,
					TenantAddress:  "共享账号",                           // 脱敏
					TokenEmail:     "augmentgateway@augmentcode.com", // 脱敏
					TokenCreatedAt: &alloc.Token.CreatedAt,
					IsCurrentUsing: false,
					IsSharedToken:  true,
				}
				list = append(list, detail)
			}
		}
	}

	return list, nil
}

// ValidateUserTokenOwnership 验证用户是否拥有该TOKEN的使用权
// 包含：1. 用户提交的TOKEN 2. 系统分配给用户的共享TOKEN
func (s *TokenAllocationService) ValidateUserTokenOwnership(userID uint, tokenID string) (*database.Token, error) {
	// 1. 先检查是否为用户提交的TOKEN
	var token database.Token
	err := s.db.Where("id = ? AND submitter_user_id = ?", tokenID, userID).First(&token).Error
	if err == nil {
		// 检查TOKEN是否活跃
		if token.Status != "active" {
			return nil, errors.New("该账号已被禁用或过期")
		}
		return &token, nil
	}

	// 2. 检查是否为系统分配给用户的共享TOKEN
	var allocation database.SharedTokenAllocation
	err = s.db.Where("token_id = ? AND user_id = ?", tokenID, userID).Preload("Token").First(&allocation).Error
	if err == nil && allocation.Token != nil {
		// 检查TOKEN是否活跃
		if allocation.Token.Status != "active" {
			return nil, errors.New("该账号已被禁用或过期")
		}
		return allocation.Token, nil
	}

	return nil, errors.New("该账号不属于您或不存在")
}

// EnhanceTokenRequest 增强TOKEN请求
type EnhanceTokenRequest struct {
	ChannelID uint `json:"channel_id" binding:"required"`
}

// TokenEnhanceInfo TOKEN增强信息
type TokenEnhanceInfo struct {
	ChannelID    uint   `json:"channel_id"`
	ProviderName string `json:"provider_name"`
}

// EnhanceToken 绑定TOKEN到外部渠道（增强功能）
func (s *TokenAllocationService) EnhanceToken(userID uint, tokenID string, channelID uint) error {
	// 验证TOKEN所有权（包含用户提交的TOKEN和系统分配的共享TOKEN）
	token, err := s.ValidateUserTokenOwnership(userID, tokenID)
	if err != nil {
		return err
	}

	// 验证外部渠道存在且属于该用户
	var channel database.ExternalChannel
	if err := s.db.Where("id = ? AND user_id = ?", channelID, userID).First(&channel).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("外部渠道不存在或不属于您")
		}
		return err
	}

	// 检查是否已存在绑定记录
	var existingBinding database.TokenChannelBinding
	err = s.db.Where("token_id = ? AND user_id = ?", tokenID, userID).First(&existingBinding).Error

	if err == nil {
		// 已存在绑定，更新渠道ID
		existingBinding.ChannelID = channelID
		if err := s.db.Save(&existingBinding).Error; err != nil {
			return errors.New("更新增强绑定失败")
		}
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		// 不存在绑定，创建新绑定
		binding := database.TokenChannelBinding{
			TokenID:   tokenID,
			ChannelID: channelID,
			UserID:    userID,
		}
		if err := s.db.Create(&binding).Error; err != nil {
			return errors.New("创建增强绑定失败")
		}
	} else {
		return err
	}

	// 更新TOKEN的增强状态
	if err := s.db.Model(&token).Update("enhanced_enabled", true).Error; err != nil {
		return errors.New("更新TOKEN增强状态失败")
	}

	// 清除缓存（绑定变更后需要重新从数据库获取最新数据）
	s.invalidateTokenChannelBindingCache(tokenID, userID)

	return nil
}

// RemoveTokenEnhance 解除TOKEN的增强绑定
func (s *TokenAllocationService) RemoveTokenEnhance(userID uint, tokenID string) error {
	// 验证TOKEN所有权（包含用户提交的TOKEN和系统分配的共享TOKEN）
	token, err := s.ValidateUserTokenOwnership(userID, tokenID)
	if err != nil {
		return err
	}

	// 删除绑定记录
	result := s.db.Where("token_id = ? AND user_id = ?", tokenID, userID).Delete(&database.TokenChannelBinding{})
	if result.Error != nil {
		return errors.New("解除增强绑定失败")
	}

	// 更新TOKEN的增强状态
	if err := s.db.Model(&token).Update("enhanced_enabled", false).Error; err != nil {
		return errors.New("更新TOKEN增强状态失败")
	}

	// 清除缓存
	s.invalidateTokenChannelBindingCache(tokenID, userID)

	return nil
}

// GetTokenEnhanceInfo 获取TOKEN的增强绑定信息
func (s *TokenAllocationService) GetTokenEnhanceInfo(userID uint, tokenID string) (*TokenEnhanceInfo, error) {
	var binding database.TokenChannelBinding
	if err := s.db.Preload("Channel").Where("token_id = ? AND user_id = ?", tokenID, userID).First(&binding).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // 未绑定
		}
		return nil, err
	}

	if binding.Channel == nil {
		return nil, nil
	}

	return &TokenEnhanceInfo{
		ChannelID:    binding.ChannelID,
		ProviderName: binding.Channel.ProviderName,
	}, nil
}

// GetTokenEnhanceInfoBatch 批量获取TOKEN的增强绑定信息
func (s *TokenAllocationService) GetTokenEnhanceInfoBatch(userID uint, tokenIDs []string) (map[string]*TokenEnhanceInfo, error) {
	if len(tokenIDs) == 0 {
		return make(map[string]*TokenEnhanceInfo), nil
	}

	var bindings []database.TokenChannelBinding
	if err := s.db.Preload("Channel").Where("token_id IN ? AND user_id = ?", tokenIDs, userID).Find(&bindings).Error; err != nil {
		return nil, err
	}

	result := make(map[string]*TokenEnhanceInfo)
	for _, binding := range bindings {
		if binding.Channel != nil {
			result[binding.TokenID] = &TokenEnhanceInfo{
				ChannelID:    binding.ChannelID,
				ProviderName: binding.Channel.ProviderName,
			}
		}
	}

	return result, nil
}

// invalidateTokenChannelBindingCache 清除TOKEN渠道绑定缓存
func (s *TokenAllocationService) invalidateTokenChannelBindingCache(tokenID string, userID uint) {
	if s.cacheService == nil {
		return
	}
	ctx := context.Background()
	if err := s.cacheService.InvalidateTokenChannelBinding(ctx, tokenID, userID); err != nil {
		logger.Warnf("[TOKEN分配] 清除TOKEN渠道绑定缓存失败: tokenID=%s, userID=%d, error=%v", tokenID, userID, err)
	} else {
		logger.Infof("[TOKEN分配] 已清除TOKEN渠道绑定缓存: tokenID=%s, userID=%d", tokenID, userID)
	}
}
