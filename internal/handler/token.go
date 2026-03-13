package handler

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"augment-gateway/internal/config"
	"augment-gateway/internal/logger"
	"augment-gateway/internal/service"

	"github.com/gin-gonic/gin"
)

// TokenHandler Token处理器
type TokenHandler struct {
	tokenService      *service.TokenService
	proxyInfoService  service.ProxyInfoService
	authSessionClient service.AuthSessionClient
	config            *config.Config
}

// NewTokenHandler 创建Token处理器
func NewTokenHandler(tokenService *service.TokenService, proxyInfoService service.ProxyInfoService, authSessionClient service.AuthSessionClient, cfg *config.Config) *TokenHandler {
	return &TokenHandler{
		tokenService:      tokenService,
		proxyInfoService:  proxyInfoService,
		authSessionClient: authSessionClient,
		config:            cfg,
	}
}

// Create 创建token
func (h *TokenHandler) Create(c *gin.Context) {
	var req service.CreateTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseError(c, 400, "请求参数错误")
		return
	}

	// 处理AuthSession（如果提供）
	if req.AuthSession != "" {
		if err := h.processAuthSessionForToken(&req); err != nil {
			logger.Infof("[TOKEN管理] AuthSession处理失败: %v\n", err)
			ResponseError(c, 400, "添加失败，Session无效或已过期")
			return
		}
	}

	// 验证必填字段
	if req.Token == "" || req.TenantAddress == "" {
		ResponseError(c, 400, "TOKEN和租户地址不能为空")
		return
	}

	// 处理替换代理地址（如果提供）
	if req.ReplaceProxyURL != "" {
		replacedAddress := h.replaceTenantAddressWithProxy(req.TenantAddress, req.ReplaceProxyURL)
		logger.Infof("[TOKEN管理] 租户地址替换完成, 原地址: %s, 代理: %s, 替换后: %s\n",
			req.TenantAddress, req.ReplaceProxyURL, replacedAddress)
		req.TenantAddress = replacedAddress
	}

	token, err := h.tokenService.CreateToken(c.Request.Context(), &req)
	if err != nil {
		ResponseError(c, 500, "创建Token失败: "+err.Error())
		return
	}

	ResponseSuccessWithMsg(c, "Token创建成功", token)
}

// processAuthSessionForToken 处理AuthSession并自动填充TOKEN和租户地址
func (h *TokenHandler) processAuthSessionForToken(req *service.CreateTokenRequest) error {
	if req.AuthSession == "" {
		return nil // 没有Auth Session，跳过处理
	}

	logger.Infof("[TOKEN管理] 开始处理Auth Session, auth_session_length: %d\n", len(req.AuthSession))

	// 1. 先验证Auth Session是否有效
	if err := h.authSessionClient.ValidateAuthSession(req.AuthSession); err != nil {
		return fmt.Errorf("Auth Session无效: %v", err)
	}

	// 2. 通过Auth Session获取Tenant URL、Token、Email和新的AuthSession
	tenantURL, accessToken, email, newAuthSession, err := h.authSessionClient.AuthDevice(req.AuthSession)
	if err != nil {
		return fmt.Errorf("通过Auth Session获取认证信息失败: %v", err)
	}

	// 3. 更新请求数据（使用Auth Session获取的数据）
	req.TenantAddress = strings.TrimSuffix(tenantURL, "/") + "/"
	req.Token = accessToken
	if req.Email == "" && email != "" {
		req.Email = email
		logger.Infof("[TOKEN管理] 从AuthSession获取到邮箱: %s\n", email)
	}

	// 4. 使用刷新后的AuthSession，确保入库的是最新的
	if newAuthSession != "" && newAuthSession != req.AuthSession {
		logger.Infof("[TOKEN管理] AuthSession已刷新, new_session_length: %d\n", len(newAuthSession))
		req.AuthSession = newAuthSession
	}

	// 5. 获取Portal URL（如果请求中没有提供）
	if req.PortalURL == "" {
		appCookieSession, err := h.authSessionClient.AuthAppLogin(req.AuthSession)
		if err != nil {
			logger.Infof("[TOKEN管理] 获取App Session失败: %v，继续处理\n", err)
		} else {
			subscriptionInfo, err := h.authSessionClient.GetSubscriptionInfo(appCookieSession)
			if err != nil {
				logger.Infof("[TOKEN管理] 获取订阅信息失败: %v，继续处理\n", err)
			} else {
				if portalURLInterface, ok := subscriptionInfo["portalUrl"]; ok {
					if portalURLStr, ok := portalURLInterface.(string); ok && portalURLStr != "" {
						req.PortalURL = portalURLStr
						logger.Infof("[TOKEN管理] 成功获取Portal URL: %s\n", portalURLStr)
					}
				}
			}
		}
	}

	logger.Infof("[TOKEN管理] Auth Session处理完成, tenant_url: %s, token_length: %d",
		tenantURL, len(accessToken))

	return nil
}

// Get 获取token详情
func (h *TokenHandler) Get(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		ResponseError(c, 400, "Token ID不能为空")
		return
	}

	token, err := h.tokenService.GetToken(c.Request.Context(), id)
	if err != nil {
		ResponseError(c, 404, "Token不存在")
		return
	}

	ResponseSuccess(c, token)
}

// List 列出tokens
func (h *TokenHandler) List(c *gin.Context) {
	// 解析查询参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	status := c.Query("status")
	search := c.Query("search")
	submitterUsername := c.Query("submitter_username")
	isShared := c.Query("is_shared") // 共享状态筛选：空=全部，true=已共享，false=未共享

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	tokens, total, err := h.tokenService.ListTokens(c.Request.Context(), page, pageSize, status, search, submitterUsername, isShared)
	if err != nil {
		ResponseError(c, 500, "获取Token列表失败")
		return
	}

	ResponseSuccess(c, gin.H{
		"data": tokens,
		"pagination": gin.H{
			"page":        page,
			"page_size":   pageSize,
			"total":       total,
			"total_pages": (total + int64(pageSize) - 1) / int64(pageSize),
		},
	})
}

// Update 更新token
func (h *TokenHandler) Update(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		ResponseError(c, 400, "Token ID不能为空")
		return
	}

	var req service.UpdateTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseError(c, 400, "请求参数错误")
		return
	}

	token, err := h.tokenService.UpdateToken(c.Request.Context(), id, &req)
	if err != nil {
		ResponseError(c, 500, "更新Token失败")
		return
	}

	ResponseSuccessWithMsg(c, "Token更新成功", token)
}

// Delete 删除token
func (h *TokenHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		ResponseError(c, 400, "Token ID不能为空")
		return
	}

	if err := h.tokenService.DeleteToken(c.Request.Context(), id); err != nil {
		ResponseError(c, 500, "删除Token失败")
		return
	}

	ResponseSuccessWithMsg(c, "Token删除成功", nil)
}

// Stats 获取token统计信息
func (h *TokenHandler) Stats(c *gin.Context) {
	stats, err := h.tokenService.GetTokenStats(c.Request.Context())
	if err != nil {
		ResponseError(c, 500, "获取Token统计失败")
		return
	}

	ResponseSuccess(c, stats)
}

// Validate 验证token
func (h *TokenHandler) Validate(c *gin.Context) {
	tokenStr := c.Query("token")
	if tokenStr == "" {
		ResponseError(c, 400, "Token参数不能为空")
		return
	}

	tokenInfo, err := h.tokenService.ValidateToken(c.Request.Context(), tokenStr)
	if err != nil {
		ResponseError(c, 401, "Token无效")
		return
	}

	ResponseSuccess(c, gin.H{
		"valid": true,
		"data":  tokenInfo,
	})
}

// GetTokenUsers 获取TOKEN的使用用户列表
func (h *TokenHandler) GetTokenUsers(c *gin.Context) {
	tokenID := c.Param("id")
	if tokenID == "" {
		ResponseError(c, 400, "Token ID不能为空")
		return
	}

	users, err := h.tokenService.GetTokenUsers(c.Request.Context(), tokenID)
	if err != nil {
		ResponseError(c, 500, "获取TOKEN使用用户失败")
		return
	}

	ResponseSuccess(c, users)
}

// BatchImportTokenRequest 批量导入TOKEN请求结构
type BatchImportTokenRequest struct {
	Tokens []BatchImportTokenItem `json:"tokens" binding:"required"`
}

// BatchImportTokenItem 批量导入TOKEN项
type BatchImportTokenItem struct {
	Token         string `json:"token" binding:"required"`
	Name          string `json:"name" binding:"required"`
	Description   string `json:"description"`
	TenantAddress string `json:"tenant_address" binding:"required"`
}

// BatchImport 批量导入TOKEN
func (h *TokenHandler) BatchImport(c *gin.Context) {
	// 验证Authorization头
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" || authHeader != h.config.Security.BatchImportAuthToken {
		ResponseError(c, 401, "无效的授权令牌")
		return
	}

	var req BatchImportTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseError(c, 400, "请求参数错误")
		return
	}

	if len(req.Tokens) == 0 {
		ResponseError(c, 400, "TOKEN列表不能为空")
		return
	}

	// 使用批量导入服务
	result, err := h.tokenService.BatchImportTokens(c.Request.Context(), h.convertToBatchImportRequest(req.Tokens))
	if err != nil {
		ResponseError(c, 500, "批量导入失败: "+err.Error())
		return
	}

	ResponseSuccessWithMsg(c, "批量导入完成", result)
}

// convertToBatchImportRequest 转换为批量导入请求
func (h *TokenHandler) convertToBatchImportRequest(tokens []BatchImportTokenItem) []*service.BatchImportTokenItem {
	items := make([]*service.BatchImportTokenItem, len(tokens))
	for i, token := range tokens {
		items[i] = &service.BatchImportTokenItem{
			Token:         token.Token,
			Name:          token.Name,
			Description:   token.Description,
			TenantAddress: h.replaceTenantAddress(token.TenantAddress),
		}
	}
	return items
}

// replaceTenantAddress 替换租户地址
func (h *TokenHandler) replaceTenantAddress(address string) string {
	// 移除末尾的斜杠以便统一处理
	address = strings.TrimSuffix(address, "/")

	// 通用规则：处理所有 *.api.augmentcode.com 地址
	if strings.HasSuffix(address, ".api.augmentcode.com") {
		// 优先选择使用次数最少的代理地址
		selectedAddress, err := h.proxyInfoService.GetLeastUsedProxyAddress()
		if err != nil {
			logger.Warnf("[地址替换] 获取使用次数最少的代理地址失败: %v，降级为随机选择\n", err)
			// 如果获取失败，降级为随机选择有效代理
			validProxyAddresses, getErr := h.proxyInfoService.GetValidProxyAddresses()
			if getErr != nil || len(validProxyAddresses) == 0 {
				logger.Warnf("[地址替换] 获取有效代理地址也失败: %v，使用默认地址\n", getErr)
				// 如果连获取有效代理都失败，使用默认地址
				selectedAddress = "https://pure-hedgehog-19.deno.dev/"
			} else {
				selectedAddress = h.selectRandomAddress(validProxyAddresses)
				logger.Warnf("[地址替换] 随机选择代理地址: %s\n", selectedAddress)
			}
		}

		// 确保地址以/结尾
		if !strings.HasSuffix(selectedAddress, "/") {
			selectedAddress += "/"
		}

		// 根据替换地址类型使用不同的替换规则
		if strings.Contains(selectedAddress, "supabase.co") {
			// supabase.co 地址：将原始域名追加到代理地址路径中
			// 移除协议前缀
			originalDomain := strings.TrimPrefix(address, "https://")
			originalDomain = strings.TrimPrefix(originalDomain, "http://")
			// 格式：https://ughfgcwxvrwcqgscuifj.supabase.co/functions/v1/proxy/d15.api.augmentcode.com/
			return selectedAddress + originalDomain + "/"
		} else {
			// deno.dev 或其他地址：提取子域名部分
			parts := strings.Split(address, ".")
			if len(parts) >= 3 {
				subdomain := strings.TrimPrefix(parts[0], "https://")
				subdomain = strings.TrimPrefix(subdomain, "http://")
				return selectedAddress + subdomain + "/"
			}
		}
	}

	// 如果没有匹配的规则，返回原地址并确保以/结尾
	if !strings.HasSuffix(address, "/") {
		address += "/"
	}
	return address
}

// selectRandomAddress 随机选择一个地址
func (h *TokenHandler) selectRandomAddress(addresses []string) string {
	if len(addresses) == 0 {
		return "https://pure-hedgehog-19.deno.dev/"
	}

	if len(addresses) == 1 {
		return strings.TrimSpace(addresses[0])
	}

	// 使用现代的随机数生成方式
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	index := r.Intn(len(addresses))
	return strings.TrimSpace(addresses[index])
}

// BatchRefreshAuthSessionRequest 批量刷新AuthSession请求
type BatchRefreshAuthSessionRequest struct {
	TokenIDs []string `json:"token_ids" binding:"required"`
}

// BatchRefreshAuthSessionResult 批量刷新AuthSession结果
type BatchRefreshAuthSessionResult struct {
	Total        int                                 `json:"total"`
	SuccessCount int                                 `json:"success_count"`
	FailedCount  int                                 `json:"failed_count"`
	SkippedCount int                                 `json:"skipped_count"`
	Results      []BatchRefreshAuthSessionItemResult `json:"results"`
}

// BatchRefreshAuthSessionItemResult 单个TOKEN刷新结果
type BatchRefreshAuthSessionItemResult struct {
	TokenID     string `json:"token_id"`
	TokenPrefix string `json:"token_prefix"`
	Success     bool   `json:"success"`
	Skipped     bool   `json:"skipped"`
	Message     string `json:"message"`
}

// BatchRefreshAuthSession 批量刷新TOKEN的AuthSession
func (h *TokenHandler) BatchRefreshAuthSession(c *gin.Context) {
	var req BatchRefreshAuthSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ResponseError(c, 400, "请求参数错误")
		return
	}

	if len(req.TokenIDs) == 0 {
		ResponseError(c, 400, "TOKEN ID列表不能为空")
		return
	}

	result := &BatchRefreshAuthSessionResult{
		Total:   len(req.TokenIDs),
		Results: make([]BatchRefreshAuthSessionItemResult, 0, len(req.TokenIDs)),
	}

	ctx := c.Request.Context()

	for _, tokenID := range req.TokenIDs {
		itemResult := BatchRefreshAuthSessionItemResult{
			TokenID: tokenID,
		}

		// 获取TOKEN信息
		tokenInfo, err := h.tokenService.GetToken(ctx, tokenID)
		if err != nil {
			itemResult.Success = false
			itemResult.Message = "TOKEN不存在"
			result.FailedCount++
			result.Results = append(result.Results, itemResult)
			continue
		}

		itemResult.TokenPrefix = tokenInfo.Token.Token[:min(8, len(tokenInfo.Token.Token))] + "..."

		// 检查是否有AuthSession
		if tokenInfo.Token.AuthSession == "" {
			itemResult.Skipped = true
			itemResult.Message = "未配置AuthSession，已跳过"
			result.SkippedCount++
			result.Results = append(result.Results, itemResult)
			continue
		}

		// 验证AuthSession是否有效
		if err := h.authSessionClient.ValidateAuthSession(tokenInfo.Token.AuthSession); err != nil {
			itemResult.Success = false
			itemResult.Message = fmt.Sprintf("AuthSession验证失败: %v", err)
			result.FailedCount++
			result.Results = append(result.Results, itemResult)
			continue
		}

		// 通过AuthSession获取新的TOKEN、租户地址和新的AuthSession
		tenantURL, accessToken, _, newAuthSession, err := h.authSessionClient.AuthDevice(tokenInfo.Token.AuthSession)
		if err != nil {
			itemResult.Success = false
			itemResult.Message = fmt.Sprintf("刷新失败: %v", err)
			result.FailedCount++
			result.Results = append(result.Results, itemResult)
			continue
		}

		// 更新数据库中的TOKEN、租户地址和AuthSession
		newTenantAddress := strings.TrimSuffix(tenantURL, "/") + "/"
		updateReq := &service.UpdateTokenRequest{
			Token:         accessToken,
			TenantAddress: newTenantAddress,
			AuthSession:   &newAuthSession,
		}

		if _, err := h.tokenService.UpdateToken(ctx, tokenID, updateReq); err != nil {
			itemResult.Success = false
			itemResult.Message = fmt.Sprintf("更新数据库失败: %v", err)
			result.FailedCount++
			result.Results = append(result.Results, itemResult)
			continue
		}

		itemResult.Success = true
		itemResult.Message = "刷新成功，TOKEN和AuthSession已更新"
		result.SuccessCount++
		result.Results = append(result.Results, itemResult)

		logger.Infof("[TOKEN管理] 批量刷新 - TOKEN %s... 刷新成功\n", itemResult.TokenPrefix)
	}

	ResponseSuccessWithMsg(c, fmt.Sprintf("批量刷新完成：成功 %d，失败 %d，跳过 %d",
		result.SuccessCount, result.FailedCount, result.SkippedCount), result)
}

// replaceTenantAddressWithProxy 使用指定代理地址替换租户地址
func (h *TokenHandler) replaceTenantAddressWithProxy(tenantAddress, proxyURL string) string {
	// 移除末尾的斜杠以便统一处理
	tenantAddress = strings.TrimSuffix(tenantAddress, "/")
	proxyURL = strings.TrimSuffix(proxyURL, "/")

	// 只处理 *.api.augmentcode.com 地址
	if !strings.HasSuffix(tenantAddress, ".api.augmentcode.com") {
		// 如果不是 augmentcode 地址，返回原地址
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
		// 移除协议前缀
		originalDomain := strings.TrimPrefix(tenantAddress, "https://")
		originalDomain = strings.TrimPrefix(originalDomain, "http://")
		// 格式：https://xxx.supabase.co/functions/v1/proxy/d15.api.augmentcode.com/
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

	// 如果没有匹配的规则，返回原地址并确保以/结尾
	if !strings.HasSuffix(tenantAddress, "/") {
		tenantAddress += "/"
	}
	return tenantAddress
}

// GetBanReason 获取TOKEN的详细封禁原因（通过AuthSession调用远程API）
func (h *TokenHandler) GetBanReason(c *gin.Context) {
	tokenID := c.Param("id")
	if tokenID == "" {
		ResponseError(c, 400, "Token ID不能为空")
		return
	}

	// 获取TOKEN信息
	token, err := h.tokenService.GetToken(c.Request.Context(), tokenID)
	if err != nil {
		ResponseError(c, 404, "Token不存在")
		return
	}

	// 检查是否有AuthSession
	if token.AuthSession == "" {
		ResponseError(c, 400, "该TOKEN没有AuthSession，无法查询封禁原因")
		return
	}

	// 调用远程API获取封禁原因
	banReason, err := h.authSessionClient.GetUserBanReason(token.AuthSession)
	if err != nil {
		logger.Warnf("[TOKEN管理] 获取封禁原因失败: %v\n", err)
		ResponseError(c, 500, fmt.Sprintf("获取封禁原因失败: %v", err))
		return
	}

	ResponseSuccess(c, gin.H{
		"ban_reason": banReason,
	})
}
