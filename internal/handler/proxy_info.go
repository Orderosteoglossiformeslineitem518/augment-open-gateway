package handler

import (
	"augment-gateway/internal/service"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// ProxyInfoHandler 代理信息处理器
type ProxyInfoHandler struct {
	proxyInfoSvc service.ProxyInfoService
	userAuthSvc  *service.UserAuthService
	turnstileSvc service.TurnstileService
}

// NewProxyInfoHandler 创建代理信息处理器
func NewProxyInfoHandler(proxyInfoSvc service.ProxyInfoService, userAuthSvc *service.UserAuthService, turnstileSvc service.TurnstileService) *ProxyInfoHandler {
	return &ProxyInfoHandler{
		proxyInfoSvc: proxyInfoSvc,
		userAuthSvc:  userAuthSvc,
		turnstileSvc: turnstileSvc,
	}
}

// SubmitProxyRequest 提交代理请求结构
type SubmitProxyRequest struct {
	ProxyURLs      []string `json:"proxy_urls" binding:"required,min=1"`
	TurnstileToken string   `json:"turnstile_token" binding:"required"`
}

// UpdateProxyStatusRequest 更新代理状态请求结构
type UpdateProxyStatusRequest struct {
	Status      string `json:"status" binding:"required,oneof=pending valid invalid"`
	Description string `json:"description"`
	ProxyURL    string `json:"proxy_url"`
}

// CreateProxyRequest 创建代理请求结构
type CreateProxyRequest struct {
	ProxyURL    string `json:"proxy_url" binding:"required"`
	Description string `json:"description"`
}

// SubmitProxy 用户提交代理
func (h *ProxyInfoHandler) SubmitProxy(c *gin.Context) {
	var req SubmitProxyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseError(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}

	// 从上下文获取用户ID
	userIDVal, exists := c.Get("user_id")
	if !exists {
		ResponseError(c, http.StatusUnauthorized, "未找到用户信息")
		return
	}

	userID, ok := userIDVal.(uint)
	if !ok {
		ResponseError(c, http.StatusInternalServerError, "用户ID格式错误")
		return
	}

	// 验证Turnstile token
	clientIP := c.ClientIP()
	turnstileResp, err := h.turnstileSvc.VerifyToken(req.TurnstileToken, clientIP)
	if err != nil {
		ResponseError(c, http.StatusInternalServerError, "验证失败，请重试")
		return
	}

	if !turnstileResp.Success {
		ResponseError(c, http.StatusBadRequest, "人机验证失败，请重新验证")
		return
	}

	// 清理和验证代理地址
	var cleanURLs []string
	for _, url := range req.ProxyURLs {
		url = strings.TrimSpace(url)
		if url != "" {
			cleanURLs = append(cleanURLs, url)
		}
	}

	if len(cleanURLs) == 0 {
		ResponseError(c, http.StatusBadRequest, "请至少提供一个有效的代理地址")
		return
	}

	// 提交代理
	if err := h.proxyInfoSvc.SubmitProxy(int(userID), cleanURLs); err != nil {
		ResponseError(c, http.StatusBadRequest, err.Error())
		return
	}

	ResponseSuccessWithMsg(c, "代理信息已提交，请等待管理员审核", nil)
}

// GetUserSubmissions 获取用户的代理提交记录
func (h *ProxyInfoHandler) GetUserSubmissions(c *gin.Context) {
	// 从上下文获取用户ID
	userIDVal, exists := c.Get("user_id")
	if !exists {
		ResponseError(c, http.StatusUnauthorized, "未找到用户信息")
		return
	}

	userID, ok := userIDVal.(uint)
	if !ok {
		ResponseError(c, http.StatusInternalServerError, "用户ID格式错误")
		return
	}

	// 获取分页参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	// 获取用户提交记录
	submissions, total, err := h.proxyInfoSvc.GetUserSubmissions(int(userID), page, pageSize)
	if err != nil {
		ResponseError(c, http.StatusInternalServerError, "获取提交记录失败: "+err.Error())
		return
	}

	ResponseSuccessWithMsg(c, "获取成功", gin.H{
		"list":      submissions,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// CheckSubmissionLimit 检查用户今日提交限制
func (h *ProxyInfoHandler) CheckSubmissionLimit(c *gin.Context) {
	// 从上下文获取用户ID
	userIDVal, exists := c.Get("user_id")
	if !exists {
		ResponseError(c, http.StatusUnauthorized, "未找到用户信息")
		return
	}

	userID, ok := userIDVal.(uint)
	if !ok {
		ResponseError(c, http.StatusInternalServerError, "用户ID格式错误")
		return
	}

	// 检查提交限制
	canSubmit, currentCount, err := h.proxyInfoSvc.CanSubmitToday(int(userID))
	if err != nil {
		ResponseError(c, http.StatusInternalServerError, "检查提交限制失败: "+err.Error())
		return
	}

	ResponseSuccessWithMsg(c, "检查成功", gin.H{
		"can_submit":    canSubmit,
		"current_count": currentCount,
		"max_count":     2,
	})
}

// ListProxies 获取代理列表（管理后台）
func (h *ProxyInfoHandler) ListProxies(c *gin.Context) {
	// 获取查询参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	status := c.Query("status")
	userIDStr := c.Query("user_id")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var userID *int
	if userIDStr != "" {
		if id, err := strconv.Atoi(userIDStr); err == nil {
			userID = &id
		}
	}

	// 获取代理列表
	proxies, total, err := h.proxyInfoSvc.ListProxies(page, pageSize, status, userID)
	if err != nil {
		ResponseError(c, http.StatusInternalServerError, "获取代理列表失败: "+err.Error())
		return
	}

	ResponseSuccessWithMsg(c, "获取成功", gin.H{
		"list":      proxies,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// CreateProxy 创建代理（管理后台）
func (h *ProxyInfoHandler) CreateProxy(c *gin.Context) {
	var req CreateProxyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseError(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}

	// 清理代理地址
	proxyURL := strings.TrimSpace(req.ProxyURL)
	if proxyURL == "" {
		ResponseError(c, http.StatusBadRequest, "代理地址不能为空")
		return
	}

	// 创建代理
	if err := h.proxyInfoSvc.CreateProxy(proxyURL, req.Description, nil); err != nil {
		ResponseError(c, http.StatusBadRequest, err.Error())
		return
	}

	ResponseSuccessWithMsg(c, "代理创建成功", nil)
}

// UpdateProxyStatus 更新代理状态
func (h *ProxyInfoHandler) UpdateProxyStatus(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		ResponseError(c, http.StatusBadRequest, "无效的代理ID")
		return
	}

	var req UpdateProxyStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseError(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}

	// 更新代理信息（包括状态、描述和代理地址）
	if err := h.proxyInfoSvc.UpdateProxy(uint(id), req.Status, req.Description, req.ProxyURL); err != nil {
		ResponseError(c, http.StatusInternalServerError, "更新代理信息失败: "+err.Error())
		return
	}

	ResponseSuccessWithMsg(c, "代理状态更新成功", nil)
}

// ApproveProxy 审核通过代理
func (h *ProxyInfoHandler) ApproveProxy(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		ResponseError(c, http.StatusBadRequest, "无效的代理ID")
		return
	}

	// 审核通过
	if err := h.proxyInfoSvc.ApproveProxy(uint(id)); err != nil {
		ResponseError(c, http.StatusInternalServerError, "审核通过失败: "+err.Error())
		return
	}

	ResponseSuccessWithMsg(c, "代理审核通过", nil)
}

// RejectProxy 审核拒绝代理
func (h *ProxyInfoHandler) RejectProxy(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		ResponseError(c, http.StatusBadRequest, "无效的代理ID")
		return
	}

	type RejectRequest struct {
		Reason string `json:"reason"`
	}

	var req RejectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseError(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}

	// 审核拒绝
	if err := h.proxyInfoSvc.RejectProxy(uint(id), req.Reason); err != nil {
		ResponseError(c, http.StatusInternalServerError, "审核拒绝失败: "+err.Error())
		return
	}

	ResponseSuccessWithMsg(c, "代理审核拒绝", nil)
}

// DeleteProxy 删除代理
func (h *ProxyInfoHandler) DeleteProxy(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		ResponseError(c, http.StatusBadRequest, "无效的代理ID")
		return
	}

	// 删除代理
	if err := h.proxyInfoSvc.DeleteProxy(uint(id)); err != nil {
		ResponseError(c, http.StatusInternalServerError, "删除代理失败: "+err.Error())
		return
	}

	ResponseSuccessWithMsg(c, "代理删除成功", nil)
}
