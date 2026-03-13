package handler

import (
	"fmt"
	"net/http"

	"augment-gateway/internal/service"

	"github.com/gin-gonic/gin"
)

// InvitationCodeHandler 邀请码处理器
type InvitationCodeHandler struct {
	invitationCodeService *service.InvitationCodeService
}

// NewInvitationCodeHandler 创建邀请码处理器
func NewInvitationCodeHandler(invitationCodeService *service.InvitationCodeService) *InvitationCodeHandler {
	return &InvitationCodeHandler{
		invitationCodeService: invitationCodeService,
	}
}

// Generate 生成邀请码
// POST /api/v1/invitation-codes/generate
func (h *InvitationCodeHandler) Generate(c *gin.Context) {
	// 检查管理员权限
	role, exists := c.Get("role")
	if !exists || role != "admin" {
		ResponseError(c, http.StatusForbidden, "无权限访问")
		return
	}

	var req service.GenerateCodesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseError(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	// 获取当前用户信息
	userID, _ := c.Get("user_id")
	username, _ := c.Get("username")

	req.CreatorID = userID.(uint)
	req.CreatorName = username.(string)

	result, err := h.invitationCodeService.GenerateCodes(&req)
	if err != nil {
		ResponseError(c, http.StatusInternalServerError, err.Error())
		return
	}

	ResponseSuccessWithMsg(c, fmt.Sprintf("成功生成%d个邀请码", result.Count), result)
}

// List 获取邀请码列表
// GET /api/v1/invitation-codes
func (h *InvitationCodeHandler) List(c *gin.Context) {
	// 检查管理员权限
	role, exists := c.Get("role")
	if !exists || role != "admin" {
		ResponseError(c, http.StatusForbidden, "无权限访问")
		return
	}

	var req service.InvitationCodeListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		ResponseError(c, http.StatusBadRequest, "参数错误")
		return
	}

	result, err := h.invitationCodeService.ListCodes(&req)
	if err != nil {
		ResponseError(c, http.StatusInternalServerError, "获取邀请码列表失败")
		return
	}

	ResponseSuccess(c, result)
}

// Delete 删除单个邀请码
// DELETE /api/v1/invitation-codes/:id
func (h *InvitationCodeHandler) Delete(c *gin.Context) {
	// 检查管理员权限
	role, exists := c.Get("role")
	if !exists || role != "admin" {
		ResponseError(c, http.StatusForbidden, "无权限访问")
		return
	}

	idStr := c.Param("id")
	var id uint
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		ResponseError(c, http.StatusBadRequest, "ID格式错误")
		return
	}

	if err := h.invitationCodeService.DeleteCode(id); err != nil {
		ResponseError(c, http.StatusInternalServerError, err.Error())
		return
	}

	ResponseSuccessWithMsg(c, "删除成功", nil)
}

// Validate 验证邀请码（公开接口，用于注册时验证）
// GET /api/v1/invitation-codes/validate
func (h *InvitationCodeHandler) Validate(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		ResponseError(c, http.StatusBadRequest, "邀请码不能为空")
		return
	}

	_, err := h.invitationCodeService.ValidateCode(code)
	if err != nil {
		ResponseError(c, http.StatusBadRequest, err.Error())
		return
	}

	ResponseSuccessWithMsg(c, "邀请码有效", gin.H{"valid": true})
}
