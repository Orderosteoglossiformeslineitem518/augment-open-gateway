package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"augment-gateway/internal/config"
	"augment-gateway/internal/database"
	"augment-gateway/internal/logger"
	"augment-gateway/internal/repository"
	"augment-gateway/internal/utils"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TokenService Token服务
type TokenService struct {
	db                *gorm.DB
	cache             *CacheService
	config            *config.Config
	snowflake         *SnowflakeGenerator
	repository        repository.TokenRepository
	authSessionClient AuthSessionClient // 用于定时刷新AuthSession
}

// SnowflakeGenerator 雪花ID生成器
type SnowflakeGenerator struct {
	mu        sync.Mutex
	timestamp int64
	nodeID    int64
	sequence  int64
}

// NewSnowflakeGenerator 创建雪花ID生成器
func NewSnowflakeGenerator(nodeID int64) *SnowflakeGenerator {
	return &SnowflakeGenerator{
		nodeID: nodeID & 0x3FF, // 10位节点ID
	}
}

// Generate 生成雪花ID字符串
func (s *SnowflakeGenerator) Generate() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli()

	if now == s.timestamp {
		s.sequence = (s.sequence + 1) & 0xFFF // 12位序列号
		if s.sequence == 0 {
			// 序列号溢出，等待下一毫秒
			for now <= s.timestamp {
				now = time.Now().UnixMilli()
			}
		}
	} else {
		s.sequence = 0
	}

	s.timestamp = now

	// 组装雪花ID: 时间戳(41位) + 节点ID(10位) + 序列号(12位)
	id := ((now - 1640995200000) << 22) | (s.nodeID << 12) | s.sequence // 2022-01-01 00:00:00 UTC作为起始时间
	return fmt.Sprintf("%d", id)
}

// NewTokenService 创建Token服务
func NewTokenService(db *gorm.DB, cache *CacheService, cfg *config.Config) *TokenService {
	return &TokenService{
		db:         db,
		cache:      cache,
		config:     cfg,
		snowflake:  NewSnowflakeGenerator(1), // 使用节点ID 1
		repository: repository.NewTokenRepository(db),
	}
}

// SetAuthSessionClient 设置AuthSessionClient（用于定时刷新AuthSession）
func (s *TokenService) SetAuthSessionClient(client AuthSessionClient) {
	s.authSessionClient = client
}

// TokenInfo Token信息
type TokenInfo struct {
	database.Token
	BanReason         *string `json:"ban_reason,omitempty"`          // 封禁原因，来自ban_records表
	CurrentUsersCount int     `json:"current_users_count,omitempty"` // 当前使用人数
}

// TokenUserInfo TOKEN使用用户信息
type TokenUserInfo struct {
	UserTokenID   string     `json:"user_token_id"`   // 用户令牌ID
	UserToken     string     `json:"user_token"`      // 用户令牌值
	UserTokenName string     `json:"user_token_name"` // 用户令牌名称
	LastUsedAt    *time.Time `json:"last_used_at"`    // 最后使用时间
}

// CreateTokenRequest 创建Token请求
type CreateTokenRequest struct {
	Token           string            `json:"token"` // 可通过AuthSession自动获取
	Description     string            `json:"description"`
	TenantAddress   string            `json:"tenant_address"`    // 可通过AuthSession自动获取
	ReplaceProxyURL string            `json:"replace_proxy_url"` // 替换代理地址，用于租户地址替换
	AuthSession     string            `json:"auth_session"`      // AuthSession信息，用于自动刷新TOKEN
	PortalURL       string            `json:"portal_url"`        // Portal URL（订阅地址）
	Email           string            `json:"email"`             // 邮箱（可通过AuthSession自动获取）
	MaxRequests     int               `json:"max_requests"`
	ExpiresAt       *utils.CustomTime `json:"expires_at"`
	EnhancedEnabled bool              `json:"enhanced_enabled"` // 是否开启增强功能
}

// UpdateTokenRequest 更新Token请求
type UpdateTokenRequest struct {
	Token           string            `json:"token"` // TOKEN值（用于AuthSession刷新）
	Description     string            `json:"description"`
	TenantAddress   string            `json:"tenant_address"`
	ProxyURL        *string           `json:"proxy_url"`
	AuthSession     *string           `json:"auth_session"` // AuthSession信息，使用指针类型支持null值
	MaxRequests     int               `json:"max_requests"`
	Status          string            `json:"status"`
	ExpiresAt       *utils.CustomTime `json:"expires_at"`
	EnhancedEnabled *bool             `json:"enhanced_enabled"` // 是否开启增强功能，使用指针类型支持null值
}

// CreateUserSubmittedTokenRequest 用户提交TOKEN请求
type CreateUserSubmittedTokenRequest struct {
	Token             string     `json:"token" binding:"required"`
	TenantAddress     string     `json:"tenant_address" binding:"required"`
	PortalURL         string     `json:"portal_url"`   // Portal URL（订阅地址，可选）
	Email             string     `json:"email"`        // 邮箱（可选）
	AuthSession       string     `json:"auth_session"` // AuthSession信息，用于自动刷新TOKEN（可选）
	ExpiresAt         *time.Time `json:"expires_at"`
	IsShared          bool       `json:"is_shared"`
	MaxRequests       int        `json:"max_requests"`
	SubmitterUserID   uint       `json:"submitter_user_id"`
	SubmitterUsername string     `json:"submitter_username"`
	SessionID         string     `json:"session_id,omitempty"` // 可选的会话ID，如果提供则使用，否则生成新的
}

// GenerateToken 生成安全的token
func (s *TokenService) GenerateToken() string {
	// 生成随机字节
	bytes := make([]byte, s.config.Token.Length/2)
	if _, err := rand.Read(bytes); err != nil {
		// 如果随机数生成失败，使用UUID作为备选
		return s.config.Token.Prefix + uuid.New().String()
	}

	return s.config.Token.Prefix + hex.EncodeToString(bytes)
}

// CreateToken 创建新token
func (s *TokenService) CreateToken(ctx context.Context, req *CreateTokenRequest) (*TokenInfo, error) {
	// 检查token唯一性
	existingToken, err := s.repository.GetByToken(ctx, req.Token)
	if err == nil && existingToken != nil {
		return nil, fmt.Errorf("token already exists")
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("failed to check token uniqueness: %w", err)
	}

	// 自动补充租户地址末尾的斜杠
	if !strings.HasSuffix(req.TenantAddress, "/") {
		req.TenantAddress = req.TenantAddress + "/"
	}

	// 生成会话ID
	sessionID := s.generateSessionID()

	// 设置默认值：只有负数时才使用默认值30000，0是有效值（表示0积分账号）
	if req.MaxRequests < 0 {
		req.MaxRequests = 30000
	}

	// 设置默认过期时间（1年后）
	var expiresAt *time.Time
	if req.ExpiresAt == nil {
		expiry := time.Now().AddDate(1, 0, 0) // 1年后
		expiresAt = &expiry
	} else {
		// 从CustomTime转换为time.Time
		expiry := req.ExpiresAt.Time
		expiresAt = &expiry
	}

	// 获取邮箱：优先使用请求中已有的邮箱（从AuthSession获取），否则尝试通过get-models接口获取
	var email string
	if req.Email != "" {
		email = req.Email
		logger.Infof("[TOKEN管理] 使用AuthSession获取的邮箱: %s", email)
	} else {
		fetchedEmail, err := s.getUserEmailFromModels(req.TenantAddress, req.Token, sessionID)
		if err != nil {
			logger.Infof("[TOKEN管理] 获取邮箱失败: %v，继续创建TOKEN\n", err)
		} else {
			email = fetchedEmail
			logger.Infof("[TOKEN管理] 成功通过get-models获取邮箱: %s\n", email)
		}
	}

	// 创建token记录
	tokenRecord := &database.Token{
		ID:              s.snowflake.Generate(), // 使用雪花ID
		Token:           req.Token,
		Name:            req.Token, // 使用token作为名称
		Description:     req.Description,
		TenantAddress:   req.TenantAddress,
		SessionID:       sessionID,
		AuthSession:     req.AuthSession, // 设置AuthSession信息
		PortalURL:       &req.PortalURL,  // 设置Portal URL
		Status:          "active",
		MaxRequests:     req.MaxRequests,
		UsedRequests:    0,
		ExpiresAt:       expiresAt,
		EnhancedEnabled: req.EnhancedEnabled, // 设置增强功能状态
		Email:           &email,              // 保存获取到的邮箱
	}

	if err := s.repository.Create(ctx, tokenRecord); err != nil {
		if strings.Contains(err.Error(), "Duplicate entry") {
			return nil, fmt.Errorf("TOKEN已存在，请勿重复创建")
		}
		return nil, fmt.Errorf("创建TOKEN失败: %w", err)
	}

	// 修复 GORM 零值问题：当 MaxRequests 为 0 时，GORM 会使用数据库默认值 30000
	// 需要显式更新为 0
	if req.MaxRequests == 0 {
		if err := s.UpdateMaxRequests(ctx, tokenRecord.ID, 0); err != nil {
			logger.Warnf("[TokenService] 更新 max_requests 为 0 失败: %v", err)
		}
	}

	// 缓存token
	if err := s.cache.CacheToken(ctx, tokenRecord); err != nil {
		// 缓存失败不影响主流程
		logger.Warnf("Warning: failed to cache token: %v\n", err)
	}

	// 新创建的活跃token会影响活跃tokens列表，使缓存失效
	if err := s.cache.InvalidateActiveTokens(ctx); err != nil {
		logger.Warnf("Warning: failed to invalidate active tokens cache: %v\n", err)
	}

	return &TokenInfo{Token: *tokenRecord}, nil
}

// GetToken 获取token信息
func (s *TokenService) GetToken(ctx context.Context, id string) (*TokenInfo, error) {
	token, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("token not found: %w", err)
	}

	return &TokenInfo{Token: *token}, nil
}

// GetTokenByString 根据token字符串获取token信息
func (s *TokenService) GetTokenByString(ctx context.Context, tokenStr string) (*TokenInfo, error) {
	// 先从缓存获取
	cachedToken, hit, err := s.cache.GetToken(ctx, tokenStr)
	if err != nil {
		return nil, fmt.Errorf("获取TOKEN缓存错误: %w", err)
	}

	if hit && cachedToken != nil {
		return &TokenInfo{Token: *cachedToken}, nil
	}

	// 从数据库获取
	token, err := s.repository.GetByToken(ctx, tokenStr)
	if err != nil {
		return nil, fmt.Errorf("令牌未找到: %w", err)
	}

	// 缓存token
	if err := s.cache.CacheToken(ctx, token); err != nil {
		logger.Warnf("警告: 缓存令牌失败: %v\n", err)
	}

	return &TokenInfo{Token: *token}, nil
}

// UpdateToken 更新token
func (s *TokenService) UpdateToken(ctx context.Context, id string, req *UpdateTokenRequest) (*TokenInfo, error) {
	token, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("token not found: %w", err)
	}

	// 自动补充租户地址末尾的斜杠
	if req.TenantAddress != "" && !strings.HasSuffix(req.TenantAddress, "/") {
		req.TenantAddress = req.TenantAddress + "/"
	}

	// 更新字段
	updates := make(map[string]interface{})
	if req.Token != "" {
		updates["token"] = req.Token
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.TenantAddress != "" {
		updates["tenant_address"] = req.TenantAddress
	}
	if req.ProxyURL != nil {
		updates["proxy_url"] = *req.ProxyURL
	}
	if req.AuthSession != nil {
		updates["auth_session"] = *req.AuthSession
	}
	if req.MaxRequests > 0 {
		updates["max_requests"] = req.MaxRequests
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}
	if req.ExpiresAt != nil {
		updates["expires_at"] = req.ExpiresAt
	}
	if req.EnhancedEnabled != nil {
		updates["enhanced_enabled"] = *req.EnhancedEnabled
	}

	// 更新token字段
	for field, value := range updates {
		switch field {
		case "token":
			token.Token = value.(string)
		case "description":
			token.Description = value.(string)
		case "tenant_address":
			token.TenantAddress = value.(string)
		case "proxy_url":
			if value != nil {
				proxyURL := value.(string)
				token.ProxyURL = &proxyURL
			}
		case "auth_session":
			if value != nil {
				authSession := value.(string)
				token.AuthSession = authSession
			}
		case "max_requests":
			token.MaxRequests = value.(int)
		case "status":
			token.Status = value.(string)
		case "expires_at":
			if value != nil {
				customTime := value.(*utils.CustomTime)
				token.ExpiresAt = &customTime.Time
			}
		case "enhanced_enabled":
			token.EnhancedEnabled = value.(bool)
		}
	}

	if err := s.repository.Update(ctx, token); err != nil {
		return nil, fmt.Errorf("failed to update token: %w", err)
	}

	// 使缓存失效
	if err := s.cache.InvalidateToken(ctx, token.Token); err != nil {
		logger.Warnf("Warning: failed to invalidate token cache: %v\n", err)
	}

	// 如果更新的是状态字段，使活跃tokens缓存失效
	if _, statusUpdated := updates["status"]; statusUpdated {
		if err := s.cache.InvalidateActiveTokens(ctx); err != nil {
			logger.Warnf("Warning: failed to invalidate active tokens cache: %v\n", err)
		}
	}

	// 重新获取更新后的token
	updatedToken, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get updated token: %w", err)
	}

	return &TokenInfo{Token: *updatedToken}, nil
}

// DeleteToken 删除token
func (s *TokenService) DeleteToken(ctx context.Context, id string) error {
	token, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("token not found: %w", err)
	}

	// 软删除
	if err := s.repository.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete token: %w", err)
	}

	// 使缓存失效
	if err := s.cache.InvalidateToken(ctx, token.Token); err != nil {
		logger.Warnf("Warning: failed to invalidate token cache: %v\n", err)
	}

	// 删除token会影响活跃tokens列表，使缓存失效
	if err := s.cache.InvalidateActiveTokens(ctx); err != nil {
		logger.Warnf("Warning: failed to invalidate active tokens cache: %v\n", err)
	}

	return nil
}

// ListTokens 列出tokens
func (s *TokenService) ListTokens(ctx context.Context, page, pageSize int, status, search, submitterUsername, isShared string) ([]*TokenInfo, int64, error) {
	tokensWithBanReason, total, err := s.repository.ListWithPaginationAndBanReason(ctx, page, pageSize, status, search, submitterUsername, isShared)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list tokens: %w", err)
	}

	// 转换为TokenInfo
	var tokenInfos []*TokenInfo
	var tokenIDs []string

	for _, tokenWithBan := range tokensWithBanReason {
		tokenInfo := &TokenInfo{
			Token:             tokenWithBan.Token,
			BanReason:         tokenWithBan.BanReason,
			CurrentUsersCount: 0, // 初始化为0，确保字段始终存在
		}
		tokenInfos = append(tokenInfos, tokenInfo)
		tokenIDs = append(tokenIDs, tokenWithBan.Token.ID)
	}

	// 批量获取TOKEN使用人数统计
	if len(tokenIDs) > 0 {
		usersCountMap, err := s.cache.GetTokenCurrentUsersCount(ctx, tokenIDs)
		if err != nil {
			// 获取使用人数失败时记录警告，但不影响主流程
			logger.Warnf("警告: 获取TOKEN使用人数统计失败: %v\n", err)
		} else {
			// 将使用人数填充到TokenInfo中
			for _, tokenInfo := range tokenInfos {
				if count, exists := usersCountMap[tokenInfo.Token.ID]; exists {
					tokenInfo.CurrentUsersCount = count
				}
				// 如果不存在，保持初始化的0值
			}
		}
	}

	return tokenInfos, total, nil
}

// GetTokenUsers 获取TOKEN的使用用户列表
// 注意：由于用户令牌已迁移到 users.api_token，此函数已简化
func (s *TokenService) GetTokenUsers(ctx context.Context, tokenID string) ([]*TokenUserInfo, error) {
	// 扫描所有用户TOKEN分配缓存，找到使用该TOKEN的用户
	pattern := "AUGMENT-GATEWAY:user_token_assignment:*"
	keys, err := s.cache.ScanKeys(ctx, pattern)
	if err != nil {
		return nil, fmt.Errorf("扫描用户TOKEN分配缓存失败: %w", err)
	}

	var userTokens []string

	// 批量获取所有分配记录
	if len(keys) > 0 {
		redisClient := s.cache.GetRedisClient()
		pipe := redisClient.GetClient().Pipeline()
		cmds := make([]*redis.StringCmd, len(keys))
		for i, key := range keys {
			cmds[i] = pipe.Get(ctx, key)
		}

		_, err = pipe.Exec(ctx)
		if err != nil && !errors.Is(err, redis.Nil) {
			return nil, fmt.Errorf("批量获取TOKEN分配失败: %w", err)
		}

		// 找到使用指定TOKEN的用户令牌
		for i, cmd := range cmds {
			assignedTokenID, err := cmd.Result()
			if err == nil && assignedTokenID == tokenID {
				key := keys[i]
				if userToken := extractUserTokenFromKey(key); userToken != "" {
					userTokens = append(userTokens, userToken)
				}
			}
		}
	}

	if len(userTokens) == 0 {
		return []*TokenUserInfo{}, nil
	}

	// 从 users 表查询用户信息
	var users []database.User
	err = s.db.WithContext(ctx).Where("api_token IN ?", userTokens).Find(&users).Error
	if err != nil {
		return nil, fmt.Errorf("批量查询用户信息失败: %w", err)
	}

	// 创建用户映射表
	userMap := make(map[string]*database.User)
	for i := range users {
		userMap[users[i].ApiToken] = &users[i]
	}

	// 构建结果列表
	var tokenUsers []*TokenUserInfo
	for _, userToken := range userTokens {
		user, exists := userMap[userToken]
		if !exists {
			logger.Warnf("警告: 未找到用户信息 %s\n", userToken[:min(8, len(userToken))])
			continue
		}

		tokenUser := &TokenUserInfo{
			UserTokenID:   fmt.Sprintf("%d", user.ID),
			UserToken:     userToken,
			UserTokenName: user.Username,
			LastUsedAt:    nil, // 简化：不再查询最后使用时间
		}
		tokenUsers = append(tokenUsers, tokenUser)
	}

	return tokenUsers, nil
}

// extractUserTokenFromKey 从缓存键中提取用户令牌
// 缓存键格式: AUGMENT-GATEWAY:user_token_assignment:{userToken}
func extractUserTokenFromKey(key string) string {
	prefix := "AUGMENT-GATEWAY:user_token_assignment:"
	if strings.HasPrefix(key, prefix) {
		return key[len(prefix):]
	}
	return ""
}

// ValidateToken 验证token
func (s *TokenService) ValidateToken(ctx context.Context, tokenStr string) (*TokenInfo, error) {
	tokenInfo, err := s.GetTokenByString(ctx, tokenStr)
	if err != nil {
		return nil, err
	}

	// 检查token是否有效
	if !tokenInfo.IsActive() {
		return nil, fmt.Errorf("token is inactive or expired")
	}

	return tokenInfo, nil
}

// GetActiveTokens 获取所有活跃的tokens
func (s *TokenService) GetActiveTokens(ctx context.Context) ([]*database.Token, error) {
	// 先从缓存获取
	cachedTokens, hit, err := s.cache.GetCachedActiveTokens(ctx)
	if err == nil && hit && cachedTokens != nil {
		return cachedTokens, nil
	}

	// 缓存未命中，从数据库获取
	tokens, err := s.repository.GetActiveTokens(ctx)
	if err != nil {
		return nil, err
	}

	// 异步缓存结果
	go func() {
		cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.cache.CacheActiveTokens(cacheCtx, tokens); err != nil {
			logger.Warnf("Warning: failed to cache active tokens: %v\n", err)
		}
	}()

	return tokens, nil
}

// generateSessionID 生成会话ID
func (s *TokenService) generateSessionID() string {
	return uuid.New().String()
}

// UpdateTokenStatus 更新Token状态
func (s *TokenService) UpdateTokenStatus(ctx context.Context, tokenID string, status string) error {
	token, err := s.repository.GetByID(ctx, tokenID)
	if err != nil {
		return fmt.Errorf("token not found: %w", err)
	}

	// 更新状态
	token.Status = status
	if err := s.repository.Update(ctx, token); err != nil {
		return fmt.Errorf("failed to update token status: %w", err)
	}

	// 使缓存失效
	if err := s.cache.InvalidateToken(ctx, token.Token); err != nil {
		logger.Warnf("Warning: failed to invalidate token cache: %v\n", err)
	}

	// 状态更新会影响活跃tokens列表，使缓存失效
	if err := s.cache.InvalidateActiveTokens(ctx); err != nil {
		logger.Warnf("Warning: failed to invalidate active tokens cache: %v\n", err)
	}

	// 如果TOKEN状态变为非活跃状态（disabled、inactive、expired），清除模拟设备数据
	if status == "disabled" || status == "inactive" || status == "expired" {
		tokenDisplay := token.Token
		if len(tokenDisplay) > 8 {
			tokenDisplay = tokenDisplay[:8]
		}

		// 清除模拟会话事件数据
		if err := s.cache.DeleteSessionEvents(ctx, token.Token); err != nil {
			logger.Warnf("Warning: 清除TOKEN %s... 的模拟会话事件数据失败: %v\n", tokenDisplay, err)
		} else {
			logger.Infof("✅ 已清除TOKEN %s... 的模拟会话事件数据\n", tokenDisplay)
		}

		// 清除模拟特征向量数据
		if err := s.cache.DeleteFeatureVector(ctx, token.Token); err != nil {
			logger.Warnf("Warning: 清除TOKEN %s... 的模拟特征向量数据失败: %v\n", tokenDisplay, err)
		} else {
			logger.Infof("✅ 已清除TOKEN %s... 的模拟特征向量数据\n", tokenDisplay)
		}
	}

	return nil
}

// GetTokenStats 获取token统计信息
func (s *TokenService) GetTokenStats(ctx context.Context) (map[string]interface{}, error) {
	var stats struct {
		Total    int64 `json:"total"`
		Active   int64 `json:"active"`
		Expired  int64 `json:"expired"`
		Disabled int64 `json:"disabled"`
	}

	var err error

	// 总数
	stats.Total, err = s.repository.CountTotal(ctx)
	if err != nil {
		return nil, err
	}

	// 正常数
	stats.Active, err = s.repository.CountActive(ctx)
	if err != nil {
		return nil, err
	}

	// 过期数
	stats.Expired, err = s.repository.CountExpired(ctx)
	if err != nil {
		return nil, err
	}

	// 禁用数
	stats.Disabled, err = s.repository.CountDisabled(ctx)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"total":    stats.Total,
		"active":   stats.Active,
		"expired":  stats.Expired,
		"disabled": stats.Disabled,
	}, nil
}

// UpdateTokenExpiry 更新TOKEN过期时间
func (s *TokenService) UpdateTokenExpiry(tokenID string, expiryTime time.Time) error {
	// 更新数据库中的TOKEN过期时间
	result := s.db.Model(&database.Token{}).
		Where("id = ?", tokenID).
		Update("expires_at", expiryTime)

	if result.Error != nil {
		return fmt.Errorf("更新TOKEN过期时间失败: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("TOKEN不存在: %s", tokenID)
	}

	// 清除相关缓存
	ctx := context.Background()
	s.cache.InvalidateActiveTokens(ctx)

	return nil
}

// UpdateUsedRequests 更新TOKEN已使用次数
func (s *TokenService) UpdateUsedRequests(ctx context.Context, tokenID string, usedRequests int) error {
	// 更新数据库中的TOKEN已使用次数
	result := s.db.WithContext(ctx).Model(&database.Token{}).
		Where("id = ?", tokenID).
		Update("used_requests", usedRequests)

	if result.Error != nil {
		return fmt.Errorf("更新TOKEN已使用次数失败: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("TOKEN不存在: %s", tokenID)
	}

	// 清除相关缓存
	s.cache.InvalidateActiveTokens(ctx)

	return nil
}

// UpdateMaxRequests 更新TOKEN最大请求次数（账号类型）
func (s *TokenService) UpdateMaxRequests(ctx context.Context, tokenID string, maxRequests int) error {
	// 更新数据库中的TOKEN最大请求次数
	result := s.db.WithContext(ctx).Model(&database.Token{}).
		Where("id = ?", tokenID).
		Update("max_requests", maxRequests)

	if result.Error != nil {
		return fmt.Errorf("更新TOKEN最大请求次数失败: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("TOKEN不存在: %s", tokenID)
	}

	// 清除相关缓存
	s.cache.InvalidateActiveTokens(ctx)

	return nil
}

// UpdateTokenEmail 更新TOKEN邮箱信息
func (s *TokenService) UpdateTokenEmail(tokenID string, email string) error {
	// 更新数据库中的TOKEN邮箱信息
	result := s.db.Model(&database.Token{}).
		Where("id = ?", tokenID).
		Update("email", email)

	if result.Error != nil {
		return fmt.Errorf("更新TOKEN邮箱失败: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("TOKEN不存在: %s", tokenID)
	}

	// 清除相关缓存
	ctx := context.Background()
	s.cache.InvalidateActiveTokens(ctx)

	return nil
}

// StartAuthSessionRefreshScheduler 启动AuthSession定时刷新任务（每15天刷新一次）
func (s *TokenService) StartAuthSessionRefreshScheduler(ctx context.Context) {
	if s.authSessionClient == nil {
		logger.Infof("[TOKEN服务] AuthSessionClient未设置，AuthSession定时刷新任务未启动")
		return
	}

	ticker := time.NewTicker(15 * 24 * time.Hour) // 每15天执行一次
	defer ticker.Stop()

	logger.Infof("[TOKEN服务] AuthSession定时刷新任务已启动，每15天执行一次")

	// 不立即执行，等待第一个周期
	for {
		select {
		case <-ctx.Done():
			logger.Infof("[TOKEN服务] AuthSession定时刷新任务已停止")
			return
		case <-ticker.C:
			// 刷新所有配置了AuthSession的TOKEN
			s.refreshAllAuthSessions(ctx)
		}
	}
}

// refreshAllAuthSessions 刷新所有配置了AuthSession的TOKEN
func (s *TokenService) refreshAllAuthSessions(ctx context.Context) {
	logger.Infof("[TOKEN服务] 开始定时刷新AuthSession...")

	// 查询所有配置了AuthSession且状态为active的TOKEN
	var tokens []database.Token
	if err := s.db.Where("auth_session != '' AND auth_session IS NOT NULL AND status = ?", "active").Find(&tokens).Error; err != nil {
		logger.Infof("[TOKEN服务] 查询AuthSession TOKEN失败: %v\n", err)
		return
	}

	if len(tokens) == 0 {
		logger.Infof("[TOKEN服务] 没有需要刷新的AuthSession TOKEN")
		return
	}

	logger.Infof("[TOKEN服务] 找到 %d 个配置了AuthSession的TOKEN，开始刷新\n", len(tokens))

	successCount := 0
	failedCount := 0

	for _, token := range tokens {
		tokenPrefix := token.Token[:min(8, len(token.Token))]

		// 验证AuthSession是否有效
		if err := s.authSessionClient.ValidateAuthSession(token.AuthSession); err != nil {
			logger.Infof("[TOKEN服务] TOKEN %s... AuthSession验证失败: %v，跳过\n", tokenPrefix, err)
			failedCount++
			continue
		}

		// 通过AuthSession获取新的TOKEN、租户地址和新的AuthSession
		tenantURL, accessToken, _, newAuthSession, err := s.authSessionClient.AuthDevice(token.AuthSession)
		if err != nil {
			logger.Infof("[TOKEN服务] TOKEN %s... 刷新失败: %v，跳过\n", tokenPrefix, err)
			failedCount++
			continue
		}

		// 更新数据库
		newTenantAddress := strings.TrimSuffix(tenantURL, "/") + "/"
		updateReq := &UpdateTokenRequest{
			Token:         accessToken,
			TenantAddress: newTenantAddress,
			AuthSession:   &newAuthSession,
		}

		if _, err := s.UpdateToken(ctx, token.ID, updateReq); err != nil {
			logger.Infof("[TOKEN服务] TOKEN %s... 更新数据库失败: %v，跳过\n", tokenPrefix, err)
			failedCount++
			continue
		}

		successCount++
		logger.Infof("[TOKEN服务] TOKEN %s... 刷新成功\n", tokenPrefix)
	}

	logger.Infof("[TOKEN服务] AuthSession刷新完成：成功 %d，失败 %d\n", successCount, failedCount)
}

// BatchImportTokenItem 批量导入TOKEN项
type BatchImportTokenItem struct {
	Token         string `json:"token"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	TenantAddress string `json:"tenant_address"`
}

// BatchImportResult 批量导入结果
type BatchImportResult struct {
	Total        int                     `json:"total"`
	SuccessCount int                     `json:"success_count"`
	FailedCount  int                     `json:"failed_count"`
	Results      []BatchImportItemResult `json:"results"`
}

// BatchImportItemResult 批量导入单项结果
type BatchImportItemResult struct {
	Token     string `json:"token"`
	Name      string `json:"name"`
	Success   bool   `json:"success"`
	SessionID string `json:"session_id,omitempty"`
	ID        string `json:"id,omitempty"`
	Error     string `json:"error,omitempty"`
}

// BatchImportTokens 批量导入TOKEN
func (s *TokenService) BatchImportTokens(ctx context.Context, items []*BatchImportTokenItem) (*BatchImportResult, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("导入列表不能为空")
	}

	result := &BatchImportResult{
		Total:   len(items),
		Results: make([]BatchImportItemResult, 0, len(items)),
	}

	// 预处理：检查TOKEN唯一性
	tokenMap := make(map[string]*BatchImportTokenItem)
	duplicateTokens := make(map[string]bool)

	for _, item := range items {
		if _, exists := tokenMap[item.Token]; exists {
			duplicateTokens[item.Token] = true
		} else {
			tokenMap[item.Token] = item
		}
	}

	// 批量检查数据库中已存在的TOKEN
	existingTokens := make(map[string]bool)
	if len(tokenMap) > 0 {
		tokens := make([]string, 0, len(tokenMap))
		for token := range tokenMap {
			tokens = append(tokens, token)
		}

		// 批量查询已存在的TOKEN
		var existingRecords []database.Token
		if err := s.db.Where("token IN ?", tokens).Find(&existingRecords).Error; err != nil {
			return nil, fmt.Errorf("检查TOKEN唯一性失败: %w", err)
		}

		for _, record := range existingRecords {
			existingTokens[record.Token] = true
		}
	}

	// 准备批量插入的数据
	var tokensToInsert []database.Token
	var mu sync.Mutex

	// 处理每个TOKEN
	for _, item := range items {
		itemResult := BatchImportItemResult{
			Token: item.Token,
			Name:  item.Name,
		}

		// 检查重复
		if duplicateTokens[item.Token] {
			itemResult.Success = false
			itemResult.Error = "请求中存在重复的TOKEN"
			result.Results = append(result.Results, itemResult)
			result.FailedCount++
			continue
		}

		// 检查是否已存在
		if existingTokens[item.Token] {
			itemResult.Success = false
			itemResult.Error = "TOKEN已存在"
			result.Results = append(result.Results, itemResult)
			result.FailedCount++
			continue
		}

		// 生成会话ID和其他默认值
		sessionID := s.generateSessionID()
		expiry := time.Now().AddDate(1, 0, 0) // 1年后

		// 创建TOKEN记录
		tokenRecord := database.Token{
			ID:            s.snowflake.Generate(),
			Token:         item.Token,
			Name:          item.Name,
			Description:   item.Description,
			TenantAddress: item.TenantAddress,
			SessionID:     sessionID,
			Status:        "active",
			MaxRequests:   30000,
			UsedRequests:  0,
			ExpiresAt:     &expiry,
		}

		mu.Lock()
		tokensToInsert = append(tokensToInsert, tokenRecord)
		mu.Unlock()

		itemResult.Success = true
		itemResult.SessionID = sessionID
		itemResult.ID = tokenRecord.ID
		result.Results = append(result.Results, itemResult)
		result.SuccessCount++
	}

	// 批量插入到数据库
	if len(tokensToInsert) > 0 {
		// 使用事务进行批量插入
		err := s.db.Transaction(func(tx *gorm.DB) error {
			// 批量插入，每次最多1000条
			batchSize := 1000
			for i := 0; i < len(tokensToInsert); i += batchSize {
				end := i + batchSize
				if end > len(tokensToInsert) {
					end = len(tokensToInsert)
				}

				if err := tx.Create(tokensToInsert[i:end]).Error; err != nil {
					return fmt.Errorf("批量插入TOKEN失败: %w", err)
				}
			}
			return nil
		})

		if err != nil {
			// 如果批量插入失败，更新所有成功项为失败
			for i := range result.Results {
				if result.Results[i].Success {
					result.Results[i].Success = false
					result.Results[i].Error = "数据库插入失败: " + err.Error()
					result.Results[i].SessionID = ""
					result.Results[i].ID = ""
				}
			}
			result.SuccessCount = 0
			result.FailedCount = result.Total
			return result, nil // 返回结果而不是错误，让调用者知道具体失败情况
		}

		// 异步缓存成功插入的TOKEN
		go func() {
			cacheCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			for _, token := range tokensToInsert {
				if err := s.cache.CacheToken(cacheCtx, &token); err != nil {
					logger.Warnf("Warning: failed to cache token %s: %v\n", token.Token, err)
				}
			}

			// 清除活跃TOKEN缓存
			s.cache.InvalidateActiveTokens(cacheCtx)
		}()
	}

	return result, nil
}

// CreateUserSubmittedToken 创建用户提交的TOKEN
func (s *TokenService) CreateUserSubmittedToken(req *CreateUserSubmittedTokenRequest) (string, error) {
	// 检查token唯一性（使用Unscoped查询，包括已软删除的记录，因为唯一索引覆盖所有记录）
	var existingToken database.Token
	err := s.db.Unscoped().Where("token = ?", req.Token).First(&existingToken).Error
	if err == nil {
		// 找到已存在的记录
		if existingToken.Status == "active" && !existingToken.DeletedAt.Valid {
			// 活跃状态的TOKEN不允许重复提交
			return "", fmt.Errorf("TOKEN已存在，请勿重复提交")
		}
		// 已禁用或已删除的记录，硬删除后允许重新提交
		if delErr := s.db.Unscoped().Delete(&existingToken).Error; delErr != nil {
			return "", fmt.Errorf("清理旧TOKEN记录失败，请稍后重试")
		}
		_ = s.cache.InvalidateToken(context.Background(), existingToken.Token)
		_ = s.cache.InvalidateActiveTokens(context.Background())
		logger.Infof("[TokenService] 已清理旧TOKEN记录（状态: %s），允许重新提交", existingToken.Status)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", fmt.Errorf("检查TOKEN唯一性失败，请稍后重试")
	}

	//// 如果有邮箱信息，检查邮箱重复性
	//if req.Email != "" {
	//	existingEmailToken, err := s.repository.GetByEmail(context.Background(), req.Email)
	//	if err == nil && existingEmailToken != nil {
	//		return "", fmt.Errorf("当前TOKEN已提交过，请勿重复提交")
	//	}
	//	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
	//		return "", fmt.Errorf("检查邮箱唯一性失败: %w", err)
	//	}
	//}

	// 使用提供的sessionID，如果没有提供则生成新的
	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = s.generateSessionID()
		logger.Infof("[TokenService] 未提供sessionID，生成新的: %s\n", sessionID)
	} else {
		logger.Infof("[TokenService] 使用提供的sessionID: %s\n", sessionID)
	}

	// 如果没有提供邮箱，尝试通过get-models接口获取
	email := req.Email
	if email == "" {
		logger.Infof("[TokenService] 未提供邮箱，尝试通过get-models接口获取\n")
		fetchedEmail, err := s.getUserEmailFromModels(req.TenantAddress, req.Token, sessionID)
		if err != nil {
			logger.Infof("[TokenService] 获取邮箱失败: %v，继续创建TOKEN\n", err)
		} else {
			email = fetchedEmail
			logger.Infof("[TokenService] 成功获取邮箱: %s\n", email)
		}
	}

	// 调试日志：打印TokenService接收到的is_shared值
	logger.Infof("[TokenService] 接收到的is_shared值: %v, 即将保存到数据库\n", req.IsShared)

	// 创建token记录
	tokenRecord := &database.Token{
		ID:                s.snowflake.Generate(),
		Token:             req.Token,
		Name:              "用户提交-" + req.SubmitterUsername,
		Description:       fmt.Sprintf("用户%s提交的TOKEN，邮箱：%s", req.SubmitterUsername, email),
		TenantAddress:     req.TenantAddress,
		SessionID:         sessionID,
		AuthSession:       req.AuthSession, // 保存AuthSession信息
		Status:            "active",
		MaxRequests:       req.MaxRequests, // 使用用户选择的次数
		UsedRequests:      0,
		ExpiresAt:         req.ExpiresAt,
		SubmitterUserID:   &req.SubmitterUserID,
		SubmitterUsername: &req.SubmitterUsername,
		IsShared:          &req.IsShared,
		Email:             &email,         // 保存邮箱信息（使用获取到的邮箱）
		PortalURL:         &req.PortalURL, // 保存Portal URL
	}

	// 调试日志：打印即将保存到数据库的record的is_shared值
	logger.Infof("[TokenService] Token记录的is_shared值: %v, 准备保存到数据库\n", func() interface{} {
		if tokenRecord.IsShared != nil {
			return *tokenRecord.IsShared
		}
		return "nil"
	}())

	// 保存到数据库
	if err := s.repository.Create(context.Background(), tokenRecord); err != nil {
		if strings.Contains(err.Error(), "Duplicate entry") {
			return "", fmt.Errorf("TOKEN已存在，请勿重复提交")
		}
		return "", fmt.Errorf("创建TOKEN失败，请稍后重试")
	}

	// 修复 GORM 零值问题：当 MaxRequests 为 0 时，GORM 会使用数据库默认值 30000
	// 需要显式更新为 0
	if req.MaxRequests == 0 {
		if err := s.UpdateMaxRequests(context.Background(), tokenRecord.ID, 0); err != nil {
			logger.Warnf("[TokenService] 更新 max_requests 为 0 失败: %v", err)
		}
	}

	// 调试日志：验证数据库保存后的值
	savedToken, err := s.repository.GetByID(context.Background(), tokenRecord.ID)
	if err == nil {
		logger.Infof("[TokenService] 数据库保存后验证 - TOKEN ID: %s, is_shared: %v\n", savedToken.ID, func() interface{} {
			if savedToken.IsShared != nil {
				return *savedToken.IsShared
			}
			return "nil"
		}())
	} else {
		logger.Infof("[TokenService] 验证数据库保存失败: %v\n", err)
	}

	// 异步缓存TOKEN
	go func() {
		cacheCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := s.cache.CacheToken(cacheCtx, tokenRecord); err != nil {
			logger.Warnf("Warning: 缓存TOKEN失败 %s: %v\n", tokenRecord.Token, err)
		}

		// 清除活跃TOKEN缓存
		s.cache.InvalidateActiveTokens(cacheCtx)

		// 如果是共享TOKEN，清除用户的共享TOKEN数量缓存
		if req.IsShared {
			if err := s.InvalidateUserSharedTokenCountCache(req.SubmitterUserID); err != nil {
				logger.Warnf("Warning: 清除用户共享TOKEN数量缓存失败: %v\n", err)
			}
		}

		// 注意：这里无法直接清除用户的"无TOKEN"状态缓存，因为我们不知道用户的userToken
		// 这个清理工作应该在handler层面完成，因为handler有用户的userToken信息
	}()

	return tokenRecord.ID, nil
}

// ClearUserNoTokenCache 清除用户的"无TOKEN"状态缓存
func (s *TokenService) ClearUserNoTokenCache(ctx context.Context, userToken string) error {
	noTokenCacheKey := fmt.Sprintf("AUGMENT-GATEWAY:user_no_token:%s", userToken)
	return s.cache.DeleteKey(ctx, noTokenCacheKey)
}

// GetUserSubmittedActiveTokens 获取用户提交的有效TOKEN，按过期时间排序（即将过期的在前）
func (s *TokenService) GetUserSubmittedActiveTokens(ctx context.Context, submitterUserID uint) ([]*database.Token, error) {
	// 使用QueryBuilder构建查询，移除使用次数限制，与GetActiveTokens保持一致
	query := repository.NewQueryBuilder().
		WhereEq("submitter_user_id", submitterUserID).
		WhereEq("status", "active").
		Where("(expires_at IS NULL OR expires_at > ?)", time.Now()).
		OrderByAsc("expires_at")

	tokens, err := s.repository.List(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("获取用户提交的有效TOKEN失败: %w", err)
	}

	return tokens, nil
}

// GetUserSubmittedTokens 获取用户提交的TOKEN列表
func (s *TokenService) GetUserSubmittedTokens(userID uint, page, pageSize int) ([]database.Token, int64, error) {
	offset := (page - 1) * pageSize

	var tokens []database.Token
	var total int64

	// 查询用户提交的TOKEN
	query := s.db.Model(&database.Token{}).Where("submitter_user_id = ?", userID)

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("获取TOKEN总数失败: %w", err)
	}

	// 获取分页数据
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&tokens).Error; err != nil {
		return nil, 0, fmt.Errorf("获取TOKEN列表失败: %w", err)
	}

	return tokens, total, nil
}

// GetUserSubmittedTokensWithBanReason 获取用户提交的TOKEN列表（包含封禁原因）
func (s *TokenService) GetUserSubmittedTokensWithBanReason(userID uint, page, pageSize int) ([]*repository.TokenWithBanReason, int64, error) {
	offset := (page - 1) * pageSize

	// 构建基础查询条件，使用Model确保逻辑删除生效
	db := s.db.Model(&database.Token{}).Where("submitter_user_id = ?", userID)

	// 获取总数
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("获取TOKEN总数失败: %w", err)
	}

	// 分页查询，左连接ban_records表获取封禁原因
	var results []*repository.TokenWithBanReason

	// 重新构建查询，包含JOIN和分页，使用Model确保逻辑删除生效
	query := s.db.Model(&database.Token{}).
		Select("tokens.*, br.ban_reason").
		Joins("LEFT JOIN ban_records br ON tokens.id = br.token_id").
		Where("tokens.submitter_user_id = ?", userID).
		Order("tokens.created_at DESC").
		Limit(pageSize).
		Offset(offset)

	err := query.Scan(&results).Error
	if err != nil {
		return nil, 0, fmt.Errorf("获取TOKEN列表失败: %w", err)
	}

	return results, total, nil
}

// GetUserDailySubmissionCount 获取用户今日提交TOKEN数量
func (s *TokenService) GetUserDailySubmissionCount(userID uint) (int, error) {
	today := time.Now().Format("2006-01-02")
	startTime := today + " 00:00:00"
	endTime := today + " 23:59:59"

	var count int64
	err := s.db.Model(&database.Token{}).
		Where("submitter_user_id = ? AND created_at BETWEEN ? AND ?", userID, startTime, endTime).
		Count(&count).Error

	if err != nil {
		return 0, fmt.Errorf("获取用户今日提交数量失败: %w", err)
	}

	return int(count), nil
}


// HasUserSubmittedSharedToken 检查用户是否提交过共享TOKEN
func (s *TokenService) HasUserSubmittedSharedToken(userID uint) (bool, error) {
	var count int64
	err := s.db.Model(&database.Token{}).
		Where("submitter_user_id = ? AND is_shared = ?", userID, true).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("检查用户共享TOKEN提交状态失败: %w", err)
	}
	return count > 0, nil
}

// GetUserSharedTokenCount 获取用户提交的共享TOKEN数量（使用Redis缓存）
func (s *TokenService) GetUserSharedTokenCount(userID uint) (int, error) {
	ctx := context.Background()
	cacheKey := fmt.Sprintf("AUGMENT-GATEWAY:user_shared_token_count:%d", userID)

	// 先尝试从Redis缓存获取
	cachedCount, err := s.cache.GetString(ctx, cacheKey)
	if err == nil {
		// 缓存命中，解析并返回
		if count, parseErr := strconv.Atoi(cachedCount); parseErr == nil {
			logger.Infof("[缓存] 用户%d共享TOKEN数量缓存命中: %d\n", userID, count)
			return count, nil
		}
	}

	// 缓存未命中或解析失败，查询数据库，只计算活跃且未过期的共享TOKEN
	var count int64
	err = s.db.Model(&database.Token{}).
		Where("submitter_user_id = ? AND is_shared = ? AND status = ? AND (expires_at IS NULL OR expires_at > ?)",
			userID, true, "active", time.Now()).
		Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("获取用户共享TOKEN数量失败: %w", err)
	}

	// 将结果缓存到Redis，设置1小时过期时间
	countStr := strconv.Itoa(int(count))
	if cacheErr := s.cache.SetString(ctx, cacheKey, countStr, time.Hour); cacheErr != nil {
		// 缓存失败不影响主流程，只记录日志
		logger.Infof("[缓存] 缓存用户%d共享TOKEN数量失败: %v\n", userID, cacheErr)
	} else {
		logger.Infof("[缓存] 用户%d共享TOKEN数量已缓存: %d\n", userID, int(count))
	}

	return int(count), nil
}

// InvalidateUserSharedTokenCountCache 清除用户共享TOKEN数量缓存
func (s *TokenService) InvalidateUserSharedTokenCountCache(userID uint) error {
	ctx := context.Background()
	cacheKey := fmt.Sprintf("AUGMENT-GATEWAY:user_shared_token_count:%d", userID)

	err := s.cache.DeleteKey(ctx, cacheKey)
	if err != nil {
		logger.Infof("[缓存] 清除用户%d共享TOKEN数量缓存失败: %v\n", userID, err)
		return err
	}

	logger.Infof("[缓存] 已清除用户%d共享TOKEN数量缓存\n", userID)
	return nil
}

// getUserEmailFromModels 通过get-models接口获取用户邮箱
func (s *TokenService) getUserEmailFromModels(tenantAddress, token, sessionID string) (string, error) {
	// 构建请求URL
	url := strings.TrimSuffix(tenantAddress, "/") + "/get-models"

	// 创建请求 - 使用POST方法并发送空JSON数据
	requestBody := []byte("{}")
	req, err := http.NewRequest("POST", url, bytes.NewReader(requestBody))
	if err != nil {
		return "", fmt.Errorf("创建/get-models请求失败: %w", err)
	}

	// 生成请求ID
	requestID := uuid.New().String()

	// 提取主机名
	host := s.extractHost(tenantAddress)

	// 设置请求头（参考subscription_validator.go中的setCommonRequestHeaders）
	req.Header.Set("host", host)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", s.config.Subscription.UserAgent)
	req.Header.Set("x-request-id", requestID)
	req.Header.Set("x-request-session-id", sessionID)
	req.Header.Set("x-api-version", "2")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("accept", "*/*")
	req.Header.Set("accept-language", "*")
	req.Header.Set("sec-fetch-mode", "cors")

	// 发送请求
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("发送/get-models请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取/get-models响应失败: %w", err)
	}

	// 检查HTTP状态码
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("/get-models请求失败，状态码: %d", resp.StatusCode)
	}

	// 解析响应
	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("解析/get-models响应失败: %w", err)
	}

	// 提取 user.email
	user, ok := response["user"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("/get-models响应中未找到user字段")
	}

	email, ok := user["email"].(string)
	if !ok || email == "" {
		return "", fmt.Errorf("/get-models响应中未找到有效的email字段")
	}

	return email, nil
}

// extractHost 从租户地址中提取主机名
func (s *TokenService) extractHost(tenantAddress string) string {
	// 移除协议前缀
	host := strings.TrimPrefix(tenantAddress, "https://")
	host = strings.TrimPrefix(host, "http://")

	// 移除路径部分
	if idx := strings.Index(host, "/"); idx != -1 {
		host = host[:idx]
	}

	return host
}
