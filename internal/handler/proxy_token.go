package handler

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	mathRand "math/rand"
	"os"
	"strings"
	"time"
	"unsafe"

	"augment-gateway/internal/database"
	"augment-gateway/internal/logger"
	"augment-gateway/internal/proxy"
	"augment-gateway/internal/service"

	"github.com/gin-gonic/gin"
)

// ============================================================================
// TOKEN 选择与分配逻辑
// ============================================================================

// selectAvailableToken 选择可用的TOKEN（优先选择使用量少的TOKEN）
func (h *ProxyHandler) selectAvailableToken(ctx context.Context) (*database.Token, error) {
	// 从数据库获取活跃TOKEN
	activeTokens, err := h.tokenService.GetActiveTokens(ctx)
	if err != nil {
		logger.Infof("[代理] 获取活跃令牌错误: %v\n", err)
		return nil, fmt.Errorf("获取活跃令牌失败: %w", err)
	}

	logger.Infof("[代理] 找到 %d 个活跃令牌\n", len(activeTokens))

	if len(activeTokens) == 0 {
		logger.Infof("[代理] 没有可用的活跃令牌\n")
		return nil, fmt.Errorf("没有可用的活跃令牌")
	}

	// 移除使用次数过滤逻辑，直接使用活跃TOKEN
	// 真正达到使用上限时会通过自动封禁机制改变TOKEN状态
	availableTokens := activeTokens

	logger.Infof("[代理] 根据状态过滤后有 %d 个可用令牌\n", len(availableTokens))

	// 选择使用量最少的TOKEN
	selectedToken := h.selectLeastUsedToken(availableTokens)

	// 显示代理信息
	proxyInfo := "直连"
	if selectedToken.ProxyURL != nil && *selectedToken.ProxyURL != "" {
		proxyInfo = *selectedToken.ProxyURL
	}

	logger.Infof("[代理] 选择使用量最少的令牌: %s... (已用: %d/%d, 代理: %s) -> %s\n",
		selectedToken.Token[:min(8, len(selectedToken.Token))],
		selectedToken.UsedRequests,
		selectedToken.MaxRequests,
		proxyInfo,
		selectedToken.TenantAddress)

	return selectedToken, nil
}

// selectAvailableTokenWithLimit 选择可用TOKEN
// preferredTokenID: 优先使用的TOKEN ID（用于保持当前账号不切换），为空时按原逻辑选择
// 优先级：0. 优先检查preferredTokenID对应的TOKEN是否有效
//
//  1. 用户在shared_token_allocations表中分配的共享TOKEN（如果有效）
//  2. 用户自己提交的有效TOKEN
//  3. 其他共享TOKEN（分配数量未超过10个）
func (h *ProxyHandler) selectAvailableTokenWithLimit(ctx context.Context, userToken string, preferredTokenID string) (*database.Token, error) {
	// 获取用户信息
	userTokenInfo, err := h.userAuthService.ValidateApiToken(ctx, userToken)
	if err != nil {
		logger.Infof("[代理] 获取用户令牌信息失败: %v\n", err)
		return nil, fmt.Errorf("获取用户令牌信息失败: %w", err)
	}

	userID := userTokenInfo.UserID
	logger.Infof("[代理] 为用户 %d 选择TOKEN\n", userID)

	// 步骤0：优先检查preferredTokenID对应的TOKEN是否仍然有效（保持当前账号不切换）
	if preferredTokenID != "" {
		tokenInfo, err := h.tokenService.GetToken(ctx, preferredTokenID)
		if err == nil && tokenInfo.Token.Status == "active" {
			logger.Infof("[代理] 优先使用用户当前TOKEN（保持不切换）: %s... -> %s\n",
				tokenInfo.Token.Token[:min(8, len(tokenInfo.Token.Token))],
				tokenInfo.Token.TenantAddress)
			return &tokenInfo.Token, nil
		}
		// preferredToken无效，记录日志并继续选择其他TOKEN
		if err != nil {
			logger.Infof("[代理] 用户当前TOKEN %s 获取失败: %v，将选择其他TOKEN\n", preferredTokenID, err)
		} else {
			logger.Infof("[代理] 用户当前TOKEN %s 状态为 %s（非active），将选择其他TOKEN\n",
				preferredTokenID, tokenInfo.Token.Status)
		}
	}

	// 步骤1：优先使用用户在shared_token_allocations表中分配的共享TOKEN
	if h.sharedTokenService != nil {
		allocatedTokens, err := h.sharedTokenService.GetUserSharedTokens(userID)
		if err == nil && len(allocatedTokens) > 0 {
			// 检查分配的TOKEN是否有效（active且未过期）
			for _, token := range allocatedTokens {
				if token.IsActive() {
					logger.Infof("[代理] 优先使用用户 %d 在shared_token_allocations表中分配的TOKEN: %s...\n",
						userID, token.Token[:min(8, len(token.Token))])
					return &token, nil
				}
			}
			logger.Infof("[代理] 用户 %d 在shared_token_allocations表中分配的TOKEN均无效，继续查找其他TOKEN\n", userID)
		}
	}

	// 步骤2：检查用户自己提交的有效TOKEN
	activeTokens, err := h.tokenService.GetActiveTokens(ctx)
	if err != nil {
		logger.Infof("[代理] 获取活跃令牌失败: %v\n", err)
		return nil, fmt.Errorf("获取活跃令牌失败: %w", err)
	}

	// 查找用户自己提交的TOKEN
	for _, token := range activeTokens {
		if token.SubmitterUserID != nil && *token.SubmitterUserID == userID && token.IsActive() {
			logger.Infof("[代理] 使用用户 %d 自己提交的TOKEN: %s...\n",
				userID, token.Token[:min(8, len(token.Token))])
			return token, nil
		}
	}

	// 步骤3：从共享TOKEN池中自动分配新TOKEN
	if h.sharedTokenService != nil {
		newToken, err := h.sharedTokenService.AllocateSharedTokenToUser(userID)
		if err != nil {
			logger.Warnf("[代理] 从共享TOKEN池分配失败: %v\n", err)
		} else if newToken != nil {
			logger.Infof("[代理] ✅ 从共享TOKEN池为用户 %d 自动分配新TOKEN: %s...\n",
				userID, newToken.Token[:min(8, len(newToken.Token))])
			return newToken, nil
		}
	}

	// 共享TOKEN池也没有可用TOKEN
	logger.Infof("[代理] 用户 %d 没有可用的TOKEN（已分配的共享TOKEN、自有TOKEN和共享池均无可用）\n", userID)
	return nil, fmt.Errorf("没有可用的TOKEN")
}

// getTokenAssignmentCount 获取TOKEN当前分配的用户数量（从数据库统计）
func (h *ProxyHandler) getTokenAssignmentCount(ctx context.Context, tokenID string) (int, error) {
	_ = ctx
	if h.sharedTokenService == nil {
		return 0, fmt.Errorf("共享TOKEN服务未初始化")
	}

	count, err := h.sharedTokenService.GetSharedTokenAllocationCount(tokenID)
	if err != nil {
		return 0, fmt.Errorf("查询数据库分配数量失败: %w", err)
	}

	return int(count), nil
}

// tryAssignTokenWithLimit 尝试分配TOKEN给用户
func (h *ProxyHandler) tryAssignTokenWithLimit(ctx context.Context, userToken string, tokenID string, ttl time.Duration) (bool, error) {
	cacheKey := fmt.Sprintf("AUGMENT-GATEWAY:user_token_assignment:%s", userToken)

	redisClient := h.cacheService.GetRedisClient().GetClient()

	// 设置用户TOKEN分配缓存
	if err := redisClient.Set(ctx, cacheKey, tokenID, ttl).Err(); err != nil {
		return false, fmt.Errorf("设置用户TOKEN分配缓存失败: %w", err)
	}

	logger.Infof("[代理] 成功分配TOKEN %s... 给用户 %s...\n",
		tokenID[:min(8, len(tokenID))],
		userToken[:min(8, len(userToken))])

	return true, nil
}

// selectLeastUsedToken 选择使用量最少的TOKEN，优先考虑当前分配用户数
func (h *ProxyHandler) selectLeastUsedToken(tokens []*database.Token) *database.Token {
	if len(tokens) == 0 {
		return nil
	}

	// 如果只有一个TOKEN，直接返回
	if len(tokens) == 1 {
		return tokens[0]
	}

	// 获取所有TOKEN的当前分配用户数
	tokenIDs := make([]string, len(tokens))
	for i, token := range tokens {
		tokenIDs[i] = token.ID
	}

	ctx := context.Background()
	userCounts, err := h.cacheService.GetTokenCurrentUsersCount(ctx, tokenIDs)
	if err != nil {
		logger.Infof("[代理] 获取TOKEN用户数统计失败: %v，使用原有算法\n", err)
		return h.selectLeastUsedTokenFallback(tokens)
	}

	// 找到最小使用量
	minUsage := tokens[0].UsedRequests
	for _, token := range tokens[1:] {
		if token.UsedRequests < minUsage {
			minUsage = token.UsedRequests
		}
	}

	// 收集所有使用量为最小值的TOKEN
	leastUsedTokens := make([]*database.Token, 0)
	for _, token := range tokens {
		if token.UsedRequests == minUsage {
			leastUsedTokens = append(leastUsedTokens, token)
		}
	}

	// 如果只有一个最少使用的TOKEN，直接返回
	if len(leastUsedTokens) == 1 {
		return leastUsedTokens[0]
	}

	// 多个相同使用量的TOKEN，选择当前分配用户数最少的
	return h.selectTokenWithLeastUsers(leastUsedTokens, userCounts)
}

// selectTokenWithStrongRandom 使用加强随机算法选择TOKEN
func (h *ProxyHandler) selectTokenWithStrongRandom(tokens []*database.Token) *database.Token {
	if len(tokens) == 0 {
		return nil
	}

	if len(tokens) == 1 {
		return tokens[0]
	}

	// 使用多重随机源提高随机性
	// 1. 纳秒时间戳
	nanoSeed := time.Now().UnixNano()

	// 2. 进程ID和协程ID的组合
	pid := os.Getpid()

	// 3. 内存地址随机性（使用tokens切片的地址）
	memAddr := uintptr(unsafe.Pointer(&tokens))

	// 4. 组合多个随机源
	combinedSeed := nanoSeed ^ int64(pid) ^ int64(memAddr)

	// 使用crypto/rand作为额外的随机源
	randomBytes := make([]byte, 8)
	if _, err := rand.Read(randomBytes); err == nil {
		// 将随机字节转换为int64并与其他种子组合
		cryptoSeed := int64(binary.BigEndian.Uint64(randomBytes))
		combinedSeed ^= cryptoSeed
	}

	// 创建新的随机数生成器实例
	rng := mathRand.New(mathRand.NewSource(combinedSeed))

	// 生成随机索引
	selectedIndex := rng.Intn(len(tokens))

	return tokens[selectedIndex]
}

// selectTokenWithUserPriority 优先选择用户自己提交的TOKEN，然后选择使用量最少的
func (h *ProxyHandler) selectTokenWithUserPriority(ctx context.Context, tokens []*database.Token) (*database.Token, error) {
	_ = ctx
	if len(tokens) == 0 {
		return nil, nil
	}
	// 直接选择使用量最少的TOKEN
	return h.selectLeastUsedToken(tokens), nil
}

// getAssignedTokenForUser 为用户令牌分配固定的TOKEN（TOKEN过期的前一个小时缓存）
// 当没有可用TOKEN时返回nil，不返回error
func (h *ProxyHandler) getAssignedTokenForUser(ctx context.Context, userToken string) (*database.Token, error) {
	// 缓存键
	cacheKey := fmt.Sprintf("AUGMENT-GATEWAY:user_token_assignment:%s", userToken)
	noTokenCacheKey := fmt.Sprintf("AUGMENT-GATEWAY:user_no_token:%s", userToken)

	// 先检查是否缓存了"无TOKEN"状态
	noTokenCached, err := h.cacheService.GetString(ctx, noTokenCacheKey)
	if err == nil && noTokenCached == "1" {
		logger.Infof("[代理] 用户 %s... 缓存显示无可用TOKEN，跳过查询\n",
			userToken[:min(8, len(userToken))])
		return nil, nil
	}

	// 记录用户当前使用的TOKEN ID（用于保持账号不切换）
	var preferredTokenID string

	// 先检查缓存中是否已有分配的TOKEN
	cachedTokenID, err := h.cacheService.GetString(ctx, cacheKey)
	if err == nil && cachedTokenID != "" {
		// 缓存命中，验证TOKEN是否仍然有效
		tokenInfo, err := h.tokenService.GetToken(ctx, cachedTokenID)
		if err == nil && tokenInfo.Token.Status == "active" {
			logger.Infof("[代理] 使用缓存分配的令牌给用户 %s...: %s... -> %s",
				userToken[:min(8, len(userToken))],
				tokenInfo.Token.Token[:min(8, len(tokenInfo.Token.Token))],
				tokenInfo.Token.TenantAddress)
			return &tokenInfo.Token, nil
		} else {
			// 缓存的TOKEN无效，清除缓存，但记录TOKEN ID用于后续优先检查
			logger.Infof("[代理] 缓存的令牌 %s 无效，清除缓存\n", cachedTokenID)
			h.cacheService.DeleteKey(ctx, cacheKey)
			// 记录之前使用的TOKEN ID，在重新分配时优先检查该TOKEN是否恢复有效
			preferredTokenID = cachedTokenID
		}
	}

	// 缓存未命中或TOKEN无效，重新分配
	logger.Infof("[代理] 用户 %s... 没有有效的缓存令牌，分配新令牌\n",
		userToken[:min(8, len(userToken))])

	// 使用带分配限制的负载均衡逻辑选择TOKEN，传递preferredTokenID以优先保持当前账号
	selectedToken, err := h.selectAvailableTokenWithLimit(ctx, userToken, preferredTokenID)
	if err != nil {
		// 检查是否是号池TOKEN限制错误
		if strings.HasPrefix(err.Error(), "POOL_TOKEN_LIMIT_EXCEEDED:") {
			// 号池TOKEN切换次数超限，返回特殊错误供上层处理
			logger.Infof("[代理] 用户 %s... 号池TOKEN切换次数超限: %v\n",
				userToken[:min(8, len(userToken))], err)
			return nil, err
		}

		// 其他错误：没有可用TOKEN时，缓存"无TOKEN"状态5分钟，避免重复查询
		logger.Infof("[代理] 用户 %s... 没有可用令牌: %v，缓存无TOKEN状态5分钟\n",
			userToken[:min(8, len(userToken))], err)

		// 缓存"无TOKEN"状态5分钟
		noTokenCacheKey := fmt.Sprintf("AUGMENT-GATEWAY:user_no_token:%s", userToken)
		if cacheErr := h.cacheService.SetString(ctx, noTokenCacheKey, "1", 5*time.Minute); cacheErr != nil {
			logger.Warnf("[代理] 警告: 缓存无TOKEN状态失败: %v\n", cacheErr)
		}

		return nil, nil
	}

	// 缓存时间固定为15天
	ttl := 15 * 24 * time.Hour
	cacheDesc := "缓存15天"

	// 获取用户ID用于数据库分配记录
	userTokenInfo, err := h.userAuthService.ValidateApiToken(ctx, userToken)
	if err != nil {
		logger.Infof("[代理] 获取用户信息失败: %v\n", err)
		return nil, err
	}

	// 获取用户当前在数据库中的旧TOKEN分配（如果有）
	var oldTokenID string
	if h.sharedTokenService != nil {
		oldAllocations, _ := h.sharedTokenService.GetUserSharedTokens(userTokenInfo.UserID)
		if len(oldAllocations) > 0 {
			oldTokenID = oldAllocations[0].ID
		}
	}

	// 使用原子操作分配TOKEN，避免并发竞态条件
	success, err := h.tryAssignTokenWithLimit(ctx, userToken, selectedToken.ID, ttl)
	if err != nil {
		logger.Infof("[代理] 原子分配TOKEN失败: %v\n", err)
		return nil, err
	}
	if !success {
		logger.Infof("[代理] TOKEN %s... 分配已达上限，重新选择\n",
			selectedToken.ID[:min(8, len(selectedToken.ID))])
		// TOKEN分配已满，清除"无TOKEN"缓存并返回nil，让上层重新选择
		noTokenCacheKey := fmt.Sprintf("AUGMENT-GATEWAY:user_no_token:%s", userToken)
		h.cacheService.DeleteKey(ctx, noTokenCacheKey)
		return nil, nil
	}

	// 同时写入数据库分配记录（替换旧记录）
	if h.sharedTokenService != nil {
		if err := h.sharedTokenService.ReplaceUserSharedTokenAllocation(userTokenInfo.UserID, oldTokenID, selectedToken.ID); err != nil {
			logger.Warnf("[代理] 警告: 写入数据库分配记录失败: %v（缓存已写入，不影响使用）\n", err)
		} else {
			logger.Infof("[代理] 已同步写入数据库分配记录: 用户 %d, TOKEN %s...\n",
				userTokenInfo.UserID, selectedToken.ID[:min(8, len(selectedToken.ID))])
		}
	}

	// 成功分配TOKEN后，清除"无TOKEN"状态缓存
	noTokenCacheKey = fmt.Sprintf("AUGMENT-GATEWAY:user_no_token:%s", userToken)
	if clearErr := h.cacheService.DeleteKey(ctx, noTokenCacheKey); clearErr != nil {
		logger.Warnf("[代理] 警告: 清除无TOKEN状态缓存失败: %v\n", clearErr)
	}

	logger.Infof("[代理] 为用户 %s... 分配新令牌: %s... -> %s (%s)\n",
		userToken[:min(8, len(userToken))],
		selectedToken.Token[:min(8, len(selectedToken.Token))],
		selectedToken.TenantAddress,
		cacheDesc)

	return selectedToken, nil
}

// ============================================================================
// TOKEN 过滤逻辑
// ============================================================================

// filterTokensByUserPermission 根据用户共享权限过滤TOKEN
// 返回用户可用的TOKEN：
// 1. 用户自己提交的TOKEN（始终可用）
// 2. 管理员添加的TOKEN（需要共享权限）
// 注意：不允许使用其他用户提交的TOKEN（即使标记为共享）
func (h *ProxyHandler) filterTokensByUserPermission(tokens []*database.Token, userTokenInfo *service.UserApiTokenInfo) []*database.Token {
	var filteredTokens []*database.Token
	addedTokenIDs := make(map[string]bool)

	// 1. 首先添加用户自己提交的TOKEN（无论是否共享）
	if userTokenInfo != nil && userTokenInfo.UserID > 0 {
		for _, token := range tokens {
			if token.SubmitterUserID != nil && *token.SubmitterUserID == userTokenInfo.UserID {
				filteredTokens = append(filteredTokens, token)
				addedTokenIDs[token.ID] = true
			}
		}
		logger.Infof("[代理] 用户 %d 有 %d 个自己提交的TOKEN\n", userTokenInfo.UserID, len(filteredTokens))
	}

	// 2. 添加管理员添加的TOKEN（所有用户都可以使用共享TOKEN）
	if userTokenInfo != nil {
		for _, token := range tokens {
			// 跳过已添加的TOKEN
			if addedTokenIDs[token.ID] {
				continue
			}
			// 只允许管理员添加的TOKEN（SubmitterUserID为空）
			if token.IsAdminAdded() {
				filteredTokens = append(filteredTokens, token)
				addedTokenIDs[token.ID] = true
			}
		}
		logger.Infof("[代理] 添加管理员TOKEN后共有 %d 个可用TOKEN\n", len(filteredTokens))
	}

	logger.Infof("[代理] 过滤出 %d 个可用TOKEN\n", len(filteredTokens))
	return filteredTokens
}

// ============================================================================
// TOKEN 选择辅助方法
// ============================================================================

// selectLeastUsedTokenFallback 原有的TOKEN选择算法（作为备选方案）
func (h *ProxyHandler) selectLeastUsedTokenFallback(tokens []*database.Token) *database.Token {
	if len(tokens) == 0 {
		return nil
	}

	// 找到最小使用量
	minUsage := tokens[0].UsedRequests
	for _, token := range tokens[1:] {
		if token.UsedRequests < minUsage {
			minUsage = token.UsedRequests
		}
	}

	// 收集所有使用量为最小值的TOKEN
	leastUsedTokens := make([]*database.Token, 0)
	for _, token := range tokens {
		if token.UsedRequests == minUsage {
			leastUsedTokens = append(leastUsedTokens, token)
		}
	}

	// 如果只有一个，直接返回
	if len(leastUsedTokens) == 1 {
		return leastUsedTokens[0]
	}

	// 多个相同使用量的TOKEN，使用加强随机选择
	return h.selectTokenWithStrongRandom(leastUsedTokens)
}

// selectTokenWithLeastUsers 从TOKEN列表中选择当前分配用户数最少的TOKEN
func (h *ProxyHandler) selectTokenWithLeastUsers(tokens []*database.Token, userCounts map[string]int) *database.Token {
	if len(tokens) == 0 {
		return nil
	}

	if len(tokens) == 1 {
		return tokens[0]
	}

	// 找到最少的用户分配数
	minUserCount := -1
	for _, token := range tokens {
		userCount := userCounts[token.ID]
		if minUserCount == -1 || userCount < minUserCount {
			minUserCount = userCount
		}
	}

	// 收集所有用户分配数为最小值的TOKEN
	leastAssignedTokens := make([]*database.Token, 0)
	for _, token := range tokens {
		if userCounts[token.ID] == minUserCount {
			leastAssignedTokens = append(leastAssignedTokens, token)
		}
	}

	// 如果只有一个，直接返回
	if len(leastAssignedTokens) == 1 {
		selectedToken := leastAssignedTokens[0]
		logger.Infof("[代理] 选择用户分配数最少的TOKEN: %s... (当前分配用户数: %d)\n",
			selectedToken.Token[:min(8, len(selectedToken.Token))], minUserCount)
		return selectedToken
	}

	// 多个TOKEN的用户分配数相同，使用加强随机选择
	return h.selectTokenWithStrongRandom(leastAssignedTokens)
}

// ============================================================================
// TOKEN 状态管理
// ============================================================================

// invalidateToken 标记TOKEN为不可用
func (h *ProxyHandler) invalidateToken(ctx context.Context, tokenID string) {
	// 异步更新TOKEN状态
	go func() {
		// 创建独立的上下文
		updateCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// 先获取TOKEN信息以获取TOKEN字符串
		tokenInfo, err := h.tokenService.GetToken(updateCtx, tokenID)
		if err != nil {
			logger.Warnf("[代理] 警告: 获取TOKEN信息失败 %s: %v\n", tokenID, err)
			return
		}

		// 更新TOKEN状态为inactive
		if err := h.tokenService.UpdateTokenStatus(updateCtx, tokenID, "inactive"); err != nil {
			logger.Warnf("[代理] 警告: 令牌失效操作失败 %s: %v\n", tokenID, err)
		} else {
			logger.Infof("[代理] 成功使令牌失效: %s\n", tokenID)
		}

		// 清除TOKEN缓存
		if err := h.cacheService.DeleteTokenCache(updateCtx, tokenID); err != nil {
			logger.Warnf("[代理] 警告: 清除令牌缓存失败 %s: %v\n", tokenID, err)
		}

		// 清除该TOKEN的模拟数据
		if tokenInfo != nil {
			tokenDisplay := tokenInfo.Token.Token[:min(8, len(tokenInfo.Token.Token))]

			// 清除模拟会话事件数据
			if err := h.cacheService.DeleteSessionEvents(updateCtx, tokenInfo.Token.Token); err != nil {
				logger.Warnf("[代理] 警告: 清除TOKEN %s... 的模拟会话事件数据失败: %v\n", tokenDisplay, err)
			} else {
				logger.Infof("[代理] ✅ 已清除TOKEN %s... 的模拟会话事件数据\n", tokenDisplay)
			}

			// 清除模拟特征向量数据
			if err := h.cacheService.DeleteFeatureVector(updateCtx, tokenInfo.Token.Token); err != nil {
				logger.Warnf("[代理] 警告: 清除TOKEN %s... 的模拟特征向量数据失败: %v\n", tokenDisplay, err)
			} else {
				logger.Infof("[代理] ✅ 已清除TOKEN %s... 的模拟特征向量数据\n", tokenDisplay)
			}
		}
	}()
}

// clearUserTokenAssignment 清除用户令牌的TOKEN分配缓存
func (h *ProxyHandler) clearUserTokenAssignment(ctx context.Context, userToken string) {
	cacheKey := fmt.Sprintf("AUGMENT-GATEWAY:user_token_assignment:%s", userToken)

	// 清除用户TOKEN分配缓存
	if err := h.cacheService.DeleteKey(ctx, cacheKey); err != nil {
		logger.Warnf("[代理] 警告: 清除用户令牌分配缓存失败: %v\n", err)
	} else {
		logger.Infof("[代理] 已清除用户令牌分配缓存: %s...\n",
			userToken[:min(8, len(userToken))])
	}
}

// retryWithNewToken 尝试重新分配TOKEN并重试请求
func (h *ProxyHandler) retryWithNewToken(
	c *gin.Context,
	userTokenInfo *service.UserApiTokenInfo,
	proxyReq *proxy.ProxyRequest,
	startTime time.Time,
	needsStatsAndRateLimit bool,
) bool {
	logger.Infof("[代理] 为用户重试请求使用新令牌: %s...\n",
		userTokenInfo.Token[:min(8, len(userTokenInfo.Token))])

	// 重新分配TOKEN
	newToken, err := h.getAssignedTokenForUser(c.Request.Context(), userTokenInfo.Token)
	if err != nil {
		logger.Infof("[代理] 重试时获取新令牌失败: %v\n", err)
		return false
	}

	// 如果没有可用TOKEN，返回false让调用方处理
	if newToken == nil {
		logger.Infof("[代理] 重试时没有可用令牌\n")
		return false
	}

	// 检查新TOKEN是否与原TOKEN相同（避免无限重试）
	if proxyReq.Token != nil && newToken.ID == proxyReq.Token.ID {
		logger.Infof("[代理] 新令牌与失败令牌相同，跳过重试\n")
		return false
	}

	// 更新代理请求的TOKEN信息
	proxyReq.Token = newToken
	proxyReq.TenantAddress = newToken.TenantAddress
	proxyReq.SessionID = newToken.SessionID

	logger.Infof("[代理] 使用新令牌重试: %s... -> %s\n",
		newToken.Token[:min(8, len(newToken.Token))], newToken.TenantAddress)

	// 使用新TOKEN重新发起流式请求
	capturedContent, err := h.proxyService.ForwardStreamWithCapture(c.Request.Context(), proxyReq, c.Writer)
	if err != nil {
		logger.Infof("[代理] 使用新令牌重试失败: %v\n", err)
		return false
	}

	logger.Infof("[代理] 使用新令牌重试成功\n")

	// 检测封禁响应并处理TOKEN禁用和重新分配（仅对/chat-stream接口）
	if strings.HasSuffix(proxyReq.Path, "/chat-stream") && len(capturedContent) > 0 && proxyReq.Token != nil {
		if h.detectAndHandleBannedResponse(capturedContent, userTokenInfo, proxyReq.Token, c) {
			logger.Infof("[代理] 新TOKEN重试成功后检测到封禁响应，已处理TOKEN禁用和重新分配\n")
		}
	}

	// 重试成功（计费已在请求开始时处理，不需要额外记录）
	return true
}

// ============================================================================
// TOKEN 共享相关
// ============================================================================

// hasUserSubmittedSharedToken 检查用户是否提交过共享TOKEN
func (h *ProxyHandler) hasUserSubmittedSharedToken(userID uint) bool {
	// 通过TokenService检查用户是否提交过共享TOKEN
	hasSubmitted, err := h.tokenService.HasUserSubmittedSharedToken(userID)
	if err != nil {
		logger.Infof("[代理] 检查用户共享TOKEN提交状态失败: %v\n", err)
		return false
	}
	return hasSubmitted
}

// getUserSharedTokenCount 获取用户提交的共享TOKEN数量
func (h *ProxyHandler) getUserSharedTokenCount(userID uint) int {
	// 通过TokenService获取用户提交的共享TOKEN数量
	count, err := h.tokenService.GetUserSharedTokenCount(userID)
	if err != nil {
		logger.Infof("[代理] 获取用户共享TOKEN数量失败: %v\n", err)
		return 0
	}
	return count
}

// ============================================================================
// TOKEN 绑定渠道
// ============================================================================

// getTokenBoundChannel 获取TOKEN绑定的外部渠道（带缓存）
// userID: 当前用户ID，用于查询该用户对该TOKEN的绑定关系（支持多用户独立绑定同一共享TOKEN）
func (h *ProxyHandler) getTokenBoundChannel(token *database.Token, userID uint) (*database.ExternalChannel, error) {
	if h.externalChannelService == nil {
		return nil, fmt.Errorf("外部渠道服务未初始化")
	}

	ctx := context.Background()

	// 1. 尝试从缓存获取
	if h.cacheService != nil {
		cachedChannel, err := h.cacheService.GetCachedTokenChannelBinding(ctx, token.ID, userID)
		if err == nil && cachedChannel != nil {
			// 缓存命中，检查渠道状态
			if !cachedChannel.IsActive() {
				logger.Infof("[代理] 缓存命中但渠道已禁用 - userID: %d, tokenID: %d, channelID: %d, providerName: %s\n",
					userID, token.ID, cachedChannel.ID, cachedChannel.ProviderName)
				return nil, fmt.Errorf("绑定的渠道已禁用")
			}
			return cachedChannel, nil
		}
		// 缓存未命中或获取失败，继续从数据库查询
	}

	// 2. 从数据库查询
	db := h.externalChannelService.GetDB()
	if db == nil {
		return nil, fmt.Errorf("数据库连接不可用")
	}

	// 查询TOKEN绑定的渠道（同时预加载模型映射），按用户ID区分绑定关系
	var binding database.TokenChannelBinding
	if err := db.Preload("Channel").Preload("Channel.Models").Where("token_id = ? AND user_id = ?", token.ID, userID).First(&binding).Error; err != nil {
		return nil, err
	}

	if binding.Channel == nil {
		return nil, fmt.Errorf("绑定的渠道不存在")
	}

	// 检查渠道状态
	if !binding.Channel.IsActive() {
		logger.Infof("[代理] 渠道已禁用 - userID: %d, tokenID: %s, channelID: %d, providerName: %s, channelStatus: %s\n",
			userID, token.ID, binding.ChannelID, binding.Channel.ProviderName, binding.Channel.Status)
		return nil, fmt.Errorf("绑定的渠道已禁用")
	}

	// 3. 写入缓存
	if h.cacheService != nil {
		if err := h.cacheService.CacheTokenChannelBinding(ctx, token.ID, userID, binding.Channel); err != nil {
			logger.Warnf("[代理] 缓存TOKEN渠道绑定失败: %v", err)
			// 缓存失败不影响正常流程
		}
	}

	return binding.Channel, nil
}

