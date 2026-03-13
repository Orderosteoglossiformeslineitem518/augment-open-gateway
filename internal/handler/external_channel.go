package handler

import (
	"net/http"
	"strconv"

	"augment-gateway/internal/service"

	"github.com/gin-gonic/gin"
)

// ExternalChannelHandler 外部渠道处理器
type ExternalChannelHandler struct {
	externalChannelService *service.ExternalChannelService
}

// NewExternalChannelHandler 创建外部渠道处理器
func NewExternalChannelHandler(externalChannelService *service.ExternalChannelService) *ExternalChannelHandler {
	return &ExternalChannelHandler{
		externalChannelService: externalChannelService,
	}
}

// Create 创建外部渠道
// POST /api/v1/user/external-channels
func (h *ExternalChannelHandler) Create(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		ResponseError(c, http.StatusUnauthorized, "请先登录")
		return
	}

	var req service.CreateExternalChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseError(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}

	result, err := h.externalChannelService.Create(userID.(uint), &req)
	if err != nil {
		ResponseError(c, http.StatusBadRequest, err.Error())
		return
	}

	ResponseSuccessWithMsg(c, "外部渠道创建成功", result)
}

// GetList 获取外部渠道列表
// GET /api/v1/user/external-channels
func (h *ExternalChannelHandler) GetList(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		ResponseError(c, http.StatusUnauthorized, "请先登录")
		return
	}

	result, err := h.externalChannelService.GetList(userID.(uint))
	if err != nil {
		ResponseError(c, http.StatusInternalServerError, err.Error())
		return
	}

	ResponseSuccess(c, gin.H{
		"list":            result,
		"internal_models": h.externalChannelService.GetInternalModels(),
	})
}

// GetByID 获取外部渠道详情
// GET /api/v1/user/external-channels/:id
func (h *ExternalChannelHandler) GetByID(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		ResponseError(c, http.StatusUnauthorized, "请先登录")
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		ResponseError(c, http.StatusBadRequest, "无效的渠道ID")
		return
	}

	result, err := h.externalChannelService.GetByID(userID.(uint), uint(id))
	if err != nil {
		ResponseError(c, http.StatusNotFound, err.Error())
		return
	}

	ResponseSuccess(c, result)
}

// Update 更新外部渠道
// PUT /api/v1/user/external-channels/:id
func (h *ExternalChannelHandler) Update(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		ResponseError(c, http.StatusUnauthorized, "请先登录")
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		ResponseError(c, http.StatusBadRequest, "无效的渠道ID")
		return
	}

	var req service.UpdateExternalChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseError(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}

	result, err := h.externalChannelService.Update(userID.(uint), uint(id), &req)
	if err != nil {
		ResponseError(c, http.StatusBadRequest, err.Error())
		return
	}

	ResponseSuccessWithMsg(c, "外部渠道更新成功", result)
}

// Delete 删除外部渠道
// DELETE /api/v1/user/external-channels/:id
func (h *ExternalChannelHandler) Delete(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		ResponseError(c, http.StatusUnauthorized, "请先登录")
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		ResponseError(c, http.StatusBadRequest, "无效的渠道ID")
		return
	}

	if err := h.externalChannelService.Delete(userID.(uint), uint(id)); err != nil {
		ResponseError(c, http.StatusBadRequest, err.Error())
		return
	}

	ResponseSuccessWithMsg(c, "外部渠道删除成功", nil)
}

// GetInternalModels 获取内部模型列表
// GET /api/v1/user/external-channels/internal-models
func (h *ExternalChannelHandler) GetInternalModels(c *gin.Context) {
	ResponseSuccess(c, gin.H{
		"models": h.externalChannelService.GetInternalModels(),
	})
}

// TestChannelRequest 测试渠道请求
type TestChannelRequest struct {
	Model string `json:"model"` // 要测试的模型名称
}

// Test 测试外部渠道连通性
// POST /api/v1/user/external-channels/:id/test
func (h *ExternalChannelHandler) Test(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		ResponseError(c, http.StatusUnauthorized, "请先登录")
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		ResponseError(c, http.StatusBadRequest, "无效的渠道ID")
		return
	}

	var req TestChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseError(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	if req.Model == "" {
		ResponseError(c, http.StatusBadRequest, "请选择要测试的模型")
		return
	}

	result, err := h.externalChannelService.TestChannel(userID.(uint), uint(id), req.Model)
	if err != nil {
		ResponseError(c, http.StatusBadRequest, err.Error())
		return
	}

	ResponseSuccess(c, result)
}

// GetUsageStats 获取外部渠道使用统计
// GET /api/v1/user/external-channels/:id/usage-stats
func (h *ExternalChannelHandler) GetUsageStats(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		ResponseError(c, http.StatusUnauthorized, "请先登录")
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		ResponseError(c, http.StatusBadRequest, "无效的渠道ID")
		return
	}

	// 获取天数参数，默认7天
	daysStr := c.DefaultQuery("days", "7")
	days, err := strconv.Atoi(daysStr)
	if err != nil || (days != 7 && days != 15 && days != 30) {
		days = 7
	}

	result, err := h.externalChannelService.GetChannelUsageStats(userID.(uint), uint(id), days)
	if err != nil {
		ResponseError(c, http.StatusBadRequest, err.Error())
		return
	}

	ResponseSuccess(c, result)
}

// FetchModelsRequest 获取可用模型请求
type FetchModelsRequest struct {
	APIEndpoint string `json:"api_endpoint" binding:"required"`
	APIKey      string `json:"api_key"`
	ChannelID   uint   `json:"channel_id"`
}

// FetchModels 获取外部渠道可用模型列表
// POST /api/v1/user/external-channels/fetch-models
func (h *ExternalChannelHandler) FetchModels(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		ResponseError(c, http.StatusUnauthorized, "请先登录")
		return
	}

	var req FetchModelsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseError(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}

	result, err := h.externalChannelService.FetchAvailableModels(userID.(uint), req.APIEndpoint, req.APIKey, req.ChannelID)
	if err != nil {
		ResponseError(c, http.StatusBadRequest, err.Error())
		return
	}

	ResponseSuccess(c, result)
}
