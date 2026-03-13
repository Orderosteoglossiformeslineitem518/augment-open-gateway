package handler

import (
	"fmt"
	"net/http"
	"strings"

	"augment-gateway/internal/service"

	"github.com/gin-gonic/gin"
)

// UserAuthHandler 用户认证处理器
type UserAuthHandler struct {
	userAuthService  *service.UserAuthService
	turnstileService service.TurnstileService
}

// NewUserAuthHandler 创建用户认证处理器
func NewUserAuthHandler(userAuthService *service.UserAuthService, turnstileService service.TurnstileService) *UserAuthHandler {
	return &UserAuthHandler{
		userAuthService:  userAuthService,
		turnstileService: turnstileService,
	}
}

// RegisterRequest 用户注册请求（包含Turnstile验证）
type RegisterRequest struct {
	Username       string `json:"username" binding:"required"`
	Email          string `json:"email" binding:"required,email"`
	Password       string `json:"password" binding:"required"`
	InvitationCode string `json:"invitation_code" binding:"required"`
	TurnstileToken string `json:"turnstile_token" binding:"required"` // Turnstile验证令牌
}

// Register 用户注册
// POST /api/v1/user/register
func (h *UserAuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseError(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	// 验证Turnstile令牌
	remoteIP := c.ClientIP()
	turnstileResp, err := h.turnstileService.VerifyToken(req.TurnstileToken, remoteIP)
	if err != nil {
		ResponseError(c, http.StatusBadRequest, "人机验证失败: "+err.Error())
		return
	}
	if !turnstileResp.Success {
		ResponseError(c, http.StatusBadRequest, "人机验证未通过，请重试")
		return
	}

	// 转换为service层的请求结构
	serviceReq := &service.UserRegisterRequest{
		Username:       req.Username,
		Email:          req.Email,
		Password:       req.Password,
		InvitationCode: req.InvitationCode,
	}

	result, err := h.userAuthService.Register(c.Request.Context(), serviceReq)
	if err != nil {
		ResponseError(c, http.StatusBadRequest, err.Error())
		return
	}

	ResponseSuccessWithMsg(c, "注册成功", result)
}

// Login 用户登录
// POST /api/v1/user/login
func (h *UserAuthHandler) Login(c *gin.Context) {
	var req service.UserLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseError(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	// 验证Turnstile令牌
	remoteIP := c.ClientIP()
	turnstileResp, err := h.turnstileService.VerifyToken(req.TurnstileToken, remoteIP)
	if err != nil {
		ResponseError(c, http.StatusBadRequest, "人机验证失败: "+err.Error())
		return
	}
	if !turnstileResp.Success {
		ResponseError(c, http.StatusBadRequest, "人机验证未通过，请重试")
		return
	}

	result, err := h.userAuthService.Login(c.Request.Context(), &req)
	if err != nil {
		ResponseError(c, http.StatusBadRequest, err.Error())
		return
	}

	ResponseSuccessWithMsg(c, "登录成功", result)
}

// Refresh 刷新令牌
// POST /api/v1/user/refresh
func (h *UserAuthHandler) Refresh(c *gin.Context) {
	var req service.RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseError(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	result, err := h.userAuthService.RefreshToken(c.Request.Context(), req.RefreshToken)
	if err != nil {
		ResponseError(c, http.StatusUnauthorized, err.Error())
		return
	}

	ResponseSuccessWithMsg(c, "刷新成功", result)
}

// Logout 用户登出
// POST /api/v1/user/logout
func (h *UserAuthHandler) Logout(c *gin.Context) {
	// JWT是无状态的，登出只需要客户端删除token即可
	// 如果需要服务端黑名单，可以在这里实现
	ResponseSuccessWithMsg(c, "登出成功", nil)
}

// Me 获取当前用户信息
// GET /api/v1/user/me
func (h *UserAuthHandler) Me(c *gin.Context) {
	// 从上下文获取用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		ResponseError(c, http.StatusUnauthorized, "未授权访问")
		return
	}

	user, err := h.userAuthService.GetUserByID(c.Request.Context(), userID.(uint))
	if err != nil {
		ResponseError(c, http.StatusNotFound, err.Error())
		return
	}

	ResponseSuccess(c, user)
}

// UpdateProfile 更新用户信息
// PUT /api/v1/user/profile
func (h *UserAuthHandler) UpdateProfile(c *gin.Context) {
	// 从上下文获取用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		ResponseError(c, http.StatusUnauthorized, "未授权访问")
		return
	}

	var req service.UserUpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseError(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	user, err := h.userAuthService.UpdateProfile(c.Request.Context(), userID.(uint), &req)
	if err != nil {
		ResponseError(c, http.StatusBadRequest, err.Error())
		return
	}

	ResponseSuccessWithMsg(c, "更新成功", user)
}

// ChangePassword 修改密码
// PUT /api/v1/user/password
func (h *UserAuthHandler) ChangePassword(c *gin.Context) {
	// 从上下文获取用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		ResponseError(c, http.StatusUnauthorized, "未授权访问")
		return
	}

	var req service.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseError(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	err := h.userAuthService.ChangePassword(c.Request.Context(), userID.(uint), &req)
	if err != nil {
		ResponseError(c, http.StatusBadRequest, err.Error())
		return
	}

	ResponseSuccessWithMsg(c, "密码修改成功", nil)
}

// RegenerateAPIToken 重新生成API令牌
// POST /api/v1/user/regenerate-token
func (h *UserAuthHandler) RegenerateAPIToken(c *gin.Context) {
	// 从上下文获取用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		ResponseError(c, http.StatusUnauthorized, "未授权访问")
		return
	}

	user, err := h.userAuthService.RegenerateAPIToken(c.Request.Context(), userID.(uint))
	if err != nil {
		ResponseError(c, http.StatusBadRequest, err.Error())
		return
	}

	ResponseSuccessWithMsg(c, "API令牌已重新生成", user)
}

// GetUserSettings 获取用户设置
// GET /api/v1/user/settings
func (h *UserAuthHandler) GetUserSettings(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		ResponseError(c, http.StatusUnauthorized, "未授权访问")
		return
	}

	settings, err := h.userAuthService.GetUserSettings(c.Request.Context(), userID.(uint))
	if err != nil {
		ResponseError(c, http.StatusInternalServerError, err.Error())
		return
	}

	ResponseSuccess(c, settings)
}

// UpdateUserSettings 更新用户设置
// PUT /api/v1/user/settings
func (h *UserAuthHandler) UpdateUserSettings(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		ResponseError(c, http.StatusUnauthorized, "未授权访问")
		return
	}

	var req service.UpdateUserSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseError(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	err := h.userAuthService.UpdateUserSettings(c.Request.Context(), userID.(uint), &req)
	if err != nil {
		ResponseError(c, http.StatusBadRequest, err.Error())
		return
	}

	ResponseSuccessWithMsg(c, "设置更新成功", nil)
}

// UserAuthMiddleware 用户身份验证中间件
func (h *UserAuthHandler) UserAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从请求头获取Authorization
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			ResponseError(c, http.StatusUnauthorized, "未提供认证令牌")
			c.Abort()
			return
		}

		// 解析Bearer Token
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			ResponseError(c, http.StatusUnauthorized, "认证令牌格式错误")
			c.Abort()
			return
		}

		// 验证Token
		claims, err := h.userAuthService.ValidateToken(tokenString)
		if err != nil {
			ResponseError(c, http.StatusUnauthorized, "认证令牌无效或已过期")
			c.Abort()
			return
		}

		// 获取用户信息
		user, err := h.userAuthService.GetUserByID(c.Request.Context(), claims.UserID)
		if err != nil {
			ResponseError(c, http.StatusUnauthorized, "用户不存在")
			c.Abort()
			return
		}

		// 检查用户状态
		if !user.IsActive() {
			ResponseError(c, http.StatusForbidden, "账号已被禁用")
			c.Abort()
			return
		}

		// 将用户信息存储到上下文中
		c.Set("user_id", user.ID)
		c.Set("username", user.Username)
		c.Set("role", user.Role)
		c.Set("user", user)
		c.Set("api_token", user.ApiToken)

		c.Next()
	}
}

// APITokenMiddleware API令牌验证中间件（用于代理请求）
func (h *UserAuthHandler) APITokenMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从请求头获取Authorization
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			ResponseError(c, http.StatusUnauthorized, "未提供API令牌")
			c.Abort()
			return
		}

		// 解析Bearer Token
		apiToken := strings.TrimPrefix(authHeader, "Bearer ")
		if apiToken == authHeader {
			ResponseError(c, http.StatusUnauthorized, "API令牌格式错误")
			c.Abort()
			return
		}

		// 验证API令牌
		user, err := h.userAuthService.GetUserByAPIToken(c.Request.Context(), apiToken)
		if err != nil {
			ResponseError(c, http.StatusUnauthorized, "API令牌无效")
			c.Abort()
			return
		}

		// 检查用户状态
		if !user.IsActive() {
			ResponseError(c, http.StatusForbidden, "账号已被禁用")
			c.Abort()
			return
		}

		// 检查令牌状态
		if !user.IsTokenActive() {
			ResponseError(c, http.StatusForbidden, "API令牌已被禁用")
			c.Abort()
			return
		}

		// 检查是否可以发起请求
		if !user.CanMakeRequest() {
			ResponseError(c, http.StatusForbidden, "API配额已用完或无额度")
			c.Abort()
			return
		}

		// 将用户信息存储到上下文中
		c.Set("user_id", user.ID)
		c.Set("username", user.Username)
		c.Set("role", user.Role)
		c.Set("user", user)
		c.Set("api_token", apiToken)

		c.Next()
	}
}

// ==================== 用户管理接口（管理员） ====================

// ListUsers 获取用户列表
func (h *UserAuthHandler) ListUsers(c *gin.Context) {
	var req service.UserListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		ResponseError(c, http.StatusBadRequest, "参数错误")
		return
	}

	result, err := h.userAuthService.ListUsers(&req)
	if err != nil {
		ResponseError(c, http.StatusInternalServerError, "获取用户列表失败")
		return
	}

	ResponseSuccess(c, result)
}

// BanUser 封禁用户
func (h *UserAuthHandler) BanUser(c *gin.Context) {
	userIDStr := c.Param("id")
	var userID uint
	if _, err := fmt.Sscanf(userIDStr, "%d", &userID); err != nil {
		ResponseError(c, http.StatusBadRequest, "用户ID格式错误")
		return
	}

	if err := h.userAuthService.BanUser(userID); err != nil {
		ResponseError(c, http.StatusInternalServerError, err.Error())
		return
	}

	ResponseSuccess(c, nil)
}

// UnbanUser 解封用户
func (h *UserAuthHandler) UnbanUser(c *gin.Context) {
	userIDStr := c.Param("id")
	var userID uint
	if _, err := fmt.Sscanf(userIDStr, "%d", &userID); err != nil {
		ResponseError(c, http.StatusBadRequest, "用户ID格式错误")
		return
	}

	if err := h.userAuthService.UnbanUser(userID); err != nil {
		ResponseError(c, http.StatusInternalServerError, err.Error())
		return
	}

	ResponseSuccess(c, nil)
}

// ToggleSharedPermission 切换用户共享权限
func (h *UserAuthHandler) ToggleSharedPermission(c *gin.Context) {
	userIDStr := c.Param("id")
	var userID uint
	if _, err := fmt.Sscanf(userIDStr, "%d", &userID); err != nil {
		ResponseError(c, http.StatusBadRequest, "用户ID格式错误")
		return
	}

	var req struct {
		CanUseShared bool `json:"can_use_shared"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseError(c, http.StatusBadRequest, "参数错误")
		return
	}

	if err := h.userAuthService.ToggleSharedPermission(userID, req.CanUseShared); err != nil {
		ResponseError(c, http.StatusInternalServerError, err.Error())
		return
	}

	ResponseSuccess(c, nil)
}

// AdminUpdateUser 管理员更新用户信息
func (h *UserAuthHandler) AdminUpdateUser(c *gin.Context) {
	userIDStr := c.Param("id")
	var userID uint
	if _, err := fmt.Sscanf(userIDStr, "%d", &userID); err != nil {
		ResponseError(c, http.StatusBadRequest, "用户ID格式错误")
		return
	}

	var req service.AdminUpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseError(c, http.StatusBadRequest, "参数错误")
		return
	}

	user, err := h.userAuthService.AdminUpdateUser(userID, &req)
	if err != nil {
		ResponseError(c, http.StatusInternalServerError, err.Error())
		return
	}

	ResponseSuccess(c, user)
}
