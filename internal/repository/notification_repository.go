package repository

import (
	"context"
	"errors"

	"augment-gateway/internal/database"

	"gorm.io/gorm"
)

// NotificationRepository 公告数据访问接口
type NotificationRepository interface {
	// Create 创建公告
	Create(ctx context.Context, notification *database.Notification) error
	// GetByID 根据ID获取公告
	GetByID(ctx context.Context, id uint) (*database.Notification, error)
	// List 获取公告列表
	List(ctx context.Context) ([]*database.Notification, error)
	// Update 更新公告
	Update(ctx context.Context, notification *database.Notification) error
	// Delete 删除公告
	Delete(ctx context.Context, id uint) error
	// GetActive 获取活跃的公告（用于客户端接口）
	GetActive(ctx context.Context) (*database.Notification, error)
	// EnableNotification 启用公告（同时禁用其他公告）
	EnableNotification(ctx context.Context, id uint) error
	// DisableNotification 禁用公告
	DisableNotification(ctx context.Context, id uint) error
	// GetEnabledNotification 获取当前启用的公告
	GetEnabledNotification(ctx context.Context) (*database.Notification, error)
}

// notificationRepository 公告数据访问实现
type notificationRepository struct {
	db *gorm.DB
}

// NewNotificationRepository 创建公告数据访问实例
func NewNotificationRepository(db *gorm.DB) NotificationRepository {
	return &notificationRepository{
		db: db,
	}
}

// Create 创建公告
func (r *notificationRepository) Create(ctx context.Context, notification *database.Notification) error {
	return r.db.WithContext(ctx).Create(notification).Error
}

// GetByID 根据ID获取公告
func (r *notificationRepository) GetByID(ctx context.Context, id uint) (*database.Notification, error) {
	var notification database.Notification
	err := r.db.WithContext(ctx).First(&notification, id).Error
	if err != nil {
		return nil, err
	}
	return &notification, nil
}

// List 获取公告列表
func (r *notificationRepository) List(ctx context.Context) ([]*database.Notification, error) {
	var notifications []*database.Notification
	err := r.db.WithContext(ctx).Order("created_at DESC").Find(&notifications).Error
	return notifications, err
}

// Update 更新公告
func (r *notificationRepository) Update(ctx context.Context, notification *database.Notification) error {
	return r.db.WithContext(ctx).Save(notification).Error
}

// Delete 删除公告
func (r *notificationRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&database.Notification{}, id).Error
}

// GetActive 获取活跃的公告（用于客户端接口）
// 返回启用状态的公告，如果没有启用的公告则返回nil
func (r *notificationRepository) GetActive(ctx context.Context) (*database.Notification, error) {
	var notification database.Notification
	err := r.db.WithContext(ctx).Where("enabled = ?", true).Order("created_at DESC").First(&notification).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // 没有启用的公告时返回nil，不是错误
		}
		return nil, err
	}
	return &notification, nil
}

// EnableNotification 启用公告（同时禁用其他公告）
func (r *notificationRepository) EnableNotification(ctx context.Context, id uint) error {
	// 使用事务确保原子性操作
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 先禁用所有公告
		if err := tx.Model(&database.Notification{}).Where("enabled = ?", true).Update("enabled", false).Error; err != nil {
			return err
		}

		// 2. 启用指定公告
		if err := tx.Model(&database.Notification{}).Where("id = ?", id).Update("enabled", true).Error; err != nil {
			return err
		}

		return nil
	})
}

// DisableNotification 禁用公告
func (r *notificationRepository) DisableNotification(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&database.Notification{}).Where("id = ?", id).Update("enabled", false).Error
}

// GetEnabledNotification 获取当前启用的公告
func (r *notificationRepository) GetEnabledNotification(ctx context.Context) (*database.Notification, error) {
	var notification database.Notification
	err := r.db.WithContext(ctx).Where("enabled = ?", true).First(&notification).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // 没有启用的公告时返回nil，不是错误
		}
		return nil, err
	}
	return &notification, nil
}
