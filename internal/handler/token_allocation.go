package handler

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"augment-gateway/internal/logger"
	"augment-gateway/internal/service"

	"github.com/gin-gonic/gin"
)

// TokenAllocationHandler TOKEN分配处理器
type TokenAllocationHandler struct {
	tokenAllocationService *service.TokenAllocationService
	userUsageStatsService  *service.UserUsageStatsService
	cacheService           *service.CacheService
	turnstileService       service.TurnstileService
	tokenService           *service.TokenService
	authSessionClient      service.AuthSessionClient
	proxyInfoService       service.ProxyInfoService
}

// NewTokenAllocationHandler 创建TOKEN分配处理器
func NewTokenAllocationHandler(
	tokenAllocationService *service.TokenAllocationService,
	userUsageStatsService *service.UserUsageStatsService,
	cacheService *service.CacheService,
	turnstileService service.TurnstileService,
	tokenService *service.TokenService,
	authSessionClient service.AuthSessionClient,
	proxyInfoService service.ProxyInfoService,
) *TokenAllocationHandler {
	return &TokenAllocationHandler{
		tokenAllocationService: tokenAllocationService,
		userUsageStatsService:  userUsageStatsService,
		cacheService:           cacheService,
		turnstileService:       turnstileService,
		tokenService:           tokenService,
		authSessionClient:      authSessionClient,
		proxyInfoService:       proxyInfoService,
	}
}

// GetUserAllocations 获取用户的TOKEN分配列表
// GET /api/v1/user/token-allocations
func (h *TokenAllocationHandler) GetUserAllocations(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		ResponseError(c, http.StatusUnauthorized, "请先登录")
		return
	}

	// 获取用户API令牌
	apiToken, _ := c.Get("api_token")
	apiTokenStr := ""
	if apiToken != nil {
		apiTokenStr = apiToken.(string)
	}

	// 获取当前使用的TokenID
	currentUsingTokenID := ""
	if apiTokenStr != "" && h.cacheService != nil {
		cacheKey := fmt.Sprintf("AUGMENT-GATEWAY:user_token_assignment:%s", apiTokenStr)
		if tokenID, err := h.cacheService.GetString(context.Background(), cacheKey); err == nil {
			currentUsingTokenID = tokenID
		}
	}

	var req service.UserTokenAllocationListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		ResponseError(c, http.StatusBadRequest, "参数错误")
		return
	}

	result, err := h.tokenAllocationService.GetUserAllocations(userID.(uint), &req, currentUsingTokenID)
	if err != nil {
		ResponseError(c, http.StatusInternalServerError, "获取分配列表失败")
		return
	}

	ResponseSuccess(c, result)
}

// GetUserUsageStats 获取用户使用统计（按日）
// GET /api/v1/user/usage-stats
func (h *TokenAllocationHandler) GetUserUsageStats(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		ResponseError(c, http.StatusUnauthorized, "请先登录")
		return
	}

	var req service.UserDailyStatsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		ResponseError(c, http.StatusBadRequest, "参数错误")
		return
	}

	result, err := h.userUsageStatsService.GetUserDailyStats(userID.(uint), &req)
	if err != nil {
		ResponseError(c, http.StatusInternalServerError, "获取统计数据失败: "+err.Error())
		return
	}

	ResponseSuccess(c, result)
}

// GetUserStatsOverview 获取用户统计概览
// GET /api/v1/user/usage-stats/overview
func (h *TokenAllocationHandler) GetUserStatsOverview(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		ResponseError(c, http.StatusUnauthorized, "请先登录")
		return
	}

	result, err := h.userUsageStatsService.GetUserStatsOverview(userID.(uint))
	if err != nil {
		ResponseError(c, http.StatusInternalServerError, "获取统计概览失败")
		return
	}

	ResponseSuccess(c, result)
}

// GetUserTokenAccountStats 获取用户TOKEN账号统计
// GET /api/v1/user/token-account-stats
func (h *TokenAllocationHandler) GetUserTokenAccountStats(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		ResponseError(c, http.StatusUnauthorized, "请先登录")
		return
	}

	result, err := h.tokenAllocationService.GetUserTokenAccountStats(userID.(uint))
	if err != nil {
		ResponseError(c, http.StatusInternalServerError, "获取TOKEN账号统计失败: "+err.Error())
		return
	}

	ResponseSuccess(c, result)
}

// UserDisableToken 用户禁用自己分配的TOKEN账号
// POST /api/v1/user/tokens/:token_id/disable
func (h *TokenAllocationHandler) UserDisableToken(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		ResponseError(c, http.StatusUnauthorized, "请先登录")
		return
	}

	tokenID := c.Param("token_id")
	if tokenID == "" {
		ResponseError(c, http.StatusBadRequest, "TOKEN ID不能为空")
		return
	}

	// 获取用户API令牌
	apiToken, _ := c.Get("api_token")
	apiTokenStr := ""
	if apiToken != nil {
		apiTokenStr = apiToken.(string)
	}

	// 禁用TOKEN
	if err := h.tokenAllocationService.DisableUserToken(userID.(uint), tokenID); err != nil {
		ResponseError(c, http.StatusBadRequest, err.Error())
		return
	}

	// 清理缓存：如果当前用户正在使用该TOKEN，清除分配缓存
	if h.cacheService != nil && apiTokenStr != "" {
		ctx := context.Background()
		cacheKey := fmt.Sprintf("AUGMENT-GATEWAY:user_token_assignment:%s", apiTokenStr)
		cachedTokenID, err := h.cacheService.GetString(ctx, cacheKey)
		if err == nil && cachedTokenID == tokenID {
			// 清除用户的TOKEN分配缓存
			h.cacheService.DeleteKey(ctx, cacheKey)
		}
	}

	ResponseSuccessWithMsg(c, "账号已禁用", nil)
}

// UserDeleteToken 用户删除自己提交的TOKEN账号（仅自有账号可删除）
// DELETE /api/v1/user/tokens/:token_id
func (h *TokenAllocationHandler) UserDeleteToken(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		ResponseError(c, http.StatusUnauthorized, "请先登录")
		return
	}

	tokenID := c.Param("token_id")
	if tokenID == "" {
		ResponseError(c, http.StatusBadRequest, "TOKEN ID不能为空")
		return
	}

	// 获取用户API令牌
	apiToken, _ := c.Get("api_token")
	apiTokenStr := ""
	if apiToken != nil {
		apiTokenStr = apiToken.(string)
	}

	// 删除TOKEN（仅自有账号可删除）
	if err := h.tokenAllocationService.DeleteUserToken(userID.(uint), tokenID); err != nil {
		ResponseError(c, http.StatusBadRequest, err.Error())
		return
	}

	// 清理缓存：如果当前用户正在使用该TOKEN，清除分配缓存
	if h.cacheService != nil && apiTokenStr != "" {
		ctx := context.Background()
		cacheKey := fmt.Sprintf("AUGMENT-GATEWAY:user_token_assignment:%s", apiTokenStr)
		cachedTokenID, err := h.cacheService.GetString(ctx, cacheKey)
		if err == nil && cachedTokenID == tokenID {
			// 清除用户的TOKEN分配缓存
			h.cacheService.DeleteKey(ctx, cacheKey)
		}
	}

	ResponseSuccessWithMsg(c, "账号已删除", nil)
}

// UserSwitchToken 用户切换当前使用的TOKEN账号
// POST /api/v1/user/tokens/:token_id/switch
func (h *TokenAllocationHandler) UserSwitchToken(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		ResponseError(c, http.StatusUnauthorized, "请先登录")
		return
	}

	// 获取用户API令牌
	apiToken, _ := c.Get("api_token")
	apiTokenStr := ""
	if apiToken != nil {
		apiTokenStr = apiToken.(string)
	}
	if apiTokenStr == "" {
		ResponseError(c, http.StatusBadRequest, "无法获取用户令牌信息")
		return
	}

	tokenID := c.Param("token_id")
	if tokenID == "" {
		ResponseError(c, http.StatusBadRequest, "TOKEN ID不能为空")
		return
	}

	// 验证用户是否拥有该TOKEN
	token, err := h.tokenAllocationService.ValidateUserTokenOwnership(userID.(uint), tokenID)
	if err != nil {
		ResponseError(c, http.StatusBadRequest, err.Error())
		return
	}

	// 更新Redis缓存，将用户的当前TOKEN切换为新的TOKEN
	if h.cacheService != nil {
		ctx := context.Background()
		cacheKey := fmt.Sprintf("AUGMENT-GATEWAY:user_token_assignment:%s", apiTokenStr)

		// 获取旧的TOKEN ID
		oldTokenID, _ := h.cacheService.GetString(ctx, cacheKey)

		// 如果切换到同一个TOKEN，直接返回
		if oldTokenID == tokenID {
			ResponseSuccessWithMsg(c, "当前已在使用该账号", gin.H{
				"token_id": token.ID,
				"email":    token.Email,
			})
			return
		}

		// 设置新的TOKEN分配缓存，24小时过期
		if err := h.cacheService.SetString(ctx, cacheKey, tokenID, 24*time.Hour); err != nil {
			ResponseError(c, http.StatusInternalServerError, "切换账号失败")
			return
		}
	}

	ResponseSuccessWithMsg(c, "账号切换成功", gin.H{
		"token_id": token.ID,
		"email":    token.Email,
	})
}

// GetUserAvailableTokensForSwitch 获取用户可切换的TOKEN列表
// GET /api/v1/user/tokens/available-for-switch
func (h *TokenAllocationHandler) GetUserAvailableTokensForSwitch(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		ResponseError(c, http.StatusUnauthorized, "请先登录")
		return
	}

	// 获取当前使用的TokenID
	currentUsingTokenID := ""
	apiToken, _ := c.Get("api_token")
	if apiToken != nil {
		apiTokenStr := apiToken.(string)
		if apiTokenStr != "" && h.cacheService != nil {
			cacheKey := fmt.Sprintf("AUGMENT-GATEWAY:user_token_assignment:%s", apiTokenStr)
			if tokenID, err := h.cacheService.GetString(context.Background(), cacheKey); err == nil {
				currentUsingTokenID = tokenID
			}
		}
	}

	tokens, err := h.tokenAllocationService.GetUserAvailableTokensForSwitch(userID.(uint), currentUsingTokenID)
	if err != nil {
		ResponseError(c, http.StatusInternalServerError, "获取可切换账号列表失败")
		return
	}

	ResponseSuccess(c, gin.H{
		"list":                   tokens,
		"current_using_token_id": currentUsingTokenID,
	})
}

// UserSubmitTokenRequest 用户提交TOKEN请求
type UserSubmitTokenRequest struct {
	AuthSession    string `json:"auth_session"`                       // AuthSession（可选，二选一）
	Token          string `json:"token"`                              // TOKEN（可选，二选一）
	TenantAddress  string `json:"tenant_address"`                     // 租户地址（可选，二选一）
	PortalURL      string `json:"portal_url"`                         // Portal URL（订阅地址，可选）
	ProxyAddress   string `json:"proxy_address"`                      // 代理地址（选填）
	AccountType    string `json:"account_type"`                       // 账号类型：30000_credits, 24000_credits
	TurnstileToken string `json:"turnstile_token" binding:"required"` // Turnstile验证令牌
}

// UserSubmitToken 用户提交自己的Augment TOKEN账号
// POST /api/v1/user/tokens/submit
func (h *TokenAllocationHandler) UserSubmitToken(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		ResponseError(c, http.StatusUnauthorized, "请先登录")
		return
	}

	username, _ := c.Get("username")
	usernameStr := ""
	if username != nil {
		usernameStr = username.(string)
	}

	var req UserSubmitTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseError(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
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

	// 验证必填字段：AuthSession 或 (TOKEN + 租户地址) 必须填写其一
	hasAuthSession := strings.TrimSpace(req.AuthSession) != ""
	hasToken := strings.TrimSpace(req.Token) != ""
	hasTenantAddress := strings.TrimSpace(req.TenantAddress) != ""

	if !hasAuthSession && (!hasToken || !hasTenantAddress) {
		ResponseError(c, http.StatusBadRequest, "请填写AuthSession或(TOKEN+租户地址)")
		return
	}

	// 处理AuthSession（如果提供）
	tokenValue := strings.TrimSpace(req.Token)
	tenantAddress := strings.TrimSpace(req.TenantAddress)
	portalURL := strings.TrimSpace(req.PortalURL)
	authSession := strings.TrimSpace(req.AuthSession) // 在外层声明，方便后续保存
	emailFromAuth := ""                               // 从AuthSession获取的邮箱

	if hasAuthSession {
		// 验证AuthSession格式
		if !strings.HasPrefix(authSession, ".eJ") && !strings.HasPrefix(authSession, ".ey") {
			ResponseError(c, http.StatusBadRequest, "AuthSession格式错误")
			return
		}

		// 验证Auth Session是否有效
		if err := h.authSessionClient.ValidateAuthSession(authSession); err != nil {
			logger.Infof("[用户提交TOKEN] AuthSession验证失败: %v\n", err)
			ResponseError(c, http.StatusBadRequest, "添加失败，Session无效或已过期")
			return
		}

		// 通过Auth Session获取Tenant URL、Token、Email和新的AuthSession
		tenantURL, accessToken, email, newAuthSession, err := h.authSessionClient.AuthDevice(authSession)
		if err != nil {
			logger.Infof("[用户提交TOKEN] 获取认证信息失败: %v\n", err)
			ResponseError(c, http.StatusBadRequest, "添加失败，Session无效或已过期")
			return
		}

		tokenValue = accessToken
		tenantAddress = strings.TrimSuffix(tenantURL, "/") + "/"
		emailFromAuth = email
		// 更新authSession为刷新后的值，确保保存最新的session
		if newAuthSession != "" && newAuthSession != authSession {
			authSession = newAuthSession
			logger.Infof("[用户提交TOKEN] AuthSession已刷新, new_session_length: %d", len(newAuthSession))
		}

		// 通过Auth Session获取App Cookie Session，用于获取订阅信息
		appCookieSession, err := h.authSessionClient.AuthAppLogin(authSession)
		if err != nil {
			logger.Infof("[用户提交TOKEN] 获取App Session失败: %v，继续处理\n", err)
		} else {
			// 获取订阅信息，提取Portal URL
			subscriptionInfo, err := h.authSessionClient.GetSubscriptionInfo(appCookieSession)
			if err != nil {
				logger.Infof("[用户提交TOKEN] 获取订阅信息失败: %v，继续处理\n", err)
			} else {
				// 从订阅信息中提取Portal URL
				if portalURLInterface, ok := subscriptionInfo["portalUrl"]; ok {
					if portalURLStr, ok := portalURLInterface.(string); ok && portalURLStr != "" {
						portalURL = portalURLStr
						logger.Infof("[用户提交TOKEN] 成功获取Portal URL: %s", portalURL)
					}
				}
			}
		}
	}

	// 验证TOKEN长度
	if len(tokenValue) != 64 {
		ResponseError(c, http.StatusBadRequest, "TOKEN必须为64位字符串")
		return
	}

	// 验证租户地址格式
	if !strings.HasPrefix(tenantAddress, "https://") {
		ResponseError(c, http.StatusBadRequest, "租户地址必须以https://开头")
		return
	}
	if !strings.HasSuffix(tenantAddress, "/") {
		tenantAddress = tenantAddress + "/"
	}

	// 处理代理地址（选填）
	proxyAddress := strings.TrimSpace(req.ProxyAddress)
	var finalTenantAddress string
	if proxyAddress != "" {
		// 有代理地址时进行格式验证和替换
		if !strings.HasPrefix(proxyAddress, "https://") {
			ResponseError(c, http.StatusBadRequest, "代理地址必须以https://开头")
			return
		}
		if !strings.HasSuffix(proxyAddress, "/") {
			proxyAddress = proxyAddress + "/"
		}
		// 替换租户地址为代理地址
		finalTenantAddress = h.replaceTenantAddressWithProxy(tenantAddress, proxyAddress)
	} else {
		// 无代理地址时直接使用租户地址
		finalTenantAddress = tenantAddress
	}

	// 设置账号额度
	maxRequests := 30000
	if req.AccountType == "24000_credits" {
		maxRequests = 24000
	} else if req.AccountType == "34000_credits" {
		maxRequests = 34000
	} else if req.AccountType == "4000_credits" {
		maxRequests = 4000
	} else if req.AccountType == "0_credits" {
		maxRequests = 0
	}

	// 创建TOKEN（使用刷新后的AuthSession）
	createReq := &service.CreateUserSubmittedTokenRequest{
		Token:             tokenValue,
		TenantAddress:     finalTenantAddress,
		PortalURL:         portalURL,
		Email:             emailFromAuth, // 从AuthSession获取的邮箱
		AuthSession:       authSession,   // 已在上面更新为刷新后的值
		MaxRequests:       maxRequests,
		ExpiresAt:         nil,   // 永不过期
		IsShared:          false, // 用户自己提交的不共享
		SubmitterUserID:   userID.(uint),
		SubmitterUsername: usernameStr,
	}

	tokenID, err := h.tokenService.CreateUserSubmittedToken(createReq)
	if err != nil {
		ResponseError(c, http.StatusBadRequest, err.Error())
		return
	}

	// 提交账号成功后，清除用户的"无TOKEN"状态缓存，避免之前缓存的无账号状态阻止用户使用新账号
	apiToken, _ := c.Get("api_token")
	if apiToken != nil {
		if apiTokenStr, ok := apiToken.(string); ok && apiTokenStr != "" {
			if err := h.tokenService.ClearUserNoTokenCache(c.Request.Context(), apiTokenStr); err != nil {
				logger.Warnf("[用户提交TOKEN] 清除无TOKEN状态缓存失败: %v", err)
			}
		}
	}
	ResponseSuccessWithMsg(c, "账号添加成功", gin.H{
		"token_id": tokenID,
	})
}

// EnhanceTokenRequest 增强TOKEN请求
type EnhanceTokenRequest struct {
	ChannelID uint `json:"channel_id" binding:"required"`
}

// UserEnhanceToken 用户增强TOKEN（绑定外部渠道）
// POST /api/v1/user/tokens/:token_id/enhance
func (h *TokenAllocationHandler) UserEnhanceToken(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		ResponseError(c, http.StatusUnauthorized, "请先登录")
		return
	}

	tokenID := c.Param("token_id")
	if tokenID == "" {
		ResponseError(c, http.StatusBadRequest, "TOKEN ID不能为空")
		return
	}

	var req EnhanceTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseError(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}

	if err := h.tokenAllocationService.EnhanceToken(userID.(uint), tokenID, req.ChannelID); err != nil {
		ResponseError(c, http.StatusBadRequest, err.Error())
		return
	}

	ResponseSuccessWithMsg(c, "增强绑定成功", nil)
}

// UserRemoveTokenEnhance 用户解除TOKEN增强绑定
// DELETE /api/v1/user/tokens/:token_id/enhance
func (h *TokenAllocationHandler) UserRemoveTokenEnhance(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		ResponseError(c, http.StatusUnauthorized, "请先登录")
		return
	}

	tokenID := c.Param("token_id")
	if tokenID == "" {
		ResponseError(c, http.StatusBadRequest, "TOKEN ID不能为空")
		return
	}

	if err := h.tokenAllocationService.RemoveTokenEnhance(userID.(uint), tokenID); err != nil {
		ResponseError(c, http.StatusBadRequest, err.Error())
		return
	}

	ResponseSuccessWithMsg(c, "已解除增强绑定", nil)
}

// GetTokenEnhanceInfo 获取TOKEN的增强绑定信息
// GET /api/v1/user/tokens/:token_id/enhance
func (h *TokenAllocationHandler) GetTokenEnhanceInfo(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		ResponseError(c, http.StatusUnauthorized, "请先登录")
		return
	}

	tokenID := c.Param("token_id")
	if tokenID == "" {
		ResponseError(c, http.StatusBadRequest, "TOKEN ID不能为空")
		return
	}

	info, err := h.tokenAllocationService.GetTokenEnhanceInfo(userID.(uint), tokenID)
	if err != nil {
		ResponseError(c, http.StatusInternalServerError, "获取增强信息失败")
		return
	}

	ResponseSuccess(c, info)
}

// replaceTenantAddressWithProxy 使用代理地址替换租户地址
func (h *TokenAllocationHandler) replaceTenantAddressWithProxy(tenantAddress, proxyURL string) string {
	// 移除末尾的斜杠以便统一处理
	tenantAddress = strings.TrimSuffix(tenantAddress, "/")
	proxyURL = strings.TrimSuffix(proxyURL, "/")

	// 只处理 *.api.augmentcode.com 地址
	if !strings.HasSuffix(tenantAddress, ".api.augmentcode.com") {
		if !strings.HasSuffix(tenantAddress, "/") {
			tenantAddress += "/"
		}
		return tenantAddress
	}

	// 确保代理地址以/结尾
	proxyURL += "/"

	// 根据代理地址类型使用不同的替换规则
	if strings.Contains(proxyURL, "supabase.co") {
		// supabase.co 地址：将原始域名追加到代理地址路径中
		originalDomain := strings.TrimPrefix(tenantAddress, "https://")
		originalDomain = strings.TrimPrefix(originalDomain, "http://")
		return proxyURL + originalDomain + "/"
	} else {
		// deno.dev 或其他地址：提取子域名部分
		parts := strings.Split(tenantAddress, ".")
		if len(parts) >= 3 {
			subdomain := strings.TrimPrefix(parts[0], "https://")
			subdomain = strings.TrimPrefix(subdomain, "http://")
			return proxyURL + subdomain + "/"
		}
	}

	// 如果没有匹配的规则，返回原地址
	if !strings.HasSuffix(tenantAddress, "/") {
		tenantAddress += "/"
	}
	return tenantAddress
}
