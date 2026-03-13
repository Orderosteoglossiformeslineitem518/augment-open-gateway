package handler

import (
	"augment-gateway/internal/service"

	"github.com/gin-gonic/gin"
)

// SystemConfigHandler 系统配置处理器
type SystemConfigHandler struct {
	systemConfigService *service.SystemConfigService
}

// NewSystemConfigHandler 创建系统配置处理器
func NewSystemConfigHandler(systemConfigService *service.SystemConfigService) *SystemConfigHandler {
	return &SystemConfigHandler{
		systemConfigService: systemConfigService,
	}
}

// GetSystemConfig 获取系统配置
func (h *SystemConfigHandler) GetSystemConfig(c *gin.Context) {
	config, err := h.systemConfigService.GetSystemConfig(c.Request.Context())
	if err != nil {
		ResponseError(c, 500, "获取系统配置失败: "+err.Error())
		return
	}

	ResponseSuccess(c, config)
}

// UpdateSystemConfig 更新系统配置
func (h *SystemConfigHandler) UpdateSystemConfig(c *gin.Context) {
	var req service.UpdateSystemConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseError(c, 400, "请求参数错误: "+err.Error())
		return
	}

	config, err := h.systemConfigService.UpdateSystemConfig(c.Request.Context(), &req)
	if err != nil {
		ResponseError(c, 500, "更新系统配置失败: "+err.Error())
		return
	}

	ResponseSuccessWithMsg(c, "系统配置更新成功", config)
}

// GetSystemStats 获取系统统计信息
func (h *SystemConfigHandler) GetSystemStats(c *gin.Context) {
	stats, err := h.systemConfigService.GetSystemStats(c.Request.Context())
	if err != nil {
		ResponseError(c, 500, "获取系统统计失败: "+err.Error())
		return
	}

	ResponseSuccess(c, stats)
}
