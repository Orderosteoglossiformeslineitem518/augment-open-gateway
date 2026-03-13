package handler

import (
	"strconv"

	"augment-gateway/internal/service"

	"github.com/gin-gonic/gin"
)

// SystemAnnouncementHandler 系统公告处理器
type SystemAnnouncementHandler struct {
	systemAnnouncementService *service.SystemAnnouncementService
}

// NewSystemAnnouncementHandler 创建系统公告处理器
func NewSystemAnnouncementHandler(systemAnnouncementService *service.SystemAnnouncementService) *SystemAnnouncementHandler {
	return &SystemAnnouncementHandler{
		systemAnnouncementService: systemAnnouncementService,
	}
}

// CreateAnnouncement 创建公告
func (h *SystemAnnouncementHandler) CreateAnnouncement(c *gin.Context) {
	var req service.CreateSystemAnnouncementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseError(c, 400, "请求参数错误: "+err.Error())
		return
	}

	announcement, err := h.systemAnnouncementService.CreateAnnouncement(c.Request.Context(), &req)
	if err != nil {
		ResponseError(c, 500, err.Error())
		return
	}

	ResponseSuccessWithMsg(c, "公告创建成功", announcement)
}

// GetAnnouncement 获取公告
func (h *SystemAnnouncementHandler) GetAnnouncement(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		ResponseError(c, 400, "无效的公告ID")
		return
	}

	announcement, err := h.systemAnnouncementService.GetAnnouncement(c.Request.Context(), uint(id))
	if err != nil {
		ResponseError(c, 404, err.Error())
		return
	}

	ResponseSuccess(c, announcement)
}

// ListAnnouncements 获取公告列表
func (h *SystemAnnouncementHandler) ListAnnouncements(c *gin.Context) {
	announcements, err := h.systemAnnouncementService.ListAnnouncements(c.Request.Context())
	if err != nil {
		ResponseError(c, 500, err.Error())
		return
	}

	ResponseSuccess(c, gin.H{
		"list":  announcements,
		"total": len(announcements),
	})
}

// UpdateAnnouncement 更新公告
func (h *SystemAnnouncementHandler) UpdateAnnouncement(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		ResponseError(c, 400, "无效的公告ID")
		return
	}

	var req service.UpdateSystemAnnouncementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseError(c, 400, "请求参数错误: "+err.Error())
		return
	}

	announcement, err := h.systemAnnouncementService.UpdateAnnouncement(c.Request.Context(), uint(id), &req)
	if err != nil {
		ResponseError(c, 500, err.Error())
		return
	}

	ResponseSuccessWithMsg(c, "公告更新成功", announcement)
}

// DeleteAnnouncement 删除公告
func (h *SystemAnnouncementHandler) DeleteAnnouncement(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		ResponseError(c, 400, "无效的公告ID")
		return
	}

	err = h.systemAnnouncementService.DeleteAnnouncement(c.Request.Context(), uint(id))
	if err != nil {
		ResponseError(c, 500, err.Error())
		return
	}

	ResponseSuccessWithMsg(c, "公告删除成功", nil)
}

// PublishAnnouncement 发布公告
func (h *SystemAnnouncementHandler) PublishAnnouncement(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		ResponseError(c, 400, "无效的公告ID")
		return
	}

	err = h.systemAnnouncementService.PublishAnnouncement(c.Request.Context(), uint(id))
	if err != nil {
		ResponseError(c, 500, err.Error())
		return
	}

	ResponseSuccessWithMsg(c, "公告发布成功", nil)
}

// CancelAnnouncement 取消公告
func (h *SystemAnnouncementHandler) CancelAnnouncement(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		ResponseError(c, 400, "无效的公告ID")
		return
	}

	err = h.systemAnnouncementService.CancelAnnouncement(c.Request.Context(), uint(id))
	if err != nil {
		ResponseError(c, 500, err.Error())
		return
	}

	ResponseSuccessWithMsg(c, "公告取消成功", nil)
}

// GetPublishedAnnouncements 获取已发布的公告（公开接口，不需要鉴权）
func (h *SystemAnnouncementHandler) GetPublishedAnnouncements(c *gin.Context) {
	announcements, err := h.systemAnnouncementService.GetPublishedAnnouncements(c.Request.Context(), 5)
	if err != nil {
		ResponseError(c, 500, err.Error())
		return
	}

	// 转换为前端需要的格式
	var result []gin.H
	for _, a := range announcements {
		result = append(result, gin.H{
			"id":         a.ID,
			"title":      a.Title,
			"content":    a.Content,
			"created_at": a.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	ResponseSuccess(c, gin.H{
		"announcements": result,
	})
}

// GetPublishedAnnouncementsWithUnread 获取已发布的公告（需要用户登录，包含未读状态）
func (h *SystemAnnouncementHandler) GetPublishedAnnouncementsWithUnread(c *gin.Context) {
	// 从上下文中获取用户信息
	userIDValue, exists := c.Get("user_id")
	if !exists {
		ResponseError(c, 401, "用户未登录")
		return
	}
	userID, ok := userIDValue.(uint)
	if !ok {
		ResponseError(c, 401, "无效的用户ID")
		return
	}

	ctx := c.Request.Context()

	// 获取公告列表
	announcements, err := h.systemAnnouncementService.GetPublishedAnnouncements(ctx, 5)
	if err != nil {
		ResponseError(c, 500, err.Error())
		return
	}

	// 获取未读状态
	hasUnread, err := h.systemAnnouncementService.HasUnreadAnnouncements(ctx, userID)
	if err != nil {
		// 获取未读状态失败不影响公告列表返回，默认为无未读
		hasUnread = false
	}

	// 转换为前端需要的格式
	var result []gin.H
	for _, a := range announcements {
		result = append(result, gin.H{
			"id":         a.ID,
			"title":      a.Title,
			"content":    a.Content,
			"created_at": a.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	ResponseSuccess(c, gin.H{
		"announcements": result,
		"has_unread":    hasUnread,
	})
}

// MarkAnnouncementsAsRead 标记公告为已读（需要用户登录）
func (h *SystemAnnouncementHandler) MarkAnnouncementsAsRead(c *gin.Context) {
	// 从上下文中获取用户信息
	userIDValue, exists := c.Get("user_id")
	if !exists {
		ResponseError(c, 401, "用户未登录")
		return
	}
	userID, ok := userIDValue.(uint)
	if !ok {
		ResponseError(c, 401, "无效的用户ID")
		return
	}

	err := h.systemAnnouncementService.MarkAnnouncementsAsRead(c.Request.Context(), userID)
	if err != nil {
		ResponseError(c, 500, "标记已读失败: "+err.Error())
		return
	}

	ResponseSuccessWithMsg(c, "标记已读成功", nil)
}
