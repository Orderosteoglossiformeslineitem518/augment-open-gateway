package handler

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"augment-gateway/internal/config"
	"augment-gateway/internal/database"
	"augment-gateway/internal/logger"
	"augment-gateway/internal/proxy"
	"augment-gateway/internal/service"
	"augment-gateway/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// SignatureResult 签名结果结构体
type SignatureResult struct {
	SignatureVersion string `json:"signatureVersion"`
	Timestamp        int64  `json:"timestamp"`
	Signature        string `json:"signature"`
	Vector           string `json:"vector"`
	FailureReason    string `json:"failureReason,omitempty"`
}

// SignatureBuilder 签名构建器
type SignatureBuilder struct {
	version string
}

// NewSignatureBuilder 创建新的签名构建器
func NewSignatureBuilder() *SignatureBuilder {
	return &SignatureBuilder{
		version: "1.0",
	}
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// extractStatusCode 从错误信息中提取状态码
func extractStatusCode(errMsg string) string {
	if strings.Contains(errMsg, "401") {
		return "401"
	}
	if strings.Contains(errMsg, "403") {
		return "403"
	}
	return "未知"
}

// parseVersion 解析版本号字符串为 [major, minor, patch]
func parseVersion(version string) ([]int, error) {
	// 移除可能的前缀 "v"
	version = strings.TrimPrefix(version, "v")

	// 按 "." 分割版本号
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("无效的版本号格式: %s", version)
	}

	result := make([]int, 3)
	for i, part := range parts {
		num, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("无效的版本号部分: %s", part)
		}
		result[i] = num
	}

	return result, nil
}

// compareVersions 比较两个版本号，返回 -1(v1 < v2), 0(v1 == v2), 1(v1 > v2)
func compareVersions(v1, v2 []int) int {
	for i := 0; i < 3; i++ {
		if v1[i] < v2[i] {
			return -1
		}
		if v1[i] > v2[i] {
			return 1
		}
	}
	return 0
}

// extractVSCodeAugmentVersion 从 User-Agent 中提取 vscode-augment 版本号
// 例如: "Augment.vscode-augment/0.561.0 (win32; x64; 10.0.26100) vscode/1.105.1" -> "0.561.0"
func extractVSCodeAugmentVersion(userAgent string) (string, error) {
	// 检查是否包含 vscode-augment
	if !strings.Contains(userAgent, "vscode-augment") {
		return "", fmt.Errorf("User-Agent 不包含 vscode-augment")
	}

	// 使用正则表达式提取版本号
	re := regexp.MustCompile(`vscode-augment/(\d+\.\d+\.\d+)`)
	matches := re.FindStringSubmatch(userAgent)
	if len(matches) < 2 {
		return "", fmt.Errorf("无法从 User-Agent 中提取版本号")
	}

	return matches[1], nil
}

// checkVSCodeAugmentVersion 检查 VSCode Augment 版本是否满足最低要求
// 返回 (是否满足要求, 检测到的版本号, 错误)
func (h *ProxyHandler) checkVSCodeAugmentVersion(userAgent string) (bool, string, error) {
	// 如果未启用版本检测，直接返回通过
	if !h.config.Proxy.EnableVersionCheck {
		return true, "", nil
	}

	// 提取版本号
	currentVersion, err := extractVSCodeAugmentVersion(userAgent)
	if err != nil {
		// 如果无法提取版本号（例如不是 vscode-augment 客户端），则放行
		return true, "", nil
	}

	// 解析当前版本号
	currentVer, err := parseVersion(currentVersion)
	if err != nil {
		logger.Warnf("[版本检测] 解析当前版本号失败: %v\n", err)
		return true, currentVersion, nil // 解析失败时放行，避免误拦截
	}

	// 解析最低版本号
	minVer, err := parseVersion(h.config.Proxy.MinVSCodeAugmentVersion)
	if err != nil {
		logger.Warnf("[版本检测] 解析最低版本号失败: %v\n", err)
		return true, currentVersion, nil // 配置错误时放行
	}

	// 比较版本号
	result := compareVersions(currentVer, minVer)
	if result < 0 {
		// 当前版本低于最低要求
		return false, currentVersion, nil
	}

	return true, currentVersion, nil
}

// ProxyHandler 代理处理器
type ProxyHandler struct {
	proxyService              *proxy.ProxyService
	tokenService              *service.TokenService
	userAuthService           *service.UserAuthService // 用户认证服务（替代userTokenService）
	cacheService              *service.CacheService
	statsService              *service.StatsService
	banRecordService          *service.BanRecordService
	requestRecordService      *service.RequestRecordService
	conversationIDService     *service.ConversationIDService
	authSessionClient         service.AuthSessionClient
	limitResponseHandler      *LimitResponseHandler
	poolTokenLimitService     *service.PoolTokenLimitService
	poolTokenLimitRespHandler *PoolTokenLimitResponseHandler
	mockDataGenerator         *utils.MockDataGenerator
	getModelsModifier         *GetModelsModifier
	subscriptionInfoModifier  *SubscriptionInfoModifier
	signatureBuilder          *SignatureBuilder
	config                    *config.Config
	userUsageStatsService     *service.UserUsageStatsService  // 用户使用统计服务
	enhancedProxyHandler      *EnhancedProxyHandler           // 增强代理处理器
	externalChannelService    *service.ExternalChannelService // 外部渠道服务
	sharedTokenService        *service.SharedTokenService     // 共享TOKEN服务
	remoteModelService        *service.RemoteModelService     // 远程模型服务
}

// NewProxyHandler 创建代理处理器
func NewProxyHandler(
	proxyService *proxy.ProxyService,
	tokenService *service.TokenService,
	userAuthService *service.UserAuthService,
	cacheService *service.CacheService,
	statsService *service.StatsService,
	banRecordService *service.BanRecordService,
	requestRecordService *service.RequestRecordService,
	conversationIDService *service.ConversationIDService,
	authSessionClient service.AuthSessionClient,
	userUsageStatsService *service.UserUsageStatsService,
	externalChannelService *service.ExternalChannelService,
	sharedTokenService *service.SharedTokenService,
	remoteModelService *service.RemoteModelService,
	cfg *config.Config,
) *ProxyHandler {
	limitResponseHandler := NewLimitResponseHandler(cacheService, userAuthService)
	poolTokenLimitService := service.NewPoolTokenLimitService(cacheService)
	poolTokenLimitRespHandler := NewPoolTokenLimitResponseHandler()

	// 创建增强代理处理器（如果外部渠道服务可用）
	var enhancedProxyHandler *EnhancedProxyHandler
	if externalChannelService != nil {
		enhancedProxyHandler = NewEnhancedProxyHandler(externalChannelService.GetDB(), externalChannelService, cfg)
		// 设置用户认证服务（用于获取用户设置）
		enhancedProxyHandler.SetUserAuthService(userAuthService)
	}

	return &ProxyHandler{
		proxyService:              proxyService,
		tokenService:              tokenService,
		userAuthService:           userAuthService,
		cacheService:              cacheService,
		statsService:              statsService,
		banRecordService:          banRecordService,
		requestRecordService:      requestRecordService,
		authSessionClient:         authSessionClient,
		conversationIDService:     conversationIDService,
		limitResponseHandler:      limitResponseHandler,
		poolTokenLimitService:     poolTokenLimitService,
		poolTokenLimitRespHandler: poolTokenLimitRespHandler,
		mockDataGenerator:         utils.NewMockDataGenerator(),
		getModelsModifier:         NewGetModelsModifier(&cfg.GetModels),
		subscriptionInfoModifier:  NewSubscriptionInfoModifier(&cfg.SubscriptionInfo),
		signatureBuilder:          NewSignatureBuilder(),
		config:                    cfg,
		userUsageStatsService:     userUsageStatsService,
		enhancedProxyHandler:      enhancedProxyHandler,
		externalChannelService:    externalChannelService,
		sharedTokenService:        sharedTokenService,
		remoteModelService:        remoteModelService,
	}
}

// extractToken 提取token
func (h *ProxyHandler) extractToken(c *gin.Context) string {
	// 根据Augment API文档，优先从Authorization头提取Bearer token
	auth := c.GetHeader("Authorization")
	if auth != "" {
		if strings.HasPrefix(auth, "Bearer ") {
			return strings.TrimPrefix(auth, "Bearer ")
		}
		// 如果Authorization头存在但不是Bearer格式，直接使用
		return auth
	}

	// 从查询参数提取（备选方案）
	if token := c.Query("token"); token != "" {
		return token
	}

	// 从X-API-Key头提取（备选方案）
	if token := c.GetHeader("X-API-Key"); token != "" {
		return token
	}

	// 从自定义头部提取（如果客户端使用其他方式）
	if token := c.GetHeader("X-Auth-Token"); token != "" {
		return token
	}

	return ""
}

// validateToken 验证token
func (h *ProxyHandler) validateToken(ctx context.Context, tokenStr string) (*service.TokenInfo, error) {
	// 先从缓存获取
	cachedToken, hit, err := h.cacheService.GetToken(ctx, tokenStr)
	if err != nil {
		return nil, fmt.Errorf("缓存错误: %w", err)
	}

	if hit && cachedToken != nil {
		// 缓存命中，检查token是否有效
		if !cachedToken.IsActive() {
			return nil, fmt.Errorf("令牌未激活或已过期")
		}
		return &service.TokenInfo{
			Token: *cachedToken,
		}, nil
	}

	// 缓存未命中，从数据库获取
	tokenInfo, err := h.tokenService.ValidateToken(ctx, tokenStr)
	if err != nil {
		return nil, err
	}

	// 缓存token信息
	if err := h.cacheService.CacheToken(ctx, &tokenInfo.Token); err != nil {
		// 缓存失败不影响主流程，只记录日志
		logger.Warnf("警告: 缓存令牌失败: %v\n", err)
	}

	return tokenInfo, nil
}

// readRequestBody 读取请求体
func (h *ProxyHandler) readRequestBody(c *gin.Context) ([]byte, error) {
	if c.Request.Body == nil {
		return nil, nil
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil, fmt.Errorf("读取请求体失败: %w", err)
	}

	return body, nil
}

// recordStats 记录统计信息
func (h *ProxyHandler) recordStats(
	tokenInfo *service.TokenInfo,
	req *proxy.ProxyRequest,
	resp *proxy.ProxyResponse,
) {
	// 异步记录统计信息
	go func() {
		// 创建新的上下文，避免原上下文取消影响统计记录
		statsCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		success := resp.StatusCode == 200

		// 记录到缓存
		if err := h.cacheService.IncrementRequestCount(statsCtx, req.Token.Token); err != nil {
			logger.Warnf("警告: 增加请求计数失败: %v\n", err)
		}

		if success {
			if err := h.cacheService.IncrementSuccessCount(statsCtx, req.Token.Token); err != nil {
				logger.Warnf("警告: 增加成功计数失败: %v\n", err)
			}
		} else {
			if err := h.cacheService.IncrementErrorCount(statsCtx, req.Token.Token); err != nil {
				logger.Warnf("警告: 增加错误计数失败: %v\n", err)
			}
		}

		// 记录到数据库 - 直接TOKEN代理
		requestLog := h.proxyService.LogRequest(&tokenInfo.Token, req, resp)
		if err := h.statsService.LogRequest(statsCtx, requestLog); err != nil {
			logger.Warnf("警告: 记录请求日志失败: %v\n", err)
		}

	}()
}

// handleProxyError 处理代理错误
func (h *ProxyHandler) handleProxyError(
	c *gin.Context,
	tokenInfo *service.TokenInfo,
	req *proxy.ProxyRequest,
	err error,
) {
	// 记录错误统计
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := h.cacheService.IncrementErrorCount(ctx, req.Token.Token); err != nil {
			logger.Warnf("警告: 增加错误计数失败: %v\n", err)
		}

		// 记录错误日志
		errorResp := &proxy.ProxyResponse{
			StatusCode:   http.StatusBadGateway,
			ErrorMessage: err.Error(),
		}
		requestLog := h.proxyService.LogRequest(&tokenInfo.Token, req, errorResp)
		if err := h.statsService.LogRequest(ctx, requestLog); err != nil {
			logger.Warnf("警告: 记录错误请求日志失败: %v\n", err)
		}
	}()

	h.respondError(c, http.StatusBadGateway, "代理错误", err)
}

// respondSuccess 返回成功响应
func (h *ProxyHandler) respondSuccess(c *gin.Context, resp *proxy.ProxyResponse) {
	// 复制响应头
	for key, values := range resp.Headers {
		for _, value := range values {
			c.Header(key, value)
		}
	}

	// 设置状态码和响应体
	c.Data(resp.StatusCode, c.GetHeader("Content-Type"), resp.Body)
}

// respondError 返回错误响应
func (h *ProxyHandler) respondError(c *gin.Context, statusCode int, message string, err error) {
	// 对于代理错误，我们保持HTTP状态码，但使用统一的响应格式
	response := Response{
		Code: statusCode,
		Msg:  message,
		Data: nil,
	}

	// 如果有详细错误信息，添加到data中
	if requestID, exists := c.Get("request_id"); err != nil || exists {
		data := gin.H{}
		if err != nil {
			data["details"] = err.Error()
		}
		if exists {
			data["request_id"] = requestID
		}
		data["time"] = time.Now().Unix()
		response.Data = data
	}

	c.JSON(statusCode, response)
}

// ForwardWithUserToken 使用用户令牌转发请求
func (h *ProxyHandler) ForwardWithUserToken(c *gin.Context) {
	startTime := time.Now()
	requestID := uuid.New().String()

	// 设置请求ID到上下文
	c.Set("request_id", requestID)

	// 检查转发服务维护状态
	if h.config.Proxy.ForwardDisabled {
		logger.Infof("[代理] 转发服务维护中，拒绝请求\n")
		c.JSON(http.StatusUnauthorized, gin.H{
			"code": 401,
			"msg":  "当前转发服务维护中，暂不可用",
			"data": nil,
		})
		return
	}

	// 记录客户端请求（异步执行）
	clientIP := c.ClientIP()
	h.requestRecordService.RecordRequest(c.Request, clientIP)

	// 解析用户令牌
	userToken := h.extractToken(c)
	if userToken == "" {
		h.respondError(c, http.StatusUnauthorized, "User token is required", nil)
		return
	}

	// 快速验证用户令牌（优先使用缓存）
	userTokenInfo, validationErr := h.validateUserTokenFast(c.Request.Context(), userToken)
	if validationErr != nil {
		// 根据验证失败的类型采取不同的处理策略
		switch validationErr.Type {
		case "disabled":
			h.limitResponseHandler.RespondWithDisabledTokenMessage(c)
			return
		case "expired":
			h.limitResponseHandler.RespondWithDisabledTokenMessage(c)
			return
		case "max_requests":
			h.limitResponseHandler.RespondWithMaxRequestsMessage(c)
			return
		default:
			h.limitResponseHandler.RespondWithServiceError(c)
			return
		}
	}

	// 构建完整的请求路径，包含查询参数
	fullPath := c.Param("path")
	if c.Request.URL.RawQuery != "" {
		fullPath += "?" + c.Request.URL.RawQuery
	}

	// 检查黑名单限制（优先级高于白名单）
	if h.config.Proxy.BlacklistEnabled {
		if h.isBlacklistedPath(fullPath) {
			logger.Infof("[代理] 黑名单限制已启用，拦截黑名单请求: %s，返回空成功响应", fullPath)
			c.JSON(http.StatusOK, gin.H{})
			return
		}
	}

	// 检查是否为 /subscription-info 请求，需要异步更新TOKEN过期时间
	isSubscriptionInfo := strings.Contains(fullPath, "/subscription-info")

	// 判断是否为需要统计和限频的接口（仅对/chat-stream接口进行统计和限频）
	needsStatsAndRateLimit := strings.HasSuffix(fullPath, "/chat-stream")

	// 使用优化的限制检查
	if needsStatsAndRateLimit {
		allowed, limitType := h.optimizedLimitCheck(c.Request.Context(), userToken, userTokenInfo, fullPath)
		if !allowed {
			// 返回相应的限制响应
			switch limitType {
			case "rate_limit":
				resetMinutes := h.limitResponseHandler.GetRateLimitResetTime()
				h.limitResponseHandler.RespondWithRateLimitMessage(c, resetMinutes)
				return
			case "max_requests":
				h.limitResponseHandler.RespondWithMaxRequestsMessage(c)
				return
			case "disabled":
				h.limitResponseHandler.RespondWithDisabledTokenMessage(c)
				return
			}
		}
	}

	// 获取用户令牌分配的固定TOKEN
	availableToken, err := h.getAssignedTokenForUser(c.Request.Context(), userTokenInfo.Token)
	if err != nil {
		// 检查是否是号池TOKEN限制错误
		if strings.HasPrefix(err.Error(), "POOL_TOKEN_LIMIT_EXCEEDED:") {
			// 解析限制信息
			parts := strings.Split(err.Error(), ":")
			if len(parts) >= 3 {
				currentCount, _ := strconv.Atoi(parts[1])
				maxCount, _ := strconv.Atoi(parts[2])

				// 返回号池TOKEN限制超限的模拟响应
				logger.Infof("[代理] 用户 %s... 号池TOKEN切换次数超限，返回模拟响应\n",
					userTokenInfo.Token[:min(8, len(userTokenInfo.Token))])
				h.poolTokenLimitRespHandler.SendPoolTokenLimitExceededResponse(c, currentCount, maxCount)
				return
			}
		}

		h.respondError(c, http.StatusServiceUnavailable, "Token service error", err)
		return
	}

	// 如果没有可用TOKEN，根据请求类型返回不同响应
	if availableToken == nil {
		if strings.HasSuffix(fullPath, "/get-models") {
			// API接口返回401状态码，让客户端停止重试
			logger.Infof("[代理] 用户 %s... 没有可用令牌，API请求返回401状态码",
				userTokenInfo.Token[:min(8, len(userTokenInfo.Token))])
			h.respondError(c, http.StatusUnauthorized, "No available tokens", fmt.Errorf("用户没有可用的TOKEN"))
			return
		} else if strings.HasSuffix(fullPath, "/chat-stream") {
			// 聊天接口返回模拟对话响应
			logger.Infof("[代理] 用户 %s... 没有可用令牌，聊天请求返回模拟响应",
				userTokenInfo.Token[:min(8, len(userTokenInfo.Token))])
			h.limitResponseHandler.RespondWithNoTokenAvailable(c)
			return
		} else {
			// 其他接口返回空JSON响应
			logger.Infof("[代理] 用户 %s... 没有可用令牌，返回空JSON响应",
				userTokenInfo.Token[:min(8, len(userTokenInfo.Token))])
			c.JSON(http.StatusOK, gin.H{})
			return
		}
	}

	// 移除使用次数检查，TOKEN状态检查已在获取时完成
	// 真正达到使用上限时会通过自动封禁机制改变TOKEN状态

	// 读取请求体
	body, err := h.readRequestBody(c)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, "Failed to read request body", err)
		return
	}

	// 对于 codebase-retrieval 和 commit-retrieval 请求，清空 dialog 字段
	// 这些是无状态的代码检索请求，不需要对话历史，避免孤立的 tool_use 导致请求失败
	if strings.Contains(fullPath, "/agents/codebase-retrieval") || strings.Contains(fullPath, "/agents/commit-retrieval") {
		body = h.clearDialogField(body)
	}

	// 检查是否为事件记录请求，如果conversation_id对应的对话已被转发到外部渠道，则拦截请求
	if strings.Contains(fullPath, "/record-session-events") || strings.Contains(fullPath, "/record-request-events") {
		if intercepted := h.interceptEnhancedConversationEvents(c, body, fullPath); intercepted {
			return
		}
	}

	// 获取 User-Agent
	userAgent := c.Request.UserAgent()

	// 检查是否为 /get-models 请求，需要特殊处理响应数据
	if strings.HasSuffix(fullPath, "/get-models") {
		h.handleGetModelsRequest(c, availableToken, userTokenInfo, fullPath, body)
		return
	}

	// 检查是否为 /subscription-info 请求，需要特殊处理响应数据
	if strings.HasSuffix(fullPath, "/subscription-info") {
		h.handleSubscriptionInfoRequest(c, availableToken, userTokenInfo, fullPath, body)
		return
	}

	// 检查是否为 /subscription-banner 请求，转发到远程但返回空JSON给客户端
	if strings.Contains(fullPath, "/subscription-banner") {
		h.handleSubscriptionBannerRequest(c, availableToken, fullPath, body)
		return
	}

	// 检查是否为 /report-feature-vector 请求，需要模拟请求体
	if strings.Contains(fullPath, "/report-feature-vector") {
		// 获取请求头中的 x-request-id，如果没有则生成一个 UUID
		requestID := c.GetHeader("x-request-id")
		if requestID == "" {
			requestID = uuid.New().String()
		}

		simulatedBody, err := h.getOrGenerateFeatureVectorBody(c.Request.Context(), availableToken, requestID, userAgent)
		if err != nil {
			logger.Infof("[代理] 获取或生成模拟特征向量请求体失败: %v", err)
		} else {
			body = simulatedBody
			logger.Infof("[代理] 已为 /report-feature-vector 请求使用TOKEN %s... 的模拟请求体",
				availableToken.Token[:min(8, len(availableToken.Token))])
		}
	}

	// 检查是否启用 conversation_id 替换功能
	if h.config.Proxy.EnableConversationIDReplacement {
		// 检查是否为需要处理 conversation_id 的接口
		needsConversationIDHandling := strings.HasSuffix(fullPath, "/chat-stream") ||
			strings.Contains(fullPath, "/record-request-events") ||
			strings.Contains(fullPath, "/record-session-events")

		if needsConversationIDHandling {
			// 处理 conversation_id 替换
			modifiedBody, err := h.handleConversationIDReplacement(
				c.Request.Context(),
				body,
				userTokenInfo.Token,
				availableToken.ID,
				fullPath,
			)
			if err != nil {
				logger.Warnf("[conversation_id] 替换失败: %v，继续使用原始请求体", err)
			} else {
				body = modifiedBody
			}
		}
	}

	// 构建代理请求
	proxyReq := &proxy.ProxyRequest{
		Token:         availableToken,
		Method:        c.Request.Method,
		Path:          fullPath,
		Headers:       c.Request.Header,
		Body:          body,
		ClientIP:      proxy.GetClientIP(c.Request),
		UserAgent:     c.Request.UserAgent(),
		TenantAddress: availableToken.TenantAddress,
		SessionID:     availableToken.SessionID,
	}

	// 检查是否需要添加签名请求头
	h.addSignatureHeadersIfNeeded(proxyReq, fullPath, body)

	// 检查是否为 /chat-stream 请求，需要进行特殊处理
	if strings.HasSuffix(fullPath, "/chat-stream") {
		// 0. 检查是否使用共享TOKEN，如果是则必须绑定外部渠道
		if h.sharedTokenService != nil {
			isShared, _ := h.sharedTokenService.IsSharedToken(availableToken.ID)
			if isShared {
				// 检查用户是否绑定了外部渠道
				hasChannel := false
				if h.externalChannelService != nil {
					channel, err := h.getTokenBoundChannel(availableToken, userTokenInfo.UserID)
					if err == nil && channel != nil {
						hasChannel = true
					}
				}
				if !hasChannel {
					// 共享TOKEN未绑定外部渠道，返回提示消息
					logger.Infof("[代理] 共享TOKEN %s... 未绑定外部渠道，返回提示消息",
						availableToken.Token[:min(8, len(availableToken.Token))])
					h.limitResponseHandler.RespondWithSharedTokenNoChannel(c)
					return
				}
			}
		}

		// 1. 检查 VSCode Augment 版本
		versionOK, detectedVersion, err := h.checkVSCodeAugmentVersion(userAgent)
		if err != nil {
			logger.Warnf("[版本检测] 版本检测失败: %v\n", err)
		}

		if !versionOK {
			// 版本过低，返回模拟响应
			logger.Warnf("[版本检测] 检测到版本过低: %s < %s，拦截请求",
				detectedVersion, h.config.Proxy.MinVSCodeAugmentVersion)
			h.limitResponseHandler.RespondWithVersionTooLow(c, detectedVersion, h.config.Proxy.MinVSCodeAugmentVersion)
			return
		}

		// 2. 检查TOKEN是否启用了增强功能
		// 安全检查：如果请求参数中的message字段包含 BeginResponseMarker，跳过外部渠道转发，回退到本地代理
		// 特殊情况：对于积分为0的账号（max_requests为0），即使包含BeginResponseMarker也要使用外部渠道转发
		containsSecurityMarker := false
		isZeroCreditAccount := availableToken.MaxRequests == 0
		var chatReq struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(body, &chatReq); err == nil && strings.Contains(chatReq.Message, BeginResponseMarker) {
			if isZeroCreditAccount {
				// 积分为0的账号，即使包含BeginResponseMarker也使用外部渠道转发
				logger.Infof("[增强代理] 检测到增强提示请求，但账号积分为0，强制使用外部渠道转发")
				// containsSecurityMarker 保持为 false，继续使用外部渠道
			} else {
				containsSecurityMarker = true
				logger.Infof("[增强代理] 当前为增强提示请求，跳过外部渠道转发")
			}
		}

		// 检查当前TOKEN是否为共享TOKEN
		isSharedToken := false
		if h.sharedTokenService != nil {
			isSharedToken, _ = h.sharedTokenService.IsSharedToken(availableToken.ID)
		}

		// 共享账号透传权限检查：从请求体中提取模型名称，检查该模型是否允许共享账号透传
		// 若用户已在绑定渠道中配置了该模型的映射，则放行走增强代理（使用映射），不在此拦截
		if isSharedToken && h.remoteModelService != nil {
			var modelReq struct {
				Model string `json:"model"`
			}
			if err := json.Unmarshal(body, &modelReq); err == nil && modelReq.Model != "" {
				if !h.remoteModelService.IsModelPassthroughAllowed(modelReq.Model) {
					// 不允许透传时，仅当渠道已配置该模型映射才放行，否则拦截
					channelForCheck, errCh := h.getTokenBoundChannel(availableToken, userTokenInfo.UserID)
					hasMapping := errCh == nil && channelForCheck != nil && availableToken.EnhancedEnabled &&
						h.enhancedProxyHandler != nil && h.enhancedProxyHandler.HasModelMapping(channelForCheck, modelReq.Model)
					if !hasMapping {
						logger.Infof("[代理] 共享TOKEN %s... 请求的模型 %s 不允许共享账号透传且渠道未配置映射",
							availableToken.Token[:min(8, len(availableToken.Token))], modelReq.Model)
						h.limitResponseHandler.RespondWithModelNotMapped(c)
						return
					}
				}
			}
		}

		if availableToken.EnhancedEnabled && h.enhancedProxyHandler != nil && !containsSecurityMarker {
			// 获取TOKEN绑定的外部渠道
			channel, err := h.getTokenBoundChannel(availableToken, userTokenInfo.UserID)
			if err == nil && channel != nil {
				logger.Infof("[增强代理] TOKEN %s... 启用了增强功能，使用外部渠道: %s",
					availableToken.Token[:min(8, len(availableToken.Token))], channel.ProviderName)

				// 使用增强代理处理器处理请求（带故障转移）
				enhancedErr := h.enhancedProxyHandler.HandleEnhancedChatStreamWithFailover(c, body, availableToken, channel, userTokenInfo.UserID)
				// 判断是否为模型未配置映射的错误（将回退到透传，不记录增强代理错误日志）
				isModelNotMappedErr := enhancedErr != nil &&
					(errors.Is(enhancedErr, ErrModelNotMapped) || strings.Contains(enhancedErr.Error(), "模型未配置映射"))

				// 记录请求日志（排除即将回退透传的模型未映射情况，透传成功后由计费逻辑记录）
				if !isModelNotMappedErr {
					h.logEnhancedProxyRequest(c.Request.Context(), userTokenInfo, proxyReq, startTime, channel.ID, channel.ProviderName, enhancedErr)
				}

				if enhancedErr != nil {
					logger.Infof("[增强代理] 增强代理处理失败: %v", enhancedErr)

					// 模型为null（渠道无模型映射且请求未指定模型）：直接拦截，返回错误
					if errors.Is(enhancedErr, ErrModelIsNull) {
						logger.Infof("[增强代理] 模型为null，返回提示消息（共享: %v）", isSharedToken)
						h.limitResponseHandler.RespondWithModelNotMapped(c)
						return
					}
					if isModelNotMappedErr {
						// 模型未配置映射：回退到直接转发上游（透传）
						// - 共享账号：已通过前面的透传权限检查，允许回退
						// - 非共享账号：不受透传限制，直接回退
						logger.Infof("[增强代理] 模型未配置映射，回退到直接转发上游（共享: %v）", isSharedToken)
						// fall through to direct forwarding below
					} else {
						// 其他错误，将错误信息以模型响应形式返回客户端
						h.enhancedProxyHandler.SendErrorResponse(c, enhancedErr.Error())
						return
					}
				} else {
					// 增强代理处理成功，将conversation_id存储到缓存中
					// 用于后续拦截record-session-events和record-request-events请求
					h.cacheEnhancedConversationID(c.Request.Context(), body)

					// 增强代理处理成功，直接返回
					return
				}
			} else if err != nil {
				logger.Infof("[增强代理] 获取绑定渠道失败: %v，使用原始代理", err)
			}
		}

		// 3. 替换 chat_history 中的 request_id
		modifiedBody, err := h.replaceChatHistoryRequestIDs(body)
		if err != nil {
			logger.Infof("[代理] 替换 chat_history request_id 失败: %v", err)
			// 失败不影响正常请求，继续使用原始请求体
		} else {
			body = modifiedBody
		}

		// 4. 进行基于请求的计费检查
		h.handleRequestBasedBilling(c.Request.Context(), body, userTokenInfo, proxyReq, startTime)
	}

	// 验证请求
	if err := h.proxyService.ValidateRequest(proxyReq); err != nil {
		h.respondError(c, http.StatusBadRequest, "Invalid request", err)
		return
	}

	// 所有请求都使用流式处理，远程服务器返回 Transfer-Encoding: chunked 时自动使用流式传输
	h.handleStreamingRequest(c, userTokenInfo, proxyReq, startTime, needsStatsAndRateLimit, isSubscriptionInfo)
}

// UserTokenValidationError 用户令牌验证错误类型
type UserTokenValidationError struct {
	Type    string // "not_found", "disabled", "expired", "max_requests", "cache_error"
	Message string
	Err     error
}

// SessionEventsData TOKEN的模拟会话事件数据结构
type SessionEventsData struct {
	ClientName string             `json:"client_name"`
	Events     []SessionEventItem `json:"events"`
	ProjectID  string             `json:"project_id"` // 项目ID
	FileCount  int                `json:"file_count"` // 文件数量
	Theme      string             `json:"theme"`      // 主题
	FontSize   string             `json:"font_size"`  // 字体大小
	Language   string             `json:"language"`   // 语言
	UserAgent  string             `json:"user_agent"` // 用户代理
}

// SessionEventItem 会话事件项
type SessionEventItem struct {
	Time  string                 `json:"time"`
	Event map[string]interface{} `json:"event"`
}

// FeatureVectorData TOKEN的模拟特征向量数据结构
type FeatureVectorData struct {
	ClientName    string            `json:"client_name"`
	FeatureVector map[string]string `json:"feature_vector"`
	UserAgent     string            `json:"user_agent"` // 用户代理
}

func (e *UserTokenValidationError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// validateUserToken 验证用户令牌（使用 User.ApiToken）
func (h *ProxyHandler) validateUserToken(ctx context.Context, tokenStr string) (*service.UserApiTokenInfo, *UserTokenValidationError) {
	// 从数据库验证API令牌
	userTokenInfo, err := h.userAuthService.ValidateApiToken(ctx, tokenStr)
	if err != nil {
		return nil, &UserTokenValidationError{
			Type:    "not_found",
			Message: "User token not found",
			Err:     err,
		}
	}

	// 检查令牌是否有效
	if !userTokenInfo.IsActive() {
		if userTokenInfo.Status != "active" {
			return userTokenInfo, &UserTokenValidationError{
				Type:    "disabled",
				Message: "User token is disabled",
			}
		}
		if userTokenInfo.ExpiresAt != nil && userTokenInfo.ExpiresAt.Before(time.Now()) {
			return userTokenInfo, &UserTokenValidationError{
				Type:    "expired",
				Message: "User token is expired",
			}
		}
	}

	return userTokenInfo, nil
}

// selectAvailableToken - 方法已移至 proxy_token.go

// selectAvailableTokenWithLimit - 方法已移至 proxy_token.go

// getTokenAssignmentCount - 方法已移至 proxy_token.go

// tryAssignTokenWithLimit - 方法已移至 proxy_token.go

// selectLeastUsedToken - 方法已移至 proxy_token.go

// selectTokenWithStrongRandom - 方法已移至 proxy_token.go

// selectTokenWithUserPriority - 方法已移至 proxy_token.go

// getAssignedTokenForUser - 方法已移至 proxy_token.go

// handleUserTokenProxyError 处理用户令牌代理错误
func (h *ProxyHandler) handleUserTokenProxyError(
	c *gin.Context,
	userTokenInfo *service.UserApiTokenInfo,
	proxyReq *proxy.ProxyRequest,
	err error,
) {
	// 打印502错误的详细信息
	logger.Errorf("=== 代理错误详情 ===")
	logger.Errorf("用户令牌: %s...", userTokenInfo.Token[:min(8, len(userTokenInfo.Token))])
	logger.Errorf("请求地址: %s%s", proxyReq.TenantAddress, proxyReq.Path)
	logger.Errorf("请求方法: %s", proxyReq.Method)
	logger.Errorf("使用的TOKEN: %s...", proxyReq.Token.Token[:min(8, len(proxyReq.Token.Token))])
	logger.Errorf("TOKEN代理: %s", h.getTokenProxyInfo(proxyReq.Token))
	logger.Errorf("错误: %v", err)
	logger.Errorf("错误类型分析:")
	if strings.Contains(err.Error(), "EOF") {
		logger.Errorf("  - EOF错误: 连接被远程服务器关闭")
		logger.Errorf("  - 可能原因: 网络超时、服务器负载过高、代理问题")
	}
	if strings.Contains(err.Error(), "timeout") {
		logger.Errorf("  - 超时错误: 请求超时")
	}
	if strings.Contains(err.Error(), "connection refused") {
		logger.Errorf("  - 连接拒绝: 远程服务器拒绝连接")
	}
	logger.Errorf("==================")

	// 只对/chat-stream接口记录错误统计
	if strings.HasPrefix(proxyReq.Path, "/chat-stream") {
		h.recordUserTokenError(userTokenInfo)
	}

	// 返回错误响应
	h.respondError(c, http.StatusBadGateway, "Proxy request failed", err)
}

// recordUserTokenError 记录用户令牌错误（简化版本）
func (h *ProxyHandler) recordUserTokenError(userTokenInfo *service.UserApiTokenInfo) {
	// 异步记录错误统计，减少主流程延迟
	go func() {
		statsCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		// 仅记录错误计数，不记录详细日志
		h.cacheService.IncrementErrorCount(statsCtx, userTokenInfo.Token)
	}()
}

// handleStreamingRequest 处理流式请求
func (h *ProxyHandler) handleStreamingRequest(
	c *gin.Context,
	userTokenInfo *service.UserApiTokenInfo,
	proxyReq *proxy.ProxyRequest,
	startTime time.Time,
	needsStatsAndRateLimit bool,
	isSubscriptionInfo bool,
) {
	requestID, _ := c.Get("request_id")
	userToken := userTokenInfo.Token

	logger.Infof("[代理] 处理流式请求 %s -> %s%s",
		userToken[:min(8, len(userToken))],
		proxyReq.TenantAddress,
		proxyReq.Path)

	// 使用流式转发并捕获响应内容
	capturedContent, err := h.proxyService.ForwardStreamWithCapture(c.Request.Context(), proxyReq, c.Writer)
	if err != nil {
		// 检查是否是客户端断开连接导致的错误，这种情况不算失败
		if errors.Is(err, context.Canceled) ||
			strings.Contains(err.Error(), "context canceled") ||
			strings.Contains(err.Error(), "broken pipe") ||
			strings.Contains(err.Error(), "connection reset") {
			logger.Infof("[代理] 客户端取消流式传输 - ID: %v, 耗时: %v",
				requestID, time.Since(startTime))
			return // 客户端断开不算错误
		}

		// 检查是否是401、402或403错误，如果是，需要记录封号信息和处理TOKEN封禁
		if strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "402") || strings.Contains(err.Error(), "403") {
			logger.Infof("[代理] 检测到401/402/403错误，TOKEN可能被封禁: %v\n", err)

			// 只对401错误尝试刷新TOKEN
			if strings.Contains(err.Error(), "401") && proxyReq.Token != nil {
				logger.Infof("[代理] 检测到401响应，尝试通过AuthSession刷新TOKEN")

				// 尝试刷新TOKEN
				refreshed, refreshErr := h.tryRefreshTokenByAuthSession(c.Request.Context(), proxyReq.Token)
				if refreshed {
					logger.Infof("[代理] ✅ AuthSession刷新TOKEN成功，TOKEN继续使用")
					// 刷新成功，不禁用TOKEN，直接返回
					return
				}

				// 刷新失败，记录日志并继续禁用流程
				if refreshErr != nil {
					logger.Infof("[代理] ❌ AuthSession刷新TOKEN失败: %v，继续禁用TOKEN", refreshErr)
				}
			}

			// 记录封号信息
			if proxyReq.Token != nil {
				banReason := fmt.Sprintf("HTTP %s 错误", extractStatusCode(err.Error()))

				// 如果是401且尝试过刷新，更新描述
				if strings.Contains(err.Error(), "401") && proxyReq.Token.AuthSession != "" {
					banReason = "检测到401响应，AuthSession刷新失败，自动禁用"
				}

				// 如果是402错误，设置特定的封禁原因
				if strings.Contains(err.Error(), "402") {
					banReason = "账号订阅已失效(提示需要付费)"
				}

				go h.recordBanInfo(c, proxyReq.Token, userTokenInfo, banReason, err.Error())

				// 异步处理TOKEN禁用和用户令牌重新分配
				go h.handleTokenBanAndReassignment(proxyReq.Token, userTokenInfo, banReason)
			}

			// 直接返回远程服务器的响应，不再返回模拟响应
			logger.Infof("[代理] TOKEN被封禁，直接返回远程服务器响应")
			return
		}

		// 检查是否是EOF错误，如果是，使用相同TOKEN重试
		if strings.Contains(err.Error(), "EOF") {
			logger.Infof("[代理] 检测到EOF错误，使用相同TOKEN重试: %v", err)

			// 对于EOF错误，使用相同TOKEN直接重试，不需要重新分配
			capturedContent, retryErr := h.proxyService.ForwardStreamWithCapture(c.Request.Context(), proxyReq, c.Writer)
			if retryErr == nil {
				logger.Infof("[代理] EOF重试成功")

				// 检测封禁响应并处理TOKEN禁用和重新分配（仅对/chat-stream接口）
				if strings.HasSuffix(proxyReq.Path, "/chat-stream") && len(capturedContent) > 0 && proxyReq.Token != nil {
					if h.detectAndHandleBannedResponse(capturedContent, userTokenInfo, proxyReq.Token, c) {
						logger.Infof("[代理] EOF重试成功后检测到封禁响应，已处理TOKEN禁用和重新分配")
					}
				}

				// 重试成功（计费已在请求开始时处理，不需要额外记录）
				return // 重试成功
			}

			logger.Infof("[代理] EOF重试失败: %v", retryErr)
			// 如果重试仍然失败，继续执行原有错误处理逻辑
		}

		logger.Infof("[代理] 流式转发失败: %v", err)
		h.handleUserTokenProxyError(c, userTokenInfo, proxyReq, err)
		return
	}

	// 记录流式请求完成
	logger.Infof("[代理] 流式传输成功完成 - ID: %v, 总耗时: %v",
		requestID, time.Since(startTime))

	// 检测封禁响应并处理TOKEN禁用和重新分配（仅对/chat-stream接口）
	if strings.HasSuffix(proxyReq.Path, "/chat-stream") && len(capturedContent) > 0 && proxyReq.Token != nil {
		if h.detectAndHandleBannedResponse(capturedContent, userTokenInfo, proxyReq.Token, c) {
			logger.Infof("[代理] 检测到封禁响应，已处理TOKEN禁用和重新分配")
		}
	}

	// 注意：计费逻辑已改为基于请求数据，在请求开始时执行
	// 流式请求成功完成，不需要额外记录（计费请求已在请求开始时记录）

	// 异步处理 subscription-info 响应，更新TOKEN过期时间
	if isSubscriptionInfo && len(capturedContent) > 0 && proxyReq.Token != nil {
		go h.asyncUpdateTokenExpiryFromResponse(proxyReq.Token, capturedContent)
	}
}

// validateUserTokenFast 快速验证用户令牌
// 注意：现在直接使用 User.ApiToken 验证，缓存逻辑已简化
func (h *ProxyHandler) validateUserTokenFast(ctx context.Context, userToken string) (*service.UserApiTokenInfo, *UserTokenValidationError) {
	// 直接调用验证方法（User.ApiToken 存储在 users 表中）
	return h.validateUserToken(ctx, userToken)
}

// optimizedLimitCheck 优化的限制检查（仅对特定路径执行）
func (h *ProxyHandler) optimizedLimitCheck(ctx context.Context, userToken string, userTokenInfo *service.UserApiTokenInfo, fullPath string) (bool, string) {
	// 只对特定路径进行限制检查
	if !strings.HasSuffix(fullPath, "/chat-stream") {
		return true, "ok" // 非chat-stream路径跳过限制检查
	}

	// 检查缓存的限制状态
	cachedStatus, hit, err := h.limitResponseHandler.GetCachedUserTokenStatus(ctx, userToken)
	if err != nil {
		logger.Warnf("警告: 获取缓存令牌状态失败: %v", err)
	}

	if hit && cachedStatus != "" {
		if cachedStatus == "ok" {
			return true, cachedStatus
		}
		return false, cachedStatus
	}

	// 缓存未命中，进行实际检查
	allowed, limitType, err := h.limitResponseHandler.CheckUserTokenLimits(ctx, userTokenInfo)
	if err != nil {
		logger.Warnf("警告: 限制检查失败: %v\n", err)
		return true, "ok" // 检查失败时允许通过，避免影响可用性
	}

	// 缓存结果
	var cacheTTL time.Duration
	if limitType == "disabled" {
		cacheTTL = time.Hour * 24
	} else if limitType == "ok" {
		cacheTTL = time.Minute * 5 // 正常状态缓存时间延长
	} else {
		cacheTTL = time.Minute * 2 // 频率限制缓存时间缩短，与1分钟窗口保持一致
	}

	if err := h.limitResponseHandler.CacheUserTokenStatus(ctx, userToken, limitType, cacheTTL); err != nil {
		logger.Warnf("警告: 缓存令牌状态失败: %v\n", err)
	}

	return allowed, limitType
}

// isBlacklistedPath 检查请求路径是否在黑名单中
func (h *ProxyHandler) isBlacklistedPath(fullPath string) bool {
	// 定义黑名单路径列表
	blacklistPaths := []string{
		"/client-metrics", // 客户端指标上报
		"/report-error",   // 错误报告
	}

	// 移除查询参数，只检查路径部分
	pathOnly := fullPath
	if queryIndex := strings.Index(fullPath, "?"); queryIndex != -1 {
		pathOnly = fullPath[:queryIndex]
	}

	// 检查是否匹配黑名单中的任何路径
	for _, blacklistedPath := range blacklistPaths {
		// 对于以 /** 结尾的路径，使用通配符前缀匹配
		if strings.HasSuffix(blacklistedPath, "/**") {
			prefix := strings.TrimSuffix(blacklistedPath, "/**")
			if strings.HasPrefix(pathOnly, prefix) {
				return true
			}
		} else if strings.HasSuffix(blacklistedPath, "/") {
			// 对于以 / 结尾的路径，使用前缀匹配
			if strings.HasPrefix(pathOnly, blacklistedPath) {
				return true
			}
		} else {
			// 对于其他路径，检查是否包含该路径
			if strings.Contains(pathOnly, blacklistedPath) {
				return true
			}
		}
	}

	return false
}

// generateRandomBytes 生成随机字节
func (sb *SignatureBuilder) generateRandomBytes(length int) ([]byte, error) {
	bytes := make([]byte, length)
	_, err := rand.Read(bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return bytes, nil
}

// base64URLEncode Base64 URL安全编码
func (sb *SignatureBuilder) base64URLEncode(data []byte) string {
	encoded := base64.StdEncoding.EncodeToString(data)
	encoded = strings.ReplaceAll(encoded, "+", "-")
	encoded = strings.ReplaceAll(encoded, "/", "_")
	encoded = strings.ReplaceAll(encoded, "=", "")
	return encoded
}

// sha256Hash SHA-256哈希计算
func (sb *SignatureBuilder) sha256Hash(data []byte) []byte {
	hash := sha256.Sum256(data)
	return hash[:]
}

// BuildSignature 构建签名
func (sb *SignatureBuilder) BuildSignature(apiEndpoint string, requestData []byte) (*SignatureResult, error) {
	// 1. 生成时间戳
	timestamp := time.Now().UnixMilli()

	// 2. 生成随机向量
	vectorBytes, err := sb.generateRandomBytes(32)
	if err != nil {
		return &SignatureResult{
			FailureReason: fmt.Sprintf("Failed to generate random vector: %v", err),
		}, nil
	}
	vector := sb.base64URLEncode(vectorBytes)

	// 3. 组装签名数据
	signatureData := fmt.Sprintf("%s|%d|%s", apiEndpoint, timestamp, vector)
	if requestData != nil {
		signatureData += "|" + hex.EncodeToString(requestData)
	}

	// 4. 计算签名
	signatureHash := sb.sha256Hash([]byte(signatureData))
	signature := sb.base64URLEncode(signatureHash)

	return &SignatureResult{
		SignatureVersion: sb.version,
		Timestamp:        timestamp,
		Signature:        signature,
		Vector:           vector,
	}, nil
}

// ValidateEndpoint 验证端点是否需要签名
func (sb *SignatureBuilder) ValidateEndpoint(endpoint string) bool {
	validEndpoints := []string{"chat-stream", "remote-agents/create"}
	for _, validEndpoint := range validEndpoints {
		if strings.Contains(endpoint, validEndpoint) {
			return true
		}
	}
	return false
}

// addSignatureHeadersIfNeeded 检查是否需要添加签名请求头
func (h *ProxyHandler) addSignatureHeadersIfNeeded(proxyReq *proxy.ProxyRequest, fullPath string, body []byte) {
	// 检查是否存在需要签名的请求头标识
	// 根据文档，当客户端设置了特定的特性标志时，会需要签名
	if !h.shouldAddSignature(proxyReq.Headers, fullPath) {
		return
	}

	// 提取API端点名称（移除查询参数）
	apiEndpoint := fullPath
	if queryIndex := strings.Index(fullPath, "?"); queryIndex != -1 {
		apiEndpoint = fullPath[:queryIndex]
	}

	// 验证端点是否需要签名
	if !h.signatureBuilder.ValidateEndpoint(apiEndpoint) {
		logger.Infof("[代理] 端点 %s 不需要签名，跳过签名生成\n", apiEndpoint)
		return
	}

	// 生成签名
	signatureResult, err := h.signatureBuilder.BuildSignature(apiEndpoint, body)
	if err != nil {
		logger.Infof("[代理] 生成签名失败: %v\n", err)
		return
	}

	// 添加签名请求头
	proxyReq.Headers.Set("x-signature-version", signatureResult.SignatureVersion)
	proxyReq.Headers.Set("x-signature-timestamp", fmt.Sprintf("%d", signatureResult.Timestamp))
	proxyReq.Headers.Set("x-signature-signature", signatureResult.Signature)
	proxyReq.Headers.Set("x-signature-vector", signatureResult.Vector)

	logger.Infof("[代理] 已为端点 %s 添加签名请求头\n", apiEndpoint)
}

// shouldAddSignature 检查是否应该添加签名
func (h *ProxyHandler) shouldAddSignature(headers http.Header, fullPath string) bool {
	// 第一步：检查当前接口是否需要防欺诈签名
	if !h.signatureBuilder.ValidateEndpoint(fullPath) {
		// 如果接口不需要签名，直接返回false
		return false
	}

	// 第二步：检查客户端是否发送了签名相关的请求头
	// 如果客户端发送了签名请求头，说明客户端尝试进行签名验证，需要重新构建正确的签名
	hasSignatureHeaders := headers.Get("x-signature-version") != "" ||
		headers.Get("x-signature-timestamp") != "" ||
		headers.Get("x-signature-signature") != "" ||
		headers.Get("x-signature-vector") != ""

	// 只有当接口需要签名且客户端发送了签名请求头时，才需要重新构建签名
	return hasSignatureHeaders
}

// getTokenProxyInfo 获取TOKEN代理信息
func (h *ProxyHandler) getTokenProxyInfo(token *database.Token) string {
	if token == nil {
		return "未知"
	}
	if token.ProxyURL == nil || *token.ProxyURL == "" {
		return "直连"
	}
	return *token.ProxyURL
}

// asyncUpdateTokenExpiryFromResponse 异步更新TOKEN过期时间
func (h *ProxyHandler) asyncUpdateTokenExpiryFromResponse(token *database.Token, responseData []byte) {
	// 解析响应内容，提取过期时间
	h.updateTokenExpiryFromResponse(token, responseData)
}

// updateTokenExpiryFromResponse 从响应中提取过期时间并更新TOKEN
func (h *ProxyHandler) updateTokenExpiryFromResponse(token *database.Token, responseData []byte) {
	// 所有TOKEN都是积分账号，不再刷新过期时间
	logger.Infof("[代理] TOKEN %s... 为积分账号，跳过过期时间刷新\n",
		token.Token[:min(8, len(token.Token))])
}

// handleSubscriptionBannerRequest 处理 /subscription-banner 请求
// 转发请求到远程服务器，但返回空JSON给客户端
func (h *ProxyHandler) handleSubscriptionBannerRequest(c *gin.Context, token *database.Token, fullPath string, body []byte) {
	// 构建代理请求
	proxyReq := &proxy.ProxyRequest{
		Token:         token,
		Method:        c.Request.Method,
		Path:          fullPath,
		Headers:       c.Request.Header,
		Body:          body,
		ClientIP:      proxy.GetClientIP(c.Request),
		UserAgent:     c.Request.UserAgent(),
		TenantAddress: token.TenantAddress,
		SessionID:     token.SessionID,
	}

	// 异步转发请求到远程服务器（不等待响应）
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// 使用 ForwardStreamWithCapture 转发请求，丢弃响应
		_, err := h.proxyService.ForwardStreamWithCapture(ctx, proxyReq, &discardResponseWriter{})
		if err != nil {
			logger.Debugf("[代理] /subscription-banner 转发失败（忽略）: %v", err)
		}
	}()

	// 直接返回空JSON给客户端
	c.JSON(http.StatusOK, gin.H{})
}
