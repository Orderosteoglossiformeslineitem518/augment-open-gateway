package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"augment-gateway/internal/database"
	"augment-gateway/internal/logger"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SessionService 会话服务
type SessionService struct {
	db    *gorm.DB
	cache *CacheService
}

// NewSessionService 创建会话服务
func NewSessionService(db *gorm.DB, cache *CacheService) *SessionService {
	return &SessionService{
		db:    db,
		cache: cache,
	}
}

// SessionInfo 会话信息
type SessionInfo struct {
	SessionID    string    `json:"session_id"`
	TokenID      string    `json:"token_id"`
	Token        string    `json:"token"`
	DeviceID     string    `json:"device_id"`
	UserAgent    string    `json:"user_agent"`
	ClientIP     string    `json:"client_ip"`
	CreatedAt    time.Time `json:"created_at"`
	LastAccessAt time.Time `json:"last_access_at"`
	RequestCount int64     `json:"request_count"`
	IsActive     bool      `json:"is_active"`
}

// GenerateSessionID 生成会话ID
func (s *SessionService) GenerateSessionID() string {
	// 生成32字节随机数据
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		// 如果随机数生成失败，使用UUID作为备选
		return uuid.New().String()
	}
	return hex.EncodeToString(bytes)
}

// CreateSession 创建会话
func (s *SessionService) CreateSession(ctx context.Context, tokenID string, token string, userAgent, clientIP string) (*SessionInfo, error) {
	sessionID := s.GenerateSessionID()
	deviceID := s.generateDeviceID(userAgent, clientIP)

	sessionInfo := &SessionInfo{
		SessionID:    sessionID,
		TokenID:      tokenID,
		Token:        token,
		DeviceID:     deviceID,
		UserAgent:    userAgent,
		ClientIP:     clientIP,
		CreatedAt:    time.Now(),
		LastAccessAt: time.Now(),
		RequestCount: 0,
		IsActive:     true,
	}

	// 缓存会话信息
	if err := s.cache.SetSession(ctx, sessionID, sessionInfo, 24*time.Hour); err != nil {
		return nil, fmt.Errorf("failed to cache session: %w", err)
	}

	return sessionInfo, nil
}

// GetSession 获取会话信息
func (s *SessionService) GetSession(ctx context.Context, sessionID string) (*SessionInfo, error) {
	var sessionInfo SessionInfo
	if err := s.cache.GetSession(ctx, sessionID, &sessionInfo); err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	return &sessionInfo, nil
}

// UpdateSessionAccess 更新会话访问时间
func (s *SessionService) UpdateSessionAccess(ctx context.Context, sessionID string) error {
	sessionInfo, err := s.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}

	sessionInfo.LastAccessAt = time.Now()
	sessionInfo.RequestCount++

	// 更新缓存
	return s.cache.SetSession(ctx, sessionID, sessionInfo, 24*time.Hour)
}

// InvalidateSession 使会话失效
func (s *SessionService) InvalidateSession(ctx context.Context, sessionID string) error {
	return s.cache.DeleteSession(ctx, sessionID)
}

// GetOrCreateSessionForToken 为token获取或创建会话
func (s *SessionService) GetOrCreateSessionForToken(ctx context.Context, token *database.Token, userAgent, clientIP string) (string, error) {
	// 如果token已经有session_id，直接返回
	if token.SessionID != "" {
		// 检查会话是否仍然有效
		if _, err := s.GetSession(ctx, token.SessionID); err == nil {
			return token.SessionID, nil
		}
	}

	// 创建新会话
	sessionInfo, err := s.CreateSession(ctx, token.ID, token.Token, userAgent, clientIP)
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}

	// 更新token的session_id
	if err := s.updateTokenSessionID(ctx, token.ID, sessionInfo.SessionID); err != nil {
		return "", fmt.Errorf("failed to update token session ID: %w", err)
	}

	return sessionInfo.SessionID, nil
}

// updateTokenSessionID 更新token的session_id
func (s *SessionService) updateTokenSessionID(ctx context.Context, tokenID string, sessionID string) error {
	return s.db.WithContext(ctx).Model(&database.Token{}).
		Where("id = ?", tokenID).
		Update("session_id", sessionID).Error
}

// generateDeviceID 生成设备ID
func (s *SessionService) generateDeviceID(userAgent, clientIP string) string {
	// 基于用户代理和IP生成设备指纹
	_ = fmt.Sprintf("%s:%s:%d", userAgent, clientIP, time.Now().UnixNano())

	// 生成哈希
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return uuid.New().String()
	}

	return fmt.Sprintf("dev_%s", hex.EncodeToString(bytes))
}

// ListActiveSessions 列出活跃会话
func (s *SessionService) ListActiveSessions(ctx context.Context, tokenID uint) ([]*SessionInfo, error) {
	// 这里可以从缓存或数据库获取活跃会话列表
	// 由于Redis中的会话是分散存储的，这里返回空列表
	// 实际实现中可以维护一个会话索引
	return []*SessionInfo{}, nil
}

// CleanupExpiredSessions 清理过期会话
func (s *SessionService) CleanupExpiredSessions(ctx context.Context) error {
	// Redis会自动清理过期的会话
	// 这里可以添加额外的清理逻辑
	return nil
}

// GetSessionStats 获取会话统计
func (s *SessionService) GetSessionStats(ctx context.Context, tokenID uint) (map[string]interface{}, error) {
	stats := map[string]interface{}{
		"active_sessions": 0,
		"total_requests":  0,
		"last_access":     nil,
	}

	// 这里可以实现具体的统计逻辑
	// 由于会话存储在Redis中，需要遍历相关键来统计

	return stats, nil
}

// ValidateSessionID 验证会话ID格式
func (s *SessionService) ValidateSessionID(sessionID string) bool {
	if len(sessionID) != 64 {
		return false
	}

	// 检查是否为有效的十六进制字符串
	_, err := hex.DecodeString(sessionID)
	return err == nil
}

// GetSessionByToken 根据token获取会话
func (s *SessionService) GetSessionByToken(ctx context.Context, tokenStr string) (*SessionInfo, error) {
	// 从数据库获取token信息
	var token database.Token
	if err := s.db.WithContext(ctx).Where("token = ?", tokenStr).First(&token).Error; err != nil {
		return nil, fmt.Errorf("token not found: %w", err)
	}

	if token.SessionID == "" {
		return nil, fmt.Errorf("no session associated with token")
	}

	return s.GetSession(ctx, token.SessionID)
}

// RefreshSession 刷新会话
func (s *SessionService) RefreshSession(ctx context.Context, sessionID string) error {
	sessionInfo, err := s.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}

	// 延长会话过期时间
	return s.cache.SetSession(ctx, sessionID, sessionInfo, 24*time.Hour)
}

// SessionMiddleware 会话中间件数据
type SessionMiddleware struct {
	SessionID string
	TokenID   string
	DeviceID  string
	IsValid   bool
}

// ExtractSessionFromRequest 从请求中提取会话信息
func (s *SessionService) ExtractSessionFromRequest(ctx context.Context, sessionID string) (*SessionMiddleware, error) {
	if sessionID == "" {
		return &SessionMiddleware{IsValid: false}, nil
	}

	if !s.ValidateSessionID(sessionID) {
		return &SessionMiddleware{IsValid: false}, nil
	}

	sessionInfo, err := s.GetSession(ctx, sessionID)
	if err != nil {
		return &SessionMiddleware{IsValid: false}, nil
	}

	return &SessionMiddleware{
		SessionID: sessionInfo.SessionID,
		TokenID:   sessionInfo.TokenID,
		DeviceID:  sessionInfo.DeviceID,
		IsValid:   sessionInfo.IsActive,
	}, nil
}

// GenerateRequestSessionID 为请求生成会话ID（用于X-Request-Session-ID头）
func (s *SessionService) GenerateRequestSessionID(ctx context.Context, token *database.Token, userAgent, clientIP string) (string, error) {
	// 获取或创建会话
	sessionID, err := s.GetOrCreateSessionForToken(ctx, token, userAgent, clientIP)
	if err != nil {
		return "", err
	}

	// 更新访问时间
	if err := s.UpdateSessionAccess(ctx, sessionID); err != nil {
		// 记录警告但不影响主流程
		logger.Warnf("警告: 更新会话访问时间失败: %v\n", err)
	}

	return sessionID, nil
}
