package service

import (
	"context"
	"fmt"

	"augment-gateway/internal/database"
	"augment-gateway/internal/logger"
	"augment-gateway/internal/repository"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// NotificationService 公告服务
type NotificationService struct {
	db         *gorm.DB
	repository repository.NotificationRepository
	cache      *CacheService
}

// NewNotificationService 创建公告服务
func NewNotificationService(db *gorm.DB, cache *CacheService) *NotificationService {
	return &NotificationService{
		db:         db,
		repository: repository.NewNotificationRepository(db),
		cache:      cache,
	}
}

// CreateNotificationRequest 创建公告请求
type CreateNotificationRequest struct {
	NotificationID string `json:"notification_id" binding:"max=36"`            // 通知ID，UUID格式，可选（为空时自动生成）
	Level          int    `json:"level" binding:"required,min=1,max=3"`        // 公告等级：1=信息，2=警告，3=错误
	Message        string `json:"message" binding:"required,max=35"`           // 公告消息内容，限制35个字符
	ActionTitle    string `json:"action_title" binding:"max=50"`               // 操作按钮标题，可选
	ActionURL      string `json:"action_url" binding:"max=255"`                // 操作按钮链接，可选
	DisplayType    int    `json:"display_type" binding:"required,min=1,max=2"` // 显示类型：1=TOAST，2=BANNER
	Enabled        bool   `json:"enabled"`                                     // 启用状态：true=启用，false=禁用，默认false
}

// UpdateNotificationRequest 更新公告请求
type UpdateNotificationRequest struct {
	NotificationID string `json:"notification_id" binding:"required,max=36"`   // 通知ID，UUID格式，必填
	Level          int    `json:"level" binding:"required,min=1,max=3"`        // 公告等级：1=信息，2=警告，3=错误
	Message        string `json:"message" binding:"required,max=35"`           // 公告消息内容，限制35个字符
	ActionTitle    string `json:"action_title" binding:"max=50"`               // 操作按钮标题，可选
	ActionURL      string `json:"action_url" binding:"max=255"`                // 操作按钮链接，可选
	DisplayType    int    `json:"display_type" binding:"required,min=1,max=2"` // 显示类型：1=TOAST，2=BANNER
	Enabled        bool   `json:"enabled"`                                     // 启用状态：true=启用，false=禁用
}

// ActionItem 操作项
type ActionItem struct {
	Title string `json:"title"` // 操作按钮显示文字
	URL   string `json:"url"`   // 点击按钮后打开的链接
}

// NotificationResponse 公告响应（用于客户端接口）
type NotificationResponse struct {
	NotificationID string       `json:"notification_id"` // UUID格式
	Level          int          `json:"level"`           // 公告等级：1=信息，2=警告，3=错误
	Message        string       `json:"message"`         // 公告消息内容
	ActionItems    []ActionItem `json:"action_items"`    // 操作项数组
	DisplayType    int          `json:"display_type"`    // 显示类型：1=TOAST，2=BANNER
}

// NotificationsResponse 公告列表响应（用于客户端接口）
type NotificationsResponse struct {
	Notifications []NotificationResponse `json:"notifications"`
}

// CreateNotification 创建公告
func (s *NotificationService) CreateNotification(ctx context.Context, req *CreateNotificationRequest) (*database.Notification, error) {
	// 如果没有提供通知ID，则自动生成
	notificationID := req.NotificationID
	if notificationID == "" {
		notificationID = uuid.New().String()
	}

	notification := &database.Notification{
		NotificationID: notificationID,
		Level:          req.Level,
		Message:        req.Message,
		ActionTitle:    req.ActionTitle,
		ActionURL:      req.ActionURL,
		DisplayType:    req.DisplayType,
		Enabled:        req.Enabled, // 默认为false，需要手动启用
	}

	if err := s.repository.Create(ctx, notification); err != nil {
		return nil, fmt.Errorf("创建公告失败: %w", err)
	}

	return notification, nil
}

// GetNotification 获取公告
func (s *NotificationService) GetNotification(ctx context.Context, id uint) (*database.Notification, error) {
	notification, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("获取公告失败: %w", err)
	}
	return notification, nil
}

// NotificationWithReadCount 带已读数量的公告信息
type NotificationWithReadCount struct {
	*database.Notification
	ReadCount int64 `json:"read_count"` // 已读数量
}

// ListNotifications 获取公告列表
func (s *NotificationService) ListNotifications(ctx context.Context) ([]*database.Notification, error) {
	notifications, err := s.repository.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取公告列表失败: %w", err)
	}
	return notifications, nil
}

// ListNotificationsWithReadCount 获取带已读数量的公告列表
func (s *NotificationService) ListNotificationsWithReadCount(ctx context.Context) ([]*NotificationWithReadCount, error) {
	notifications, err := s.repository.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取公告列表失败: %w", err)
	}

	var result []*NotificationWithReadCount
	for _, notification := range notifications {
		// 获取已读数量
		readCount, err := s.cache.GetNotificationReadCount(ctx, notification.NotificationID)
		if err != nil {
			// 如果获取已读数量失败，设置为0并记录警告
			readCount = 0
			logger.Warnf("警告: 获取公告 %s 已读数量失败: %v\n", notification.NotificationID, err)
		}

		result = append(result, &NotificationWithReadCount{
			Notification: notification,
			ReadCount:    readCount,
		})
	}

	return result, nil
}

// UpdateNotification 更新公告
func (s *NotificationService) UpdateNotification(ctx context.Context, id uint, req *UpdateNotificationRequest) (*database.Notification, error) {
	// 先获取现有公告
	notification, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("公告不存在: %w", err)
	}

	// 更新字段
	notification.NotificationID = req.NotificationID
	notification.Level = req.Level
	notification.Message = req.Message
	notification.ActionTitle = req.ActionTitle
	notification.ActionURL = req.ActionURL
	notification.DisplayType = req.DisplayType
	notification.Enabled = req.Enabled

	if err := s.repository.Update(ctx, notification); err != nil {
		return nil, fmt.Errorf("更新公告失败: %w", err)
	}

	return notification, nil
}

// DeleteNotification 删除公告
func (s *NotificationService) DeleteNotification(ctx context.Context, id uint) error {
	// 先检查公告是否存在
	_, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("公告不存在: %w", err)
	}

	if err := s.repository.Delete(ctx, id); err != nil {
		return fmt.Errorf("删除公告失败: %w", err)
	}

	return nil
}

// GetNotificationForClient 获取客户端公告（用于/notifications/read接口）
func (s *NotificationService) GetNotificationForClient(ctx context.Context) (*NotificationsResponse, error) {
	// 获取启用的公告
	notification, err := s.repository.GetActive(ctx)
	if err != nil {
		// 获取失败时返回空的公告信息
		return &NotificationsResponse{
			Notifications: []NotificationResponse{},
		}, nil
	}

	// 如果没有启用的公告，返回空的公告信息
	if notification == nil {
		return &NotificationsResponse{
			Notifications: []NotificationResponse{},
		}, nil
	}

	// 构建操作项
	var actionItems []ActionItem
	if notification.ActionTitle != "" && notification.ActionURL != "" {
		actionItems = append(actionItems, ActionItem{
			Title: notification.ActionTitle,
			URL:   notification.ActionURL,
		})
	}

	// 返回数据库中的启用公告
	notificationResponse := NotificationResponse{
		NotificationID: notification.NotificationID,
		Level:          notification.Level,
		Message:        notification.Message,
		ActionItems:    actionItems,
		DisplayType:    notification.DisplayType,
	}

	return &NotificationsResponse{
		Notifications: []NotificationResponse{notificationResponse},
	}, nil
}

// EnableNotification 启用公告（同时禁用其他公告）
func (s *NotificationService) EnableNotification(ctx context.Context, id uint) error {
	// 先检查公告是否存在
	_, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("公告不存在: %w", err)
	}

	// 启用公告（同时禁用其他公告）
	if err := s.repository.EnableNotification(ctx, id); err != nil {
		return fmt.Errorf("启用公告失败: %w", err)
	}

	return nil
}

// DisableNotification 禁用公告
func (s *NotificationService) DisableNotification(ctx context.Context, id uint) error {
	// 先检查公告是否存在
	_, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("公告不存在: %w", err)
	}

	// 禁用公告
	if err := s.repository.DisableNotification(ctx, id); err != nil {
		return fmt.Errorf("禁用公告失败: %w", err)
	}

	return nil
}

// MarkNotificationAsReadRequest 标记公告已读请求
type MarkNotificationAsReadRequest struct {
	NotificationID  string  `json:"notification_id" binding:"required"` // 公告消息ID
	ActionItemTitle *string `json:"action_item_title"`                  // 操作项标题（可选）
}

// MarkNotificationAsRead 标记公告为已读
func (s *NotificationService) MarkNotificationAsRead(ctx context.Context, req *MarkNotificationAsReadRequest, userToken string) error {
	// 验证notification_id是否存在
	notifications, err := s.repository.List(ctx)
	if err != nil {
		return fmt.Errorf("获取公告列表失败: %w", err)
	}

	// 检查notification_id是否存在
	var notificationExists bool
	for _, notification := range notifications {
		if notification.NotificationID == req.NotificationID {
			notificationExists = true
			break
		}
	}

	if !notificationExists {
		return fmt.Errorf("公告不存在")
	}

	// 标记为已读（Redis会自动去重）
	if err := s.cache.MarkNotificationAsRead(ctx, req.NotificationID, userToken); err != nil {
		return fmt.Errorf("标记公告已读失败: %w", err)
	}

	return nil
}

// GetNotificationReadCount 获取公告已读数量
func (s *NotificationService) GetNotificationReadCount(ctx context.Context, notificationID string) (int64, error) {
	return s.cache.GetNotificationReadCount(ctx, notificationID)
}
