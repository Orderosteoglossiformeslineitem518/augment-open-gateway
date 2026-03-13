package handler

import (
	"strings"

	"augment-gateway/internal/service"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authService *service.AuthService
}

func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
	}
}

// Login 用户登录
func (h *AuthHandler) Login(c *gin.Context) {
	var req service.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseError(c, 400, "请求参数错误")
		return
	}

	resp, err := h.authService.Login(&req)
	if err != nil {
		ResponseError(c, 401, err.Error())
		return
	}

	ResponseSuccessWithMsg(c, "登录成功", resp)
}

// Logout 用户登出
func (h *AuthHandler) Logout(c *gin.Context) {
	// JWT是无状态的，前端只需删除token即可
	ResponseSuccessWithMsg(c, "登出成功", nil)
}

// Me 获取当前用户信息
func (h *AuthHandler) Me(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		ResponseError(c, 401, "未授权")
		return
	}

	user, err := h.authService.GetUserByID(userID.(uint))
	if err != nil {
		ResponseError(c, 500, "获取用户信息失败")
		return
	}

	ResponseSuccess(c, user)
}

// UpdateProfile 更新用户信息
func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		ResponseError(c, 401, "未授权")
		return
	}

	var req service.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseError(c, 400, "请求参数错误")
		return
	}

	// 验证请求参数
	if req.Username == "" && req.NewPassword == "" {
		ResponseError(c, 400, "至少需要提供用户名或新密码")
		return
	}

	if req.NewPassword != "" && len(req.NewPassword) < 6 {
		ResponseError(c, 400, "新密码长度不能少于6位")
		return
	}

	user, err := h.authService.UpdateProfile(userID.(uint), &req)
	if err != nil {
		ResponseError(c, 400, err.Error())
		return
	}

	ResponseSuccessWithMsg(c, "用户信息更新成功", user)
}

// AuthMiddleware 认证中间件
func (h *AuthHandler) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 跳过登录接口
		if c.Request.URL.Path == "/api/v1/auth/login" {
			c.Next()
			return
		}

		// 获取Authorization头
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			ResponseError(c, 401, "缺少Authorization头")
			c.Abort()
			return
		}

		// 检查Bearer前缀
		tokenParts := strings.SplitN(authHeader, " ", 2)
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			ResponseError(c, 401, "Authorization头格式错误")
			c.Abort()
			return
		}

		token := tokenParts[1]

		// 验证token
		claims, err := h.authService.ValidateToken(token)
		if err != nil {
			ResponseError(c, 401, "无效的token")
			c.Abort()
			return
		}

		// 将用户信息存储到上下文中
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)

		c.Next()
	}
}
