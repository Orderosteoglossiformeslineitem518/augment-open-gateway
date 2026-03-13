package service

import (
	"context"

	"augment-gateway/internal/database"

	"gorm.io/gorm"
)

// SystemConfigService 系统配置服务
type SystemConfigService struct {
	db *gorm.DB
}

// NewSystemConfigService 创建系统配置服务
func NewSystemConfigService(db *gorm.DB) *SystemConfigService {
	return &SystemConfigService{db: db}
}

// GetSystemConfig 获取系统配置
func (s *SystemConfigService) GetSystemConfig(ctx context.Context) (*database.SystemConfig, error) {
	return database.GetSystemConfig(s.db)
}

// UpdateSystemConfigRequest 更新系统配置请求
type UpdateSystemConfigRequest struct {
	RegistrationEnabled *bool   `json:"registration_enabled"`
	DefaultRateLimit    *int    `json:"default_rate_limit"`
	MaintenanceMode     *bool   `json:"maintenance_mode"`
	MaintenanceMessage  *string `json:"maintenance_message"`
}

// UpdateSystemConfig 更新系统配置
func (s *SystemConfigService) UpdateSystemConfig(ctx context.Context, req *UpdateSystemConfigRequest) (*database.SystemConfig, error) {
	config, err := database.GetSystemConfig(s.db)
	if err != nil {
		return nil, err
	}

	// 更新字段
	if req.RegistrationEnabled != nil {
		config.RegistrationEnabled = *req.RegistrationEnabled
	}
	if req.DefaultRateLimit != nil {
		config.DefaultRateLimit = *req.DefaultRateLimit
	}
	if req.MaintenanceMode != nil {
		config.MaintenanceMode = *req.MaintenanceMode
	}
	if req.MaintenanceMessage != nil {
		config.MaintenanceMessage = *req.MaintenanceMessage
	}

	// 保存更新
	if err := s.db.Save(config).Error; err != nil {
		return nil, err
	}

	return config, nil
}

// SystemStats 系统统计信息
type SystemStats struct {
	TotalUsers     int64 `json:"total_users"`
	ActiveUsers    int64 `json:"active_users"`
	AssignedTokens int64 `json:"assigned_tokens"`
	BannedUsers    int64 `json:"banned_users"`
}

// GetSystemStats 获取系统统计信息
func (s *SystemConfigService) GetSystemStats(ctx context.Context) (*SystemStats, error) {
	stats := &SystemStats{}

	// 获取总用户数
	if err := s.db.WithContext(ctx).Model(&database.User{}).Count(&stats.TotalUsers).Error; err != nil {
		return nil, err
	}

	// 获取活跃用户数（status = active）
	if err := s.db.WithContext(ctx).Model(&database.User{}).Where("status = ?", "active").Count(&stats.ActiveUsers).Error; err != nil {
		return nil, err
	}

	// 获取封禁用户数（status = banned）
	if err := s.db.WithContext(ctx).Model(&database.User{}).Where("status = ?", "banned").Count(&stats.BannedUsers).Error; err != nil {
		return nil, err
	}

	// 获取已分配令牌数（allocated_to_id 不为空表示已分配给用户）
	if err := s.db.WithContext(ctx).Model(&database.Token{}).Where("allocated_to_id IS NOT NULL").Count(&stats.AssignedTokens).Error; err != nil {
		return nil, err
	}

	return stats, nil
}
