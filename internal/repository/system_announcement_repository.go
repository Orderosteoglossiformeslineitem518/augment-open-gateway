package repository

import (
	"context"
	"errors"

	"augment-gateway/internal/database"

	"gorm.io/gorm"
)

// SystemAnnouncementRepository 系统公告数据访问接口
type SystemAnnouncementRepository interface {
	// Create 创建公告
	Create(ctx context.Context, announcement *database.SystemAnnouncement) error
	// GetByID 根据ID获取公告
	GetByID(ctx context.Context, id uint) (*database.SystemAnnouncement, error)
	// List 获取公告列表
	List(ctx context.Context) ([]*database.SystemAnnouncement, error)
	// Update 更新公告
	Update(ctx context.Context, announcement *database.SystemAnnouncement) error
	// Delete 删除公告
	Delete(ctx context.Context, id uint) error
	// GetPublished 获取已发布的公告（按创建时间倒序）
	GetPublished(ctx context.Context, limit int) ([]*database.SystemAnnouncement, error)
	// UpdateStatus 更新公告状态
	UpdateStatus(ctx context.Context, id uint, status string) error
}

// systemAnnouncementRepository 系统公告数据访问实现
type systemAnnouncementRepository struct {
	db *gorm.DB
}

// NewSystemAnnouncementRepository 创建系统公告数据访问实例
func NewSystemAnnouncementRepository(db *gorm.DB) SystemAnnouncementRepository {
	return &systemAnnouncementRepository{
		db: db,
	}
}

// Create 创建公告
func (r *systemAnnouncementRepository) Create(ctx context.Context, announcement *database.SystemAnnouncement) error {
	return r.db.WithContext(ctx).Create(announcement).Error
}

// GetByID 根据ID获取公告
func (r *systemAnnouncementRepository) GetByID(ctx context.Context, id uint) (*database.SystemAnnouncement, error) {
	var announcement database.SystemAnnouncement
	err := r.db.WithContext(ctx).First(&announcement, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &announcement, nil
}

// List 获取公告列表
func (r *systemAnnouncementRepository) List(ctx context.Context) ([]*database.SystemAnnouncement, error) {
	var announcements []*database.SystemAnnouncement
	err := r.db.WithContext(ctx).Order("created_at DESC").Find(&announcements).Error
	if err != nil {
		return nil, err
	}
	return announcements, nil
}

// Update 更新公告
func (r *systemAnnouncementRepository) Update(ctx context.Context, announcement *database.SystemAnnouncement) error {
	return r.db.WithContext(ctx).Save(announcement).Error
}

// Delete 删除公告
func (r *systemAnnouncementRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&database.SystemAnnouncement{}, id).Error
}

// GetPublished 获取已发布的公告（按创建时间倒序）
func (r *systemAnnouncementRepository) GetPublished(ctx context.Context, limit int) ([]*database.SystemAnnouncement, error) {
	var announcements []*database.SystemAnnouncement
	err := r.db.WithContext(ctx).
		Where("status = ?", "published").
		Order("created_at DESC").
		Limit(limit).
		Find(&announcements).Error
	if err != nil {
		return nil, err
	}
	return announcements, nil
}

// UpdateStatus 更新公告状态
func (r *systemAnnouncementRepository) UpdateStatus(ctx context.Context, id uint, status string) error {
	return r.db.WithContext(ctx).Model(&database.SystemAnnouncement{}).Where("id = ?", id).Update("status", status).Error
}
