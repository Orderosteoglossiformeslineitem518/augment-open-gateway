package handler

import (
	"net/http"
	"strconv"

	"augment-gateway/internal/service"

	"github.com/gin-gonic/gin"
)

// PluginHandler 插件处理器
type PluginHandler struct {
	pluginService *service.PluginService
}

// NewPluginHandler 创建插件处理器
func NewPluginHandler(pluginService *service.PluginService) *PluginHandler {
	return &PluginHandler{
		pluginService: pluginService,
	}
}

// GetList 获取插件列表
// GET /api/v1/user/plugins
func (h *PluginHandler) GetList(c *gin.Context) {
	var req service.PluginListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		ResponseError(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}

	result, err := h.pluginService.GetList(&req)
	if err != nil {
		ResponseError(c, http.StatusInternalServerError, "获取插件列表失败")
		return
	}

	ResponseSuccess(c, result)
}

// Download 获取插件下载信息
// GET /api/v1/user/plugins/:id/download
func (h *PluginHandler) Download(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		ResponseError(c, http.StatusBadRequest, "无效的插件ID")
		return
	}

	downloadURL, err := h.pluginService.GetDownloadURL(uint(id))
	if err != nil {
		ResponseError(c, http.StatusNotFound, "插件不存在")
		return
	}

	if downloadURL == "" {
		ResponseError(c, http.StatusNotFound, "插件下载地址不存在")
		return
	}

	// 返回下载地址
	ResponseSuccess(c, map[string]string{
		"download_url": downloadURL,
	})
}
