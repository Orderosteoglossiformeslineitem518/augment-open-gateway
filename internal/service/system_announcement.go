package service

import (
	"context"
	"fmt"

	"augment-gateway/internal/database"
	"augment-gateway/internal/repository"

	"gorm.io/gorm"
)

// CreateSystemAnnouncementRequest 创建系统公告请求
type CreateSystemAnnouncementRequest struct {
	Title   string `json:"title" binding:"required,max=200"`                    // 公告标题
	Content string `json:"content" binding:"required"`                          // 公告内容
	Status  string `json:"status" binding:"required,oneof=published cancelled"` // 发布状态
}

// UpdateSystemAnnouncementRequest 更新系统公告请求
type UpdateSystemAnnouncementRequest struct {
	Title   string `json:"title" binding:"required,max=200"`                    // 公告标题
	Content string `json:"content" binding:"required"`                          // 公告内容
	Status  string `json:"status" binding:"required,oneof=published cancelled"` // 发布状态
}

// SystemAnnouncementService 系统公告服务
type SystemAnnouncementService struct {
	repository repository.SystemAnnouncementRepository
	cache      *CacheService
}

// NewSystemAnnouncementService 创建系统公告服务
func NewSystemAnnouncementService(db *gorm.DB, cache *CacheService) *SystemAnnouncementService {
	return &SystemAnnouncementService{
		repository: repository.NewSystemAnnouncementRepository(db),
		cache:      cache,
	}
}

// CreateAnnouncement 创建公告
func (s *SystemAnnouncementService) CreateAnnouncement(ctx context.Context, req *CreateSystemAnnouncementRequest) (*database.SystemAnnouncement, error) {
	announcement := &database.SystemAnnouncement{
		Title:   req.Title,
		Content: req.Content,
		Status:  req.Status,
	}

	if err := s.repository.Create(ctx, announcement); err != nil {
		return nil, fmt.Errorf("创建公告失败: %w", err)
	}

	return announcement, nil
}

// GetAnnouncement 获取公告
func (s *SystemAnnouncementService) GetAnnouncement(ctx context.Context, id uint) (*database.SystemAnnouncement, error) {
	announcement, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("获取公告失败: %w", err)
	}
	if announcement == nil {
		return nil, fmt.Errorf("公告不存在")
	}
	return announcement, nil
}

// ListAnnouncements 获取公告列表
func (s *SystemAnnouncementService) ListAnnouncements(ctx context.Context) ([]*database.SystemAnnouncement, error) {
	announcements, err := s.repository.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取公告列表失败: %w", err)
	}
	return announcements, nil
}

// UpdateAnnouncement 更新公告
func (s *SystemAnnouncementService) UpdateAnnouncement(ctx context.Context, id uint, req *UpdateSystemAnnouncementRequest) (*database.SystemAnnouncement, error) {
	announcement, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("获取公告失败: %w", err)
	}
	if announcement == nil {
		return nil, fmt.Errorf("公告不存在")
	}

	announcement.Title = req.Title
	announcement.Content = req.Content
	announcement.Status = req.Status

	if err := s.repository.Update(ctx, announcement); err != nil {
		return nil, fmt.Errorf("更新公告失败: %w", err)
	}

	return announcement, nil
}

// DeleteAnnouncement 删除公告
func (s *SystemAnnouncementService) DeleteAnnouncement(ctx context.Context, id uint) error {
	announcement, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("获取公告失败: %w", err)
	}
	if announcement == nil {
		return fmt.Errorf("公告不存在")
	}

	if err := s.repository.Delete(ctx, id); err != nil {
		return fmt.Errorf("删除公告失败: %w", err)
	}

	return nil
}

// PublishAnnouncement 发布公告
func (s *SystemAnnouncementService) PublishAnnouncement(ctx context.Context, id uint) error {
	announcement, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("获取公告失败: %w", err)
	}
	if announcement == nil {
		return fmt.Errorf("公告不存在")
	}

	if err := s.repository.UpdateStatus(ctx, id, "published"); err != nil {
		return fmt.Errorf("发布公告失败: %w", err)
	}

	return nil
}

// CancelAnnouncement 取消公告
func (s *SystemAnnouncementService) CancelAnnouncement(ctx context.Context, id uint) error {
	announcement, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("获取公告失败: %w", err)
	}
	if announcement == nil {
		return fmt.Errorf("公告不存在")
	}

	if err := s.repository.UpdateStatus(ctx, id, "cancelled"); err != nil {
		return fmt.Errorf("取消公告失败: %w", err)
	}

	return nil
}

// GetPublishedAnnouncements 获取已发布的公告（公开接口，用于用户中心）
func (s *SystemAnnouncementService) GetPublishedAnnouncements(ctx context.Context, limit int) ([]*database.SystemAnnouncement, error) {
	if limit <= 0 {
		limit = 5 // 默认返回5条
	}
	announcements, err := s.repository.GetPublished(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("获取已发布公告失败: %w", err)
	}
	return announcements, nil
}

// GetLatestAnnouncementTimestamp 获取最新公告的发布时间戳
func (s *SystemAnnouncementService) GetLatestAnnouncementTimestamp(ctx context.Context) (int64, error) {
	announcements, err := s.repository.GetPublished(ctx, 1)
	if err != nil {
		return 0, fmt.Errorf("获取最新公告失败: %w", err)
	}
	if len(announcements) == 0 {
		return 0, nil // 没有公告
	}
	return announcements[0].CreatedAt.Unix(), nil
}

// HasUnreadAnnouncements 检查用户是否有未读公告
func (s *SystemAnnouncementService) HasUnreadAnnouncements(ctx context.Context, userID uint) (bool, error) {
	// 获取最新公告时间戳
	latestTimestamp, err := s.GetLatestAnnouncementTimestamp(ctx)
	if err != nil {
		return false, err
	}
	if latestTimestamp == 0 {
		return false, nil // 没有公告
	}

	// 获取用户最后阅读时间
	lastReadTimestamp, err := s.cache.GetSystemAnnouncementLastRead(ctx, userID)
	if err != nil {
		return false, err
	}

	// 如果用户从未阅读或最新公告时间晚于用户阅读时间，则有未读
	return latestTimestamp > lastReadTimestamp, nil
}

// MarkAnnouncementsAsRead 标记公告为已读
func (s *SystemAnnouncementService) MarkAnnouncementsAsRead(ctx context.Context, userID uint) error {
	return s.cache.SetSystemAnnouncementLastRead(ctx, userID)
}
