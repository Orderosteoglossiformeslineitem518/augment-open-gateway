package handler

import (
	"net/http"
	"strconv"
	"strings"

	"augment-gateway/internal/service"

	"github.com/gin-gonic/gin"
)

// NotificationHandler 公告处理器
type NotificationHandler struct {
	notificationService *service.NotificationService
}

// NewNotificationHandler 创建公告处理器
func NewNotificationHandler(notificationService *service.NotificationService) *NotificationHandler {
	return &NotificationHandler{
		notificationService: notificationService,
	}
}

// CreateNotification 创建公告
func (h *NotificationHandler) CreateNotification(c *gin.Context) {
	var req service.CreateNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseError(c, 400, "请求参数错误: "+err.Error())
		return
	}

	notification, err := h.notificationService.CreateNotification(c.Request.Context(), &req)
	if err != nil {
		ResponseError(c, 500, err.Error())
		return
	}

	ResponseSuccessWithMsg(c, "公告创建成功", notification)
}

// GetNotification 获取公告
func (h *NotificationHandler) GetNotification(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		ResponseError(c, 400, "无效的公告ID")
		return
	}

	notification, err := h.notificationService.GetNotification(c.Request.Context(), uint(id))
	if err != nil {
		ResponseError(c, 404, err.Error())
		return
	}

	ResponseSuccess(c, notification)
}

// ListNotifications 获取公告列表
func (h *NotificationHandler) ListNotifications(c *gin.Context) {
	notifications, err := h.notificationService.ListNotificationsWithReadCount(c.Request.Context())
	if err != nil {
		ResponseError(c, 500, err.Error())
		return
	}

	ResponseSuccess(c, gin.H{
		"list":  notifications,
		"total": len(notifications),
	})
}

// UpdateNotification 更新公告
func (h *NotificationHandler) UpdateNotification(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		ResponseError(c, 400, "无效的公告ID")
		return
	}

	var req service.UpdateNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseError(c, 400, "请求参数错误: "+err.Error())
		return
	}

	notification, err := h.notificationService.UpdateNotification(c.Request.Context(), uint(id), &req)
	if err != nil {
		ResponseError(c, 500, err.Error())
		return
	}

	ResponseSuccessWithMsg(c, "公告更新成功", notification)
}

// DeleteNotification 删除公告
func (h *NotificationHandler) DeleteNotification(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		ResponseError(c, 400, "无效的公告ID")
		return
	}

	err = h.notificationService.DeleteNotification(c.Request.Context(), uint(id))
	if err != nil {
		ResponseError(c, 500, err.Error())
		return
	}

	ResponseSuccessWithMsg(c, "公告删除成功", nil)
}

// ReadNotifications 客户端读取公告接口（/notifications/read）
func (h *NotificationHandler) ReadNotifications(c *gin.Context) {
	notifications, err := h.notificationService.GetNotificationForClient(c.Request.Context())
	if err != nil {
		// 如果获取失败，返回空的公告信息
		emptyResponse := service.NotificationsResponse{
			Notifications: []service.NotificationResponse{},
		}
		c.JSON(http.StatusOK, emptyResponse)
		return
	}

	c.JSON(http.StatusOK, notifications)
}

// EnableNotification 启用公告
func (h *NotificationHandler) EnableNotification(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		ResponseError(c, 400, "无效的公告ID")
		return
	}

	err = h.notificationService.EnableNotification(c.Request.Context(), uint(id))
	if err != nil {
		ResponseError(c, 500, err.Error())
		return
	}

	ResponseSuccessWithMsg(c, "公告启用成功", nil)
}

// DisableNotification 禁用公告
func (h *NotificationHandler) DisableNotification(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		ResponseError(c, 400, "无效的公告ID")
		return
	}

	err = h.notificationService.DisableNotification(c.Request.Context(), uint(id))
	if err != nil {
		ResponseError(c, 500, err.Error())
		return
	}

	ResponseSuccessWithMsg(c, "公告禁用成功", nil)
}

// MarkAsRead 标记公告为已读
func (h *NotificationHandler) MarkAsRead(c *gin.Context) {
	// 解析请求体
	var req service.MarkNotificationAsReadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseError(c, 400, "请求参数错误: "+err.Error())
		return
	}

	// 从Authorization头中提取用户token
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		ResponseError(c, 401, "缺少Authorization头")
		return
	}

	// 提取Bearer token
	var userToken string
	if strings.HasPrefix(authHeader, "Bearer ") {
		userToken = strings.TrimPrefix(authHeader, "Bearer ")
	} else {
		// 如果不是Bearer格式，直接使用整个Authorization头的值
		userToken = authHeader
	}

	if userToken == "" {
		ResponseError(c, 401, "无效的token格式")
		return
	}

	// 标记公告为已读
	err := h.notificationService.MarkNotificationAsRead(c.Request.Context(), &req, userToken)
	if err != nil {
		if strings.Contains(err.Error(), "公告不存在") {
			ResponseError(c, 404, err.Error())
			return
		}
		ResponseError(c, 500, err.Error())
		return
	}

	ResponseSuccessWithMsg(c, "标记已读成功", nil)
}
