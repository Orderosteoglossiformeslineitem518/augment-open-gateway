package handler

import (
	"fmt"
	"time"

	"augment-gateway/internal/service"

	"github.com/gin-gonic/gin"
)

// RemoteModelHandler 远程模型处理器
type RemoteModelHandler struct {
	remoteModelService *service.RemoteModelService
}

// NewRemoteModelHandler 创建远程模型处理器
func NewRemoteModelHandler(remoteModelService *service.RemoteModelService) *RemoteModelHandler {
	return &RemoteModelHandler{remoteModelService: remoteModelService}
}

// GetList 获取远程模型列表
func (h *RemoteModelHandler) GetList(c *gin.Context) {
	models, err := h.remoteModelService.GetList()
	if err != nil {
		ResponseError(c, 500, "获取远程模型列表失败: "+err.Error())
		return
	}
	ResponseSuccess(c, models)
}

// SyncModels 手动触发同步远程模型
func (h *RemoteModelHandler) SyncModels(c *gin.Context) {
	newCount, err := h.remoteModelService.SyncFromRemoteAPI()
	if err != nil {
		ResponseError(c, 500, "同步远程模型失败: "+err.Error())
		return
	}
	ResponseSuccessWithMsg(c, "同步成功", gin.H{
		"new_count": newCount,
	})
}

// UpdatePassthroughConfigRequest 更新透传配置请求
type UpdatePassthroughConfigRequest struct {
	AllowSharedTokenPassthrough bool   `json:"allow_shared_token_passthrough"`
	PassthroughExpiresAt        string `json:"passthrough_expires_at"` // ISO 8601 格式，空字符串表示永久
}

// UpdatePassthroughConfig 更新共享账号透传配置
func (h *RemoteModelHandler) UpdatePassthroughConfig(c *gin.Context) {
	// 获取模型ID
	idStr := c.Param("id")
	var id uint
	if _, err := parseUintID(idStr); err != nil {
		ResponseError(c, 400, "无效的模型ID")
		return
	}
	id = parseUintIDValue(idStr)

	// 解析请求体
	var req UpdatePassthroughConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseError(c, 400, "请求参数错误: "+err.Error())
		return
	}

	// 解析截止日期
	var expiresAt *time.Time
	if req.PassthroughExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, req.PassthroughExpiresAt)
		if err != nil {
			// 尝试解析不带时区的格式
			t, err = time.Parse("2006-01-02T15:04:05", req.PassthroughExpiresAt)
			if err != nil {
				// 尝试只有日期的格式
				t, err = time.Parse("2006-01-02", req.PassthroughExpiresAt)
				if err != nil {
					ResponseError(c, 400, "无效的日期格式，请使用 YYYY-MM-DD 或 ISO 8601 格式")
					return
				}
			}
		}
		expiresAt = &t

		// 校验截止日期不能是过去的时间
		if t.Before(time.Now()) {
			ResponseError(c, 400, "截止日期不能是过去的时间，请选择未来的时间")
			return
		}
	}

	// 更新配置
	if err := h.remoteModelService.UpdatePassthroughConfig(id, req.AllowSharedTokenPassthrough, expiresAt); err != nil {
		ResponseError(c, 500, "更新透传配置失败: "+err.Error())
		return
	}

	ResponseSuccessWithMsg(c, "更新成功", nil)
}

// SetDefaultModel 设置默认模型
func (h *RemoteModelHandler) SetDefaultModel(c *gin.Context) {
	idStr := c.Param("id")
	if _, err := parseUintID(idStr); err != nil {
		ResponseError(c, 400, "无效的模型ID")
		return
	}
	id := parseUintIDValue(idStr)

	if err := h.remoteModelService.SetDefaultModel(id); err != nil {
		ResponseError(c, 500, "设置默认模型失败: "+err.Error())
		return
	}

	ResponseSuccessWithMsg(c, "默认模型设置成功", nil)
}

// DeleteModel 删除远程模型
func (h *RemoteModelHandler) DeleteModel(c *gin.Context) {
	idStr := c.Param("id")
	if _, err := parseUintID(idStr); err != nil {
		ResponseError(c, 400, "无效的模型ID")
		return
	}
	id := parseUintIDValue(idStr)

	if err := h.remoteModelService.DeleteModel(id); err != nil {
		ResponseError(c, 500, "删除远程模型失败: "+err.Error())
		return
	}

	ResponseSuccessWithMsg(c, "删除成功", nil)
}

// parseUintID 解析uint ID
func parseUintID(idStr string) (uint, error) {
	var id uint
	for _, c := range idStr {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("无效的ID")
		}
		id = id*10 + uint(c-'0')
	}
	return id, nil
}

// parseUintIDValue 解析uint ID值（假设已经验证过）
func parseUintIDValue(idStr string) uint {
	var id uint
	for _, c := range idStr {
		id = id*10 + uint(c-'0')
	}
	return id
}
