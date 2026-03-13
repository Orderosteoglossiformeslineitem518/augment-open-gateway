package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"augment-gateway/internal/database"
	"augment-gateway/internal/logger"
	"augment-gateway/internal/proxy"
	"augment-gateway/internal/service"
)

// ============================================================================
// 计费逻辑
// ============================================================================

// handleRequestBasedBilling 处理基于请求数据的计费逻辑
// 只有当请求体中的 nodes 数组的第一个节点的 id 等于 0 时，才计为一次需要计费的对话
func (h *ProxyHandler) handleRequestBasedBilling(
	ctx context.Context,
	body []byte,
	userTokenInfo *service.UserApiTokenInfo,
	proxyReq *proxy.ProxyRequest,
	startTime time.Time,
) {
	// 如果请求体为空，不进行计费
	if len(body) == 0 {
		return
	}

	// 解析JSON请求体
	var requestData map[string]interface{}
	if err := json.Unmarshal(body, &requestData); err != nil {
		logger.Infof("[代理-请求计费] 解析JSON请求体失败: %v，跳过基于请求的计费检查\n", err)
		return
	}

	// 检查是否需要同步积分账号的额度（基于message关键字）
	h.checkAndSyncCreditAccountQuota(ctx, requestData, proxyReq)

	// 兜底方案：基于请求次数的积分同步（仅对积分账号）
	h.checkAndSyncCreditAccountByRequestCount(ctx, proxyReq)

	// 检查nodes字段
	nodesInterface, exists := requestData["nodes"]
	if !exists {
		return
	}

	// 将nodes转换为数组
	nodesArray, ok := nodesInterface.([]interface{})
	if !ok {
		return
	}

	if len(nodesArray) == 0 {
		return
	}

	// 获取第一个节点
	firstNode, ok := nodesArray[0].(map[string]interface{})
	if !ok {
		return
	}

	// 检查第一个节点的id字段
	idInterface, exists := firstNode["id"]
	if !exists {
		return
	}

	// 检查第一个节点的id是否等于0
	isFirstNodeIDZero := h.isIDEqualToZero(idInterface)
	if isFirstNodeIDZero {
		logger.Infof("[代理-请求计费] 检测到nodes数组第一个节点id=0，触发基于请求的计费\n")
		h.executeRequestBasedBilling(ctx, userTokenInfo, proxyReq, startTime)
	} else {
		logger.Infof("[代理-请求计费] nodes数组第一个节点id不等于0 (id=%v)，跳过基于请求的计费\n", idInterface)
	}
}

// executeRequestBasedBilling 执行基于请求的计费逻辑
// 异步处理，不影响正常的请求转发流程
func (h *ProxyHandler) executeRequestBasedBilling(
	ctx context.Context,
	userTokenInfo *service.UserApiTokenInfo,
	proxyReq *proxy.ProxyRequest,
	startTime time.Time,
) {
	// 异步执行计费逻辑，避免影响正常请求转发
	go func() {
		// 创建独立的上下文，设置超时时间
		billingCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// 1. 更新用户令牌使用次数
		if err := h.userAuthService.IncrementApiTokenUsage(billingCtx, userTokenInfo.ID); err != nil {
			logger.Infof("[代理-请求计费] 警告: 更新用户令牌使用次数失败: %v\n", err)
		} else {
			logger.Infof("[代理-请求计费] 用户令牌 %s... 使用次数已更新\n",
				userTokenInfo.Token[:min(8, len(userTokenInfo.Token))])
			// 清除缓存的令牌状态
			h.limitResponseHandler.ClearUserTokenStatusCache(billingCtx, userTokenInfo.Token)
		}

		// 2. 更新系统TOKEN使用次数
		// 注意：所有TOKEN都是积分账号，不通过本地计费，仅通过/get-credit-info接口同步
		if proxyReq.Token != nil && proxyReq.Token.ID != "" {
			logger.Infof("[代理-请求计费] 跳过积分账号 %s... 的本地计费，仅通过/get-credit-info接口同步\n",
				proxyReq.Token.Token[:min(8, len(proxyReq.Token.Token))])
		}

		// 3. 记录请求日志
		// 创建一个虚拟的响应对象用于日志记录
		virtualResp := &proxy.ProxyResponse{
			StatusCode: 200, // 基于请求的计费，假设请求成功
			Headers:    make(http.Header),
			Body:       []byte{}, // 空响应体
			Size:       0,
			Latency:    time.Since(startTime),
		}

		// 获取系统TOKEN ID
		var systemTokenID string
		if proxyReq.Token != nil {
			systemTokenID = proxyReq.Token.ID
		}

		// 记录请求日志
		userTokenIDStr := fmt.Sprintf("%d", userTokenInfo.ID)
		requestLog := h.proxyService.LogUserTokenRequest(userTokenIDStr, systemTokenID, proxyReq, virtualResp, nil)
		if err := h.statsService.LogRequest(billingCtx, requestLog); err != nil {
			logger.Infof("[代理-请求计费] 警告: 记录请求日志失败: %v\n", err)
		} else {
			logger.Infof("[代理-请求计费] 请求日志已记录")
		}

		// 4. 记录用户使用统计（通过API令牌查询用户）
		if h.userUsageStatsService != nil {
			if err := h.userUsageStatsService.IncrementUserStatsByToken(userTokenInfo.Token, true, 1); err != nil {
				logger.Infof("[代理-请求计费] 警告: 记录用户使用统计失败: %v\n", err)
			}
		}

		logger.Infof("[代理-请求计费] 基于请求的计费逻辑执行完成\n")
	}()
}

// logEnhancedProxyRequest 记录增强代理请求日志
func (h *ProxyHandler) logEnhancedProxyRequest(ctx context.Context, userTokenInfo *service.UserApiTokenInfo, proxyReq *proxy.ProxyRequest, startTime time.Time, channelID uint, channelName string, err error) {
	// 构建虚拟响应用于日志记录
	virtualResp := &proxy.ProxyResponse{
		StatusCode: http.StatusOK,
		Latency:    time.Since(startTime),
	}

	// 设置错误信息：标记为外部渠道请求
	if err != nil {
		virtualResp.ErrorMessage = fmt.Sprintf("外部渠道[%s]: %v", channelName, err)
		virtualResp.StatusCode = http.StatusBadGateway
	} else {
		virtualResp.ErrorMessage = fmt.Sprintf("外部渠道[%s]", channelName)
	}

	// 获取系统TOKEN ID
	var systemTokenID string
	if proxyReq.Token != nil {
		systemTokenID = proxyReq.Token.ID
	}

	// 记录请求日志（带外部渠道ID）
	userTokenIDStr := fmt.Sprintf("%d", userTokenInfo.ID)
	requestLog := h.proxyService.LogUserTokenRequest(userTokenIDStr, systemTokenID, proxyReq, virtualResp, &channelID)
	if err := h.statsService.LogRequest(ctx, requestLog); err != nil {
		logger.Infof("[增强代理] 警告: 记录请求日志失败: %v\n", err)
	} else {
		logger.Infof("[增强代理] 请求日志已记录 (外部渠道: %s, ID: %d)", channelName, channelID)
	}

	// 记录用户使用统计
	if h.userUsageStatsService != nil {
		if err := h.userUsageStatsService.IncrementUserStatsByToken(userTokenInfo.Token, err == nil, 1); err != nil {
			logger.Infof("[增强代理] 警告: 记录用户使用统计失败: %v\n", err)
		}
	}
}

// ============================================================================
// ID 检查辅助方法
// ============================================================================

// isIDEqualToZero 检查id是否等于0
func (h *ProxyHandler) isIDEqualToZero(id interface{}) bool {
	switch v := id.(type) {
	case int:
		return v == 0
	case int64:
		return v == 0
	case float64:
		return v == 0
	case string:
		return v == "0"
	default:
		return false
	}
}

// isIDEqualToOne 检查id是否等于1
func (h *ProxyHandler) isIDEqualToOne(id interface{}) bool {
	switch v := id.(type) {
	case int:
		return v == 1
	case int64:
		return v == 1
	case float64:
		return v == 1
	case string:
		return v == "1"
	default:
		return false
	}
}

// ============================================================================
// 积分同步逻辑
// ============================================================================

// checkAndSyncCreditAccountQuota 检查并同步积分账号的额度（大于等于4000的积分账号）
func (h *ProxyHandler) checkAndSyncCreditAccountQuota(ctx context.Context, requestData map[string]interface{}, proxyReq *proxy.ProxyRequest) {
	// 检查TOKEN是否为积分账号（最大使用次数大于等于4000）
	if proxyReq.Token == nil || proxyReq.Token.MaxRequests < 4000 {
		return
	}

	// 检查message字段
	messageInterface, exists := requestData["message"]
	if !exists {
		return
	}

	messageStr, ok := messageInterface.(string)
	if !ok {
		return
	}

	// 检查message是否包含特定文本
	targetMessage := "IN THIS MODE YOU ONLY ANALYZE THE MESSAGE AND DECIDE IF IT HAS INFORMATION WORTH REMEMBERING\n# YOU DON'T USE TOOLS OR ANSWER TO THE NEXT MESSAGE ITSELF"
	otherTargetMessage := "Please provide a clear and concise summary of our conversation so far. The summary must be less than 6 words long."
	if !strings.Contains(messageStr, targetMessage) || !strings.Contains(messageStr, otherTargetMessage) {
		return
	}

	logger.Infof("[代理-积分同步] 检测到积分账号触发额度同步请求，TOKEN: %s... (max_requests=%d)\n",
		proxyReq.Token.Token[:min(8, len(proxyReq.Token.Token))], proxyReq.Token.MaxRequests)

	// 异步调用/get-credit-info接口同步额度
	go h.asyncSyncCreditAccountQuota(proxyReq.Token)
}

// checkAndSyncCreditAccountByRequestCount 基于请求次数的兜底积分同步方案
// 当同一个系统TOKEN连续调用/chat-stream接口超过2次时，触发一次主动调用/get-credit-info接口同步积分数据
func (h *ProxyHandler) checkAndSyncCreditAccountByRequestCount(ctx context.Context, proxyReq *proxy.ProxyRequest) {
	// 检查TOKEN是否为积分账号（最大使用次数大于等于4000）
	if proxyReq.Token == nil || proxyReq.Token.MaxRequests < 4000 {
		return
	}

	// 递增计数器
	counter, err := h.cacheService.IncrementCreditSyncCounter(ctx, proxyReq.Token.ID)
	if err != nil {
		logger.Infof("[代理-兜底同步] 递增积分同步计数器失败: %v\n", err)
		return
	}

	logger.Infof("[代理-兜底同步] TOKEN %s... 当前/chat-stream请求计数: %d/2\n",
		proxyReq.Token.Token[:min(8, len(proxyReq.Token.Token))], counter)

	// 如果计数器达到2次，触发积分同步
	if counter >= 2 {
		logger.Infof("[代理-兜底同步] TOKEN %s... 达到2次请求阈值，触发兜底积分同步\n",
			proxyReq.Token.Token[:min(8, len(proxyReq.Token.Token))])

		// 异步执行积分同步和计数器重置
		go func(token *database.Token, tokenID string) {
			// 调用积分同步方法
			h.asyncSyncCreditAccountQuota(token)

			// 同步成功后重置计数器
			// 延迟一段时间确保同步完成
			time.Sleep(10 * time.Second)

			resetCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := h.cacheService.ResetCreditSyncCounter(resetCtx, tokenID); err != nil {
				logger.Infof("[代理-兜底同步] 重置积分同步计数器失败: %v\n", err)
			} else {
				logger.Infof("[代理-兜底同步] TOKEN %s... 积分同步计数器已重置\n",
					token.Token[:min(8, len(token.Token))])
			}
		}(proxyReq.Token, proxyReq.Token.ID)
	}
}

// asyncSyncCreditAccountQuota 异步同步积分账号的额度
func (h *ProxyHandler) asyncSyncCreditAccountQuota(token *database.Token) {
	// 延迟3-5秒后执行
	delaySeconds := 3 + time.Duration(time.Now().UnixNano()%3)
	time.Sleep(delaySeconds * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 构建请求
	url := token.TenantAddress + "get-credit-info"
	requestBody := []byte("{}")

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(requestBody))
	if err != nil {
		logger.Infof("[代理-积分同步] 创建请求失败: %v，尝试兜底方案\n", err)
		h.fallbackSyncFromPortal(ctx, token)
		return
	}

	// 提取主机名
	host := h.extractHostFromTenantAddress(token.TenantAddress)

	// 添加请求头（与 subscription_validator.go 保持一致）
	req.Header.Set("host", host)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", h.config.Subscription.UserAgent)
	req.Header.Set("x-request-id", h.generateRequestID())
	req.Header.Set("x-request-session-id", token.SessionID)
	req.Header.Set("x-api-version", "2")
	req.Header.Set("Authorization", "Bearer "+token.Token)
	req.Header.Set("accept", "*/*")
	req.Header.Set("accept-language", "*")
	req.Header.Set("sec-fetch-mode", "cors")

	// 发送请求
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		logger.Infof("[代理-积分同步] 请求失败: %v，尝试兜底方案\n", err)
		h.fallbackSyncFromPortal(ctx, token)
		return
	}
	defer resp.Body.Close()

	// 读取响应
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Infof("[代理-积分同步] 读取响应失败: %v，尝试兜底方案\n", err)
		h.fallbackSyncFromPortal(ctx, token)
		return
	}

	// 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		logger.Infof("[代理-积分同步] 请求失败，状态码: %d, 响应: %s，尝试兜底方案\n", resp.StatusCode, string(responseBody))
		h.fallbackSyncFromPortal(ctx, token)
		return
	}

	// 解析响应数据
	var creditInfo struct {
		UsageUnitsTotal     float64 `json:"usage_units_total"`
		UsageUnitsRemaining float64 `json:"usage_units_remaining"`
	}

	if err := json.Unmarshal(responseBody, &creditInfo); err != nil {
		logger.Infof("[代理-积分同步] 解析响应失败: %v，尝试兜底方案\n", err)
		h.fallbackSyncFromPortal(ctx, token)
		return
	}

	// 计算已使用额度
	usedRequests := int(creditInfo.UsageUnitsTotal - creditInfo.UsageUnitsRemaining)
	maxRequests := int(creditInfo.UsageUnitsTotal)

	logger.Infof("[代理-积分同步] 获取到积分信息 - 总额度: %d, 剩余: %.0f, 已使用: %d\n",
		maxRequests, creditInfo.UsageUnitsRemaining, usedRequests)

	// 只更新used_requests字段，不更新max_requests（避免最大使用次数随请求次数增加）
	if err := h.tokenService.UpdateUsedRequests(ctx, token.ID, usedRequests); err != nil {
		logger.Infof("[代理-积分同步] 更新used_requests失败: %v\n", err)
		return
	}

	logger.Infof("[代理-积分同步] ✅ 成功同步TOKEN %s... 的已使用次数 (已使用: %d, 最大次数保持不变: %d)\n",
		token.Token[:min(8, len(token.Token))], usedRequests, token.MaxRequests)
}

// fallbackSyncFromPortal 从 Portal URL 同步积分（兜底方案）
func (h *ProxyHandler) fallbackSyncFromPortal(ctx context.Context, token *database.Token) {
	logger.Infof("[代理-兜底同步] 开始使用 Portal URL 兜底同步积分，TOKEN: %s...\n",
		token.Token[:min(8, len(token.Token))])

	// 创建 Orb 同步客户端
	orbClient := NewOrbCreditSyncClient()

	// 从 Portal 同步积分
	usedRequests, maxRequests, err := orbClient.SyncCreditFromPortal(ctx, token)
	if err != nil {
		logger.Infof("[代理-兜底同步] Portal 同步失败: %v\n", err)
		return
	}

	logger.Infof("[代理-兜底同步] Portal 同步成功 - 总额度: %d, 已使用: %d\n", maxRequests, usedRequests)

	// 更新数据库中的使用次数
	if err := h.tokenService.UpdateUsedRequests(ctx, token.ID, usedRequests); err != nil {
		logger.Infof("[代理-兜底同步] 更新数据库失败: %v\n", err)
		return
	}

	logger.Infof("[代理-兜底同步] ✅ 成功通过 Portal 同步 TOKEN %s... 的已使用次数 (已使用: %d)\n",
		token.Token[:min(8, len(token.Token))], usedRequests)
}

// fallbackVerifyCreditFromPortal 从 Portal URL 验证积分是否耗尽（兜底方案）
// 返回值：(是否耗尽, 已使用次数, 最大次数)
func (h *ProxyHandler) fallbackVerifyCreditFromPortal(ctx context.Context, token *database.Token) (bool, int, int) {
	logger.Infof("[代理-兜底验证] 开始使用 Portal URL 兜底验证积分，TOKEN: %s...\n",
		token.Token[:min(8, len(token.Token))])

	// 创建 Orb 同步客户端
	orbClient := NewOrbCreditSyncClient()

	// 从 Portal 同步积分
	usedRequests, maxRequests, err := orbClient.SyncCreditFromPortal(ctx, token)
	if err != nil {
		logger.Infof("[代理-兜底验证] Portal 验证失败: %v\n", err)
		//兜底验证失败，默认返回未耗尽
		return false, 0, 0
	}

	// 计算剩余积分
	remainingBalance := float64(maxRequests - usedRequests)

	logger.Infof("[代理-兜底验证] Portal 验证成功 - 总额度: %d, 已使用: %d, 剩余: %.0f\n",
		maxRequests, usedRequests, remainingBalance)

	// 判断是否耗尽（剩余额度小于等于0）
	isExhausted := remainingBalance <= 0

	return isExhausted, usedRequests, maxRequests
}

// verifyCreditExhaustion 同步验证积分是否真的耗尽
// 返回值：(是否耗尽, 已使用次数, 最大次数)
func (h *ProxyHandler) verifyCreditExhaustion(token *database.Token) (bool, int, int) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 构建请求
	url := token.TenantAddress + "get-credit-info"
	requestBody := []byte("{}")

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(requestBody))
	if err != nil {
		logger.Infof("[代理-积分验证] 创建请求失败: %v，尝试兜底验证\n", err)
		return h.fallbackVerifyCreditFromPortal(ctx, token)
	}

	// 提取主机名
	host := h.extractHostFromTenantAddress(token.TenantAddress)

	// 添加请求头（与 subscription_validator.go 保持一致）
	req.Header.Set("host", host)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", h.config.Subscription.UserAgent)
	req.Header.Set("x-request-id", h.generateRequestID())
	req.Header.Set("x-request-session-id", token.SessionID)
	req.Header.Set("x-api-version", "2")
	req.Header.Set("Authorization", "Bearer "+token.Token)
	req.Header.Set("accept", "*/*")
	req.Header.Set("accept-language", "*")
	req.Header.Set("sec-fetch-mode", "cors")

	// 发送请求
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		logger.Infof("[代理-积分验证] 请求失败: %v，尝试兜底验证\n", err)
		return h.fallbackVerifyCreditFromPortal(ctx, token)
	}
	defer resp.Body.Close()

	// 读取响应
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Infof("[代理-积分验证] 读取响应失败: %v，尝试兜底验证\n", err)
		return h.fallbackVerifyCreditFromPortal(ctx, token)
	}

	// 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		logger.Infof("[代理-积分验证] 请求失败，状态码: %d, 响应: %s，尝试兜底验证\n", resp.StatusCode, string(responseBody))
		return h.fallbackVerifyCreditFromPortal(ctx, token)
	}

	// 解析响应数据
	var creditInfo struct {
		UsageUnitsTotal     float64 `json:"usage_units_total"`
		UsageUnitsRemaining float64 `json:"usage_units_remaining"`
	}

	if err := json.Unmarshal(responseBody, &creditInfo); err != nil {
		logger.Infof("[代理-积分验证] 解析响应失败: %v，尝试兜底验证\n", err)
		return h.fallbackVerifyCreditFromPortal(ctx, token)
	}

	// 计算已使用额度
	usedRequests := int(creditInfo.UsageUnitsTotal - creditInfo.UsageUnitsRemaining)
	maxRequests := int(creditInfo.UsageUnitsTotal)

	logger.Infof("[代理-积分验证] 获取到积分信息 - 总额度: %d, 剩余: %.0f, 已使用: %d\n",
		maxRequests, creditInfo.UsageUnitsRemaining, usedRequests)

	// 判断是否耗尽（剩余额度小于等于0）
	isExhausted := creditInfo.UsageUnitsRemaining <= 0

	return isExhausted, usedRequests, maxRequests
}

// ============================================================================
// Silent 跳过逻辑
// ============================================================================

// handleSilentSkipLogic 处理基于积分的跳过机制
// 所有积分账号默认都不跳过（即修改silent参数-减少计费）
func (h *ProxyHandler) handleSilentSkipLogic(ctx context.Context, token *database.Token, body []byte) (bool, error) {
	// 只有当请求是计费请求时才进行跳过逻辑检查
	// 非计费请求直接跳过silent参数修改（返回true）
	if !h.isChargingRequest(body) {
		return true, nil
	}

	// 所有积分账号默认不跳过（修改silent参数-减少计费）
	return false, nil
}

// isChargingRequest 检查请求是否为计费请求（基于nodes数组第一个节点ID是否为0）
func (h *ProxyHandler) isChargingRequest(body []byte) bool {
	if len(body) == 0 {
		return false
	}

	var requestData map[string]interface{}
	if err := json.Unmarshal(body, &requestData); err != nil {
		return false
	}

	// 检查nodes数组
	nodesInterface, exists := requestData["nodes"]
	if !exists {
		return false
	}

	nodes, ok := nodesInterface.([]interface{})
	if !ok || len(nodes) == 0 {
		return false
	}

	// 检查第一个节点
	firstNode, ok := nodes[0].(map[string]interface{})
	if !ok {
		return false
	}

	// 检查第一个节点的id字段
	idInterface, exists := firstNode["id"]
	if !exists {
		return false
	}

	// 检查ID是否等于0
	return h.isIDEqualToZero(idInterface)
}

// ============================================================================
// 辅助方法
// ============================================================================

// generateRequestID 生成请求ID
func (h *ProxyHandler) generateRequestID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// extractHostFromTenantAddress 从租户地址中提取主机名
func (h *ProxyHandler) extractHostFromTenantAddress(tenantAddress string) string {
	// 移除协议前缀
	host := strings.TrimPrefix(tenantAddress, "https://")
	host = strings.TrimPrefix(host, "http://")

	// 移除路径部分
	if idx := strings.Index(host, "/"); idx != -1 {
		host = host[:idx]
	}

	return host
}
