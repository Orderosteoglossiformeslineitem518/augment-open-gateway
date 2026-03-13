package handler

import (
	"github.com/gin-gonic/gin"
)

// Version 系统版本号
const Version = "v0.0.1"

// SystemHandler 系统信息处理器
type SystemHandler struct{}

// NewSystemHandler 创建系统信息处理器
func NewSystemHandler() *SystemHandler {
	return &SystemHandler{}
}

// GetVersion 获取系统版本号
func (h *SystemHandler) GetVersion(c *gin.Context) {
	ResponseSuccess(c, gin.H{
		"version": Version,
	})
}
