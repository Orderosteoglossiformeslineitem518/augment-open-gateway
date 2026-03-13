package handler

import (
	"net/http"
	"strconv"

	"augment-gateway/internal/service"

	"github.com/gin-gonic/gin"
)

// MonitorHandler 监测处理器
type MonitorHandler struct {
	monitorService *service.MonitorService
}

// NewMonitorHandler 创建监测处理器
func NewMonitorHandler(monitorService *service.MonitorService) *MonitorHandler {
	return &MonitorHandler{monitorService: monitorService}
}

// Create 创建监测配置
// POST /api/v1/user/monitor/configs
func (h *MonitorHandler) Create(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		ResponseError(c, http.StatusUnauthorized, "请先登录")
		return
	}

	var req service.CreateMonitorConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseError(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}

	result, err := h.monitorService.Create(userID.(uint), &req)
	if err != nil {
		ResponseError(c, http.StatusBadRequest, err.Error())
		return
	}

	ResponseSuccessWithMsg(c, "监测配置创建成功", result)
}

// Update 更新监测配置
// PUT /api/v1/user/monitor/configs/:id
func (h *MonitorHandler) Update(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		ResponseError(c, http.StatusUnauthorized, "请先登录")
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		ResponseError(c, http.StatusBadRequest, "无效的配置ID")
		return
	}

	var req service.UpdateMonitorConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseError(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}

	result, err := h.monitorService.Update(userID.(uint), uint(id), &req)
	if err != nil {
		ResponseError(c, http.StatusBadRequest, err.Error())
		return
	}

	ResponseSuccessWithMsg(c, "监测配置更新成功", result)
}

// GetList 获取监测配置列表
// GET /api/v1/user/monitor/configs
func (h *MonitorHandler) GetList(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		ResponseError(c, http.StatusUnauthorized, "请先登录")
		return
	}

	channelName := c.Query("channel_name")
	channelType := c.Query("channel_type")
	status := c.Query("status")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 10
	}

	list, total, err := h.monitorService.GetList(userID.(uint), channelName, channelType, status, page, pageSize)
	if err != nil {
		ResponseError(c, http.StatusInternalServerError, err.Error())
		return
	}

	ResponseSuccess(c, gin.H{
		"list":  list,
		"total": total,
	})
}

// GetDetail 获取监测配置详情
// GET /api/v1/user/monitor/configs/:id
func (h *MonitorHandler) GetDetail(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		ResponseError(c, http.StatusUnauthorized, "请先登录")
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		ResponseError(c, http.StatusBadRequest, "无效的配置ID")
		return
	}

	result, err := h.monitorService.GetDetail(userID.(uint), uint(id))
	if err != nil {
		ResponseError(c, http.StatusBadRequest, err.Error())
		return
	}

	ResponseSuccess(c, result)
}

// ToggleStatus 启用/禁用监测配置
// PATCH /api/v1/user/monitor/configs/:id/status
func (h *MonitorHandler) ToggleStatus(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		ResponseError(c, http.StatusUnauthorized, "请先登录")
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		ResponseError(c, http.StatusBadRequest, "无效的配置ID")
		return
	}

	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseError(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	if err := h.monitorService.ToggleStatus(userID.(uint), uint(id), req.Status); err != nil {
		ResponseError(c, http.StatusBadRequest, err.Error())
		return
	}

	ResponseSuccessWithMsg(c, "状态更新成功", nil)
}

// GetChannelModels 获取渠道可用模型列表
// GET /api/v1/user/monitor/channels/:channel_id/models
func (h *MonitorHandler) GetChannelModels(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		ResponseError(c, http.StatusUnauthorized, "请先登录")
		return
	}

	idStr := c.Param("channel_id")
	channelID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		ResponseError(c, http.StatusBadRequest, "无效的渠道ID")
		return
	}

	models, err := h.monitorService.GetChannelModels(userID.(uint), uint(channelID))
	if err != nil {
		ResponseError(c, http.StatusBadRequest, err.Error())
		return
	}

	ResponseSuccess(c, gin.H{"models": models})
}

// TriggerCheck 主动触发一次监测
// POST /api/v1/user/monitor/configs/:id/trigger
func (h *MonitorHandler) TriggerCheck(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		ResponseError(c, http.StatusUnauthorized, "请先登录")
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		ResponseError(c, http.StatusBadRequest, "无效的配置ID")
		return
	}

	if err := h.monitorService.TriggerCheck(userID.(uint), uint(id)); err != nil {
		ResponseError(c, http.StatusBadRequest, err.Error())
		return
	}

	ResponseSuccessWithMsg(c, "监测已触发", nil)
}

// Delete 删除监测配置
// DELETE /api/v1/user/monitor/configs/:id
func (h *MonitorHandler) Delete(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		ResponseError(c, http.StatusUnauthorized, "请先登录")
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		ResponseError(c, http.StatusBadRequest, "无效的配置ID")
		return
	}

	if err := h.monitorService.Delete(userID.(uint), uint(id)); err != nil {
		ResponseError(c, http.StatusBadRequest, err.Error())
		return
	}

	ResponseSuccessWithMsg(c, "监测配置已删除", nil)
}
