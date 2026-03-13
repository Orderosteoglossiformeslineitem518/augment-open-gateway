package handler

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"augment-gateway/internal/database"
	"augment-gateway/internal/logger"
	"augment-gateway/internal/service"
	"augment-gateway/internal/utils"

	"github.com/gin-gonic/gin"
)

// ============================================================================
// 响应结构体定义
// ============================================================================

// ChatResponse 对话响应结构体
type ChatResponse struct {
	StopReason interface{} `json:"stop_reason"` // 支持 int、string 类型
	Nodes      []ChatNode  `json:"nodes"`
	Text       string      `json:"text"`
}

// ChatNode 对话节点结构体
type ChatNode struct {
	ID      int         `json:"id"`
	Type    int         `json:"type"`
	Content string      `json:"content"`
	ToolUse interface{} `json:"tool_use"`
}

// ============================================================================
// 封禁检测与处理
// ============================================================================

// detectAndHandleBannedResponse 检测TOKEN使用上限响应并处理TOKEN禁用和重新分配
func (h *ProxyHandler) detectAndHandleBannedResponse(responseBody []byte, userTokenInfo *service.UserApiTokenInfo, currentToken *database.Token, c *gin.Context) bool {
	// 检查是否为gzip压缩的数据并解压缩
	var decompressedData []byte
	if len(responseBody) > 2 && responseBody[0] == 0x1f && responseBody[1] == 0x8b {
		// 这是gzip压缩的数据，需要解压缩
		reader, err := gzip.NewReader(bytes.NewReader(responseBody))
		if err != nil {
			logger.Infof("[代理] 创建gzip读取器失败: %v\n", err)
			return false
		}
		defer reader.Close()

		decompressedData, err = io.ReadAll(reader)
		if err != nil {
			logger.Infof("[代理] gzip解压缩失败: %v\n", err)
			return false
		}
	} else {
		// 未压缩的数据
		decompressedData = responseBody
	}

	// 检查是否包含各种封禁响应文本
	responseText := string(decompressedData)

	emailStr := "未知"
	if currentToken.Email != nil {
		emailStr = *currentToken.Email
	}
	logger.Infof("[代理-封禁检测] Token-Email: %s, 检测响应内容长度: %d bytes\n", emailStr, len(responseText))

	// 特殊处理：新格式的TOKEN使用上限响应（包含具体邮箱地址）
	if len(responseText) <= 800 && strings.Contains(responseText, "You are out of user messages for") &&
		strings.Contains(responseText, "@") &&
		strings.Contains(responseText, "Please update your account [here](https://app.augmentcode.com/account) to continue using Augment") {
		logger.Infof("[代理] 🚫 检测到新格式TOKEN使用上限响应（包含邮箱），开始处理TOKEN禁用\n")

		// 记录封号信息
		go h.recordBanInfo(c, currentToken, userTokenInfo, "TOKEN使用次数达到上限", responseText)

		// 异步处理TOKEN禁用和用户令牌重新分配
		go h.handleTokenBanAndReassignment(currentToken, userTokenInfo, "TOKEN使用次数达到上限")

		return true // 检测到TOKEN使用上限响应
	}

	// 特殊处理：邮箱订阅失效响应不需要对话结束条件
	if len(responseText) <= 800 && strings.Contains(responseText, "Your subscription for account") &&
		strings.Contains(responseText, "is inactive. Please update your plan [here](https://app.augmentcode.com/account") &&
		strings.Contains(responseText, "to continue using Augment") {
		logger.Infof("[代理] 🚫 检测到邮箱订阅失效响应，开始处理TOKEN禁用\n")

		// 记录封号信息
		go h.recordBanInfo(c, currentToken, userTokenInfo, "账号订阅已失效", responseText)

		// 异步处理TOKEN禁用和用户令牌重新分配
		go h.handleTokenBanAndReassignment(currentToken, userTokenInfo, "账号订阅已失效")

		return true // 检测到邮箱订阅失效响应
	}

	// 特殊处理：新格式的邮箱订阅失效响应（包含具体邮箱地址）
	if len(responseText) <= 800 && strings.Contains(responseText, "Your subscription for") &&
		strings.Contains(responseText, "@") &&
		strings.Contains(responseText, "is inactive. Please update your plan [here](https://app.augmentcode.com/account") &&
		strings.Contains(responseText, "to continue using Augment") {
		logger.Infof("[代理] 🚫 检测到新格式邮箱订阅失效响应（包含邮箱），开始处理TOKEN禁用\n")

		// 记录封号信息
		go h.recordBanInfo(c, currentToken, userTokenInfo, "账号订阅已失效", responseText)

		// 异步处理TOKEN禁用和用户令牌重新分配
		go h.handleTokenBanAndReassignment(currentToken, userTokenInfo, "账号订阅已失效")

		return true // 检测到新格式邮箱订阅失效响应
	}

	// 特殊处理：积分配额用尽响应不需要对话结束条件
	if strings.Contains(responseText, "You have run out of credits for") &&
		strings.Contains(responseText, "Click [here](https://app.augmentcode.com/account) to upgrade") {
		logger.Infof("[代理] 🚫 检测到积分配额用尽响应，开始验证积分状态\n")

		// 调用 /get-credit-info 接口验证积分是否真的耗尽
		isCreditExhausted, usedRequests, maxRequests := h.verifyCreditExhaustion(currentToken)

		if !isCreditExhausted {
			logger.Infof("[代理] ✅ 验证结果：积分未耗尽 (已使用: %d, 总额度: %d)，跳过处理\n", usedRequests, maxRequests)
			return false // 积分未耗尽，不执行处理
		}

		logger.Infof("[代理] ⚠️ 验证结果：积分已耗尽 (已使用: %d, 总额度: %d)\n", usedRequests, maxRequests)

		// 更新TOKEN的使用次数为最大使用次数
		ctx := context.Background()
		if err := h.tokenService.UpdateUsedRequests(ctx, currentToken.ID, maxRequests); err != nil {
			logger.Infof("[代理] ❌ 更新TOKEN使用次数失败: %v\n", err)
		} else {
			logger.Infof("[代理] ✅ 已更新TOKEN使用次数为最大值: %d\n", maxRequests)
		}

		// 积分用尽时，将账号类型更新为0（不再封禁账号，账号依旧可用）
		logger.Infof("[代理] 📝 TOKEN积分用尽，更新账号类型为0\n")
		if err := h.tokenService.UpdateMaxRequests(ctx, currentToken.ID, 0); err != nil {
			logger.Infof("[代理] ❌ 更新TOKEN账号类型失败: %v\n", err)
		} else {
			logger.Infof("[代理] ✅ 已将TOKEN账号类型更新为0，账号仍可正常使用\n")
		}

		return true // 检测到积分配额用尽响应
	}

	// 检测其他封禁文本 - 只在对话结束时检测，避免误判
	var banReason string
	var detected bool

	// 进一步验证：检查封禁文本是否出现在最后的响应中
	detected = h.detectBanTextInFinalResponse(decompressedData, &banReason)

	if detected {
		logger.Infof("[代理] 🚫 检测到封禁响应，开始记录封号信息和处理TOKEN禁用 - 原因: %s\n", banReason)

		// 记录封号信息
		go h.recordBanInfo(c, currentToken, userTokenInfo, banReason, responseText)

		// 异步处理TOKEN禁用和用户令牌重新分配
		go h.handleTokenBanAndReassignment(currentToken, userTokenInfo, banReason)

		return true // 检测到封禁响应
	}

	return false // 未检测到封禁响应
}

// recordBanInfo 记录封号信息
func (h *ProxyHandler) recordBanInfo(c *gin.Context, currentToken *database.Token, userTokenInfo *service.UserApiTokenInfo, banReason, responseText string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var tokenID *string
	var userToken *string

	if currentToken != nil {
		tokenID = &currentToken.ID
	}

	if userTokenInfo != nil {
		userToken = &userTokenInfo.Token
	}

	// 记录封号信息
	err := h.banRecordService.CreateBanRecordFromRequest(
		ctx,
		c,
		tokenID,
		userToken,
		banReason,
		responseText,
	)

	if err != nil {
		logger.Warnf("[代理] 警告: 记录封号信息失败: %v\n", err)
	} else {
		logger.Infof("[代理] ✅ 成功记录封号信息 - 原因: %s\n", banReason)
	}
}

// sendTokenReassignmentMessage 发送TOKEN重新分配的模拟响应
func (h *ProxyHandler) sendTokenReassignmentMessage(c *gin.Context) {
	// 使用流式响应工具发送消息
	helper := utils.NewStreamResponseHelper()
	message := "温馨提示：检测到当前TOKEN已被封禁，请自行添加新的TOKEN账号后重试。"
	helper.SendStreamMessage(c, message)
}

// handleTokenBanAndReassignment 处理TOKEN禁用和用户令牌重新分配
func (h *ProxyHandler) handleTokenBanAndReassignment(bannedToken *database.Token, userTokenInfo *service.UserApiTokenInfo, banReason string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if bannedToken == nil {
		logger.Warnf("[代理] 警告: 无法处理TOKEN禁用，TOKEN为空\n")
		return
	}

	logger.Infof("[代理] 开始禁用TOKEN: %s...\n", bannedToken.Token[:min(8, len(bannedToken.Token))])

	// 1. 更新TOKEN状态为disabled，在现有描述前面添加自动封禁标识
	newDescription := "(自动封禁) " + bannedToken.Description
	updateReq := &service.UpdateTokenRequest{
		Status:      "disabled",
		Description: newDescription,
	}

	if _, err := h.tokenService.UpdateToken(ctx, bannedToken.ID, updateReq); err != nil {
		logger.Errorf("[代理] 错误: 更新TOKEN状态失败 %s: %v\n", bannedToken.ID, err)
	} else {
		logger.Infof("[代理] ✅ 成功禁用TOKEN: %s，描述已更新\n", bannedToken.Token[:min(8, len(bannedToken.Token))])

		tokenDisplay := bannedToken.Token[:min(8, len(bannedToken.Token))]

		// 清除该TOKEN的模拟会话事件数据
		if err := h.cacheService.DeleteSessionEvents(ctx, bannedToken.Token); err != nil {
			logger.Warnf("[代理] 警告: 清除TOKEN %s... 的模拟会话事件数据失败: %v\n", tokenDisplay, err)
		} else {
			logger.Infof("[代理] ✅ 已清除TOKEN %s... 的模拟会话事件数据\n", tokenDisplay)
		}

		// 清除该TOKEN的模拟特征向量数据
		if err := h.cacheService.DeleteFeatureVector(ctx, bannedToken.Token); err != nil {
			logger.Warnf("[代理] 警告: 清除TOKEN %s... 的模拟特征向量数据失败: %v\n", tokenDisplay, err)
		} else {
			logger.Infof("[代理] ✅ 已清除TOKEN %s... 的模拟特征向量数据\n", tokenDisplay)
		}

	}

	// 2. 清除当前用户的TOKEN分配缓存，强制重新分配
	h.clearUserTokenAssignment(ctx, userTokenInfo.Token)

	// 2.5 清理shared_token_allocations表中该TOKEN的所有分配记录，并清除所有受影响用户的缓存
	if h.sharedTokenService != nil {
		// 获取所有受影响的用户ID并删除分配记录
		affectedUserIDs, err := h.sharedTokenService.RevokeTokenAllAllocationsAndGetAffectedUserIDs(bannedToken.ID)
		if err != nil {
			logger.Warnf("[代理] 警告: 清理shared_token_allocations失败: %v\n", err)
		} else {
			logger.Infof("[代理] ✅ 已清理被封禁TOKEN %s... 的所有shared_token_allocations记录，受影响用户数: %d\n",
				bannedToken.ID[:min(8, len(bannedToken.ID))], len(affectedUserIDs))

			// 清除所有受影响用户的TOKEN分配缓存
			if len(affectedUserIDs) > 0 && h.userAuthService != nil {
				h.clearAffectedUsersCaches(ctx, affectedUserIDs, userTokenInfo.UserID)
			}
		}
	}

	// 3. 为用户重新分配新的可用TOKEN
	newToken, err := h.getAssignedTokenForUser(ctx, userTokenInfo.Token)
	if err != nil {
		logger.Warnf("[代理] 警告: 为用户重新分配TOKEN失败: %v\n", err)
		// 即使重新分配失败，也要清理旧TOKEN的增强绑定关系
		h.cleanupTokenChannelBindings(ctx, bannedToken.ID, userTokenInfo.UserID)
		return
	}

	if newToken == nil {
		logger.Warnf("[代理] 警告: 没有可用的TOKEN可以重新分配给用户\n")
		// 即使没有可用TOKEN，也要清理旧TOKEN的增强绑定关系
		h.cleanupTokenChannelBindings(ctx, bannedToken.ID, userTokenInfo.UserID)
		return
	}

	logger.Infof("[代理] ✅ 成功为用户 %s... 重新分配TOKEN: %s... -> %s\n",
		userTokenInfo.Token[:min(8, len(userTokenInfo.Token))],
		newToken.Token[:min(8, len(newToken.Token))],
		newToken.TenantAddress)

	// 4. 迁移增强渠道绑定关系：从旧TOKEN迁移到新TOKEN
	h.migrateTokenChannelBindings(ctx, bannedToken.ID, newToken.ID, userTokenInfo.UserID)
}

// clearAffectedUsersCaches 清除所有受影响用户的TOKEN分配缓存
// 跳过当前用户（已在上面单独处理）
func (h *ProxyHandler) clearAffectedUsersCaches(ctx context.Context, affectedUserIDs []uint, currentUserID uint) {
	// 批量查询受影响用户的API Token
	db := h.userAuthService.GetDB()
	if db == nil {
		logger.Warnf("[代理] 警告: 无法获取数据库连接，跳过清理受影响用户缓存\n")
		return
	}

	// 过滤掉当前用户，避免重复清理
	otherUserIDs := make([]uint, 0, len(affectedUserIDs))
	for _, userID := range affectedUserIDs {
		if userID != currentUserID {
			otherUserIDs = append(otherUserIDs, userID)
		}
	}

	if len(otherUserIDs) == 0 {
		return
	}

	// 批量查询用户的API Token
	var users []struct {
		ID       uint
		ApiToken string
	}
	if err := db.Table("users").Select("id, api_token").Where("id IN ?", otherUserIDs).Find(&users).Error; err != nil {
		logger.Warnf("[代理] 警告: 批量查询受影响用户API Token失败: %v\n", err)
		return
	}

	// 清除每个用户的TOKEN分配缓存
	clearedCount := 0
	for _, user := range users {
		if user.ApiToken != "" {
			h.clearUserTokenAssignment(ctx, user.ApiToken)
			clearedCount++
		}
	}

	logger.Infof("[代理] ✅ 已清除 %d 个受影响用户的TOKEN分配缓存\n", clearedCount)
}

// migrateTokenChannelBindings 迁移增强渠道绑定关系：从旧TOKEN迁移到新TOKEN
func (h *ProxyHandler) migrateTokenChannelBindings(ctx context.Context, oldTokenID, newTokenID string, userID uint) {
	if h.externalChannelService == nil {
		return
	}

	db := h.externalChannelService.GetDB()
	if db == nil {
		logger.Warnf("[代理] 警告: 无法获取数据库连接，跳过增强绑定迁移\n")
		return
	}

	// 查询旧TOKEN的增强绑定关系
	var oldBindings []database.TokenChannelBinding
	if err := db.Where("token_id = ? AND user_id = ?", oldTokenID, userID).Find(&oldBindings).Error; err != nil {
		logger.Warnf("[代理] 警告: 查询旧TOKEN增强绑定关系失败: %v\n", err)
		return
	}

	if len(oldBindings) == 0 {
		logger.Infof("[代理] 旧TOKEN %s... 无增强绑定关系，无需迁移\n", oldTokenID[:min(8, len(oldTokenID))])
		return
	}

	logger.Infof("[代理] 发现旧TOKEN %s... 有 %d 个增强绑定关系，开始迁移到新TOKEN %s...\n",
		oldTokenID[:min(8, len(oldTokenID))], len(oldBindings), newTokenID[:min(8, len(newTokenID))])

	// 开启事务进行迁移
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	migratedCount := 0
	for _, binding := range oldBindings {
		// 检查新TOKEN是否已有相同渠道的绑定
		var existingBinding database.TokenChannelBinding
		err := tx.Where("token_id = ? AND channel_id = ? AND user_id = ?", newTokenID, binding.ChannelID, userID).First(&existingBinding).Error

		if err == nil {
			// 新TOKEN已存在该渠道绑定，跳过
			logger.Infof("[代理] 新TOKEN已存在渠道 %d 的绑定，跳过\n", binding.ChannelID)
			continue
		}

		// 创建新绑定
		newBinding := database.TokenChannelBinding{
			TokenID:   newTokenID,
			ChannelID: binding.ChannelID,
			UserID:    userID,
		}
		if err := tx.Create(&newBinding).Error; err != nil {
			logger.Warnf("[代理] 警告: 创建新TOKEN增强绑定失败: %v\n", err)
			continue
		}
		migratedCount++
	}

	// 删除旧TOKEN的所有绑定关系
	if err := tx.Where("token_id = ? AND user_id = ?", oldTokenID, userID).Delete(&database.TokenChannelBinding{}).Error; err != nil {
		logger.Warnf("[代理] 警告: 删除旧TOKEN增强绑定失败: %v\n", err)
		tx.Rollback()
		return
	}

	// 更新新TOKEN的enhanced_enabled状态为true
	if migratedCount > 0 {
		if err := tx.Model(&database.Token{}).Where("id = ?", newTokenID).Update("enhanced_enabled", true).Error; err != nil {
			logger.Warnf("[代理] 警告: 更新新TOKEN增强状态失败: %v\n", err)
		}
	}

	// 更新旧TOKEN的enhanced_enabled状态为false
	if err := tx.Model(&database.Token{}).Where("id = ?", oldTokenID).Update("enhanced_enabled", false).Error; err != nil {
		logger.Warnf("[代理] 警告: 更新旧TOKEN增强状态失败: %v\n", err)
	}

	if err := tx.Commit().Error; err != nil {
		logger.Warnf("[代理] 警告: 提交增强绑定迁移事务失败: %v\n", err)
		return
	}

	// 清除旧TOKEN和新TOKEN的缓存
	if h.cacheService != nil {
		_ = h.cacheService.InvalidateTokenChannelBinding(ctx, oldTokenID, userID)
		_ = h.cacheService.InvalidateTokenChannelBinding(ctx, newTokenID, userID)
	}

	logger.Infof("[代理] ✅ 成功迁移 %d 个增强绑定关系: %s... -> %s...\n",
		migratedCount, oldTokenID[:min(8, len(oldTokenID))], newTokenID[:min(8, len(newTokenID))])
}

// cleanupTokenChannelBindings 清理TOKEN的增强渠道绑定关系（当无法重新分配TOKEN时使用）
func (h *ProxyHandler) cleanupTokenChannelBindings(ctx context.Context, tokenID string, userID uint) {
	if h.externalChannelService == nil {
		return
	}

	db := h.externalChannelService.GetDB()
	if db == nil {
		logger.Warnf("[代理] 警告: 无法获取数据库连接，跳过增强绑定清理\n")
		return
	}

	// 删除该TOKEN的所有绑定关系
	result := db.Where("token_id = ? AND user_id = ?", tokenID, userID).Delete(&database.TokenChannelBinding{})
	if result.Error != nil {
		logger.Warnf("[代理] 警告: 清理TOKEN %s... 增强绑定失败: %v\n", tokenID[:min(8, len(tokenID))], result.Error)
		return
	}

	if result.RowsAffected > 0 {
		// 更新TOKEN的enhanced_enabled状态为false
		if err := db.Model(&database.Token{}).Where("id = ?", tokenID).Update("enhanced_enabled", false).Error; err != nil {
			logger.Warnf("[代理] 警告: 更新TOKEN增强状态失败: %v\n", err)
		}
		// 清除缓存
		if h.cacheService != nil {
			_ = h.cacheService.InvalidateTokenChannelBinding(ctx, tokenID, userID)
		}
		logger.Infof("[代理] ✅ 已清理TOKEN %s... 的 %d 个增强绑定关系\n", tokenID[:min(8, len(tokenID))], result.RowsAffected)
	}
}

// ============================================================================
// 对话结束判断
// ============================================================================

// shouldCountAsConversation 检查是否应该计为一次对话
// 使用新的对话完全结束判断逻辑
func (h *ProxyHandler) shouldCountAsConversation(responseBody []byte) bool {
	// 检查是否为gzip压缩的数据并解压缩
	var decompressedData []byte
	if len(responseBody) > 2 && responseBody[0] == 0x1f && responseBody[1] == 0x8b {
		// 这是gzip压缩的数据，需要解压缩
		reader, err := gzip.NewReader(bytes.NewReader(responseBody))
		if err != nil {
			logger.Infof("[代理] 创建gzip读取器失败: %v\n", err)
			return false
		}
		defer reader.Close()

		decompressedData, err = io.ReadAll(reader)
		if err != nil {
			logger.Infof("[代理] gzip解压缩失败: %v\n", err)
			return false
		}
		logger.Infof("[代理] 检测到gzip压缩数据，解压缩成功，原始大小: %d bytes，解压后大小: %d bytes\n",
			len(responseBody), len(decompressedData))
	} else {
		// 未压缩的数据
		decompressedData = responseBody
	}

	// 检查是否包含TOKEN使用上限响应文本
	responseText := string(decompressedData)
	if strings.Contains(responseText, "You are out of user messages for account") {
		logger.Infof("[代理] 🚫 检测到TOKEN使用上限响应，不计费\n")
		return false // TOKEN使用上限响应不计费
	}

	// 检查新格式的TOKEN使用上限响应（包含具体邮箱地址）
	if strings.Contains(responseText, "You are out of user messages for") &&
		strings.Contains(responseText, "@") {
		logger.Infof("[代理] 🚫 检测到新格式TOKEN使用上限响应（包含邮箱），不计费\n")
		return false // TOKEN使用上限响应不计费
	}

	// 检查邮箱订阅失效响应
	if strings.Contains(responseText, "Your subscription for account") &&
		strings.Contains(responseText, "is inactive. Please update your plan [here](https://app.augmentcode.com/account") {
		logger.Infof("[代理] 🚫 检测到邮箱订阅失效响应，不计费\n")
		return false // 邮箱订阅失效响应不计费
	}

	// 检查新格式的邮箱订阅失效响应（包含具体邮箱地址）
	if strings.Contains(responseText, "Your subscription for") &&
		strings.Contains(responseText, "@") &&
		strings.Contains(responseText, "is inactive. Please update your plan [here](https://app.augmentcode.com/account") {
		logger.Infof("[代理] 🚫 检测到新格式邮箱订阅失效响应（包含邮箱），不计费\n")
		return false // 新格式邮箱订阅失效响应不计费
	}

	// 检查请求被阻止响应
	if strings.Contains(responseText, "Request blocked. Please reach out to support@augmentcode.com if you think this was a mistake") {
		logger.Infof("[代理] 🚫 检测到请求被阻止响应，不计费\n")
		return false // 请求被阻止响应不计费
	}

	// 使用新的对话完全结束判断逻辑
	return h.isConversationCompletelyEnded(decompressedData)
}

// isConversationCompletelyEnded 判断对话是否完全结束
// 使用新的对话完全结束判断逻辑
func (h *ProxyHandler) isConversationCompletelyEnded(responseData []byte) bool {
	// 将响应数据按行分割，解析每个JSON对象
	lines := strings.Split(string(responseData), "\n")

	var lastValidResponse *ChatResponse

	// 遍历所有行，找到最后一个有效的响应
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var response ChatResponse
		if err := json.Unmarshal([]byte(line), &response); err != nil {
			continue // 跳过无效的JSON行
		}

		lastValidResponse = &response
	}

	if lastValidResponse == nil {
		logger.Infof("[代理] 未找到有效的响应数据，不计费\n")
		return false
	}

	// 获取stop_reason和nodes
	stopReason := lastValidResponse.StopReason
	nodes := lastValidResponse.Nodes
	text := lastValidResponse.Text

	logger.Infof("[代理] 对话结束判断 - stop_reason: %v, nodes数量: %d, text长度: %d\n",
		stopReason, len(nodes), len(text))

	// 1. 明确的结束信号（用于调试日志）
	_ = stopReason // 避免未使用变量警告

	// 2. 检查是否有工具调用请求
	hasToolUseRequest := false
	for _, node := range nodes {
		if node.ToolUse != nil {
			hasToolUseRequest = true
			break
		}
	}

	// 3. 检查节点类型 - 基于观察到的模式
	// type 0: 普通内容节点
	// type 2: 可能的中间结束节点
	// type 3: 最终确认结束节点
	hasFinalEndingNode := false
	for _, node := range nodes {
		if node.Type == 3 {
			hasFinalEndingNode = true
			break
		}
	}

	// 4. 检查是否是空响应（通常表示流结束）
	isEmptyResponse := strings.TrimSpace(text) == "" && len(nodes) > 0

	// 综合判断逻辑
	if stopReason == 1 || (stopReason != nil && fmt.Sprintf("%v", stopReason) == "1") {
		// 如果有工具调用请求，对话继续
		if hasToolUseRequest {
			logger.Infof("[代理] stop_reason=1但有工具调用请求，对话继续，不计费\n")
			return false
		}

		// 如果有最终结束节点，对话结束
		if hasFinalEndingNode {
			logger.Infof("[代理] stop_reason=1且有最终结束节点(type=3)，对话结束，计费\n")
			return true
		}

		// 如果是空响应且有结束类型节点，对话结束
		if isEmptyResponse {
			hasEndTypeNode := false
			for _, node := range nodes {
				if node.Type >= 2 {
					hasEndTypeNode = true
					break
				}
			}
			if hasEndTypeNode {
				logger.Infof("[代理] stop_reason=1且为空响应且有结束类型节点，对话结束，计费\n")
				return true
			}
		}

		// 其他情况下，END_TURN 可能只是轮次结束，不是对话结束
		logger.Infof("[代理] stop_reason=1但不满足对话结束条件，不计费\n")
		return false
	}

	// 其他明确的结束信号
	if stopReason == 2 || (stopReason != nil && fmt.Sprintf("%v", stopReason) == "2") {
		logger.Infof("[代理] stop_reason=2(MAX_TOKENS)，强制结束，计费\n")
		return true
	}

	if stopReason == "STREAM_CANCELLED" {
		logger.Infof("[代理] stop_reason=STREAM_CANCELLED，流取消，计费\n")
		return true
	}

	if stopReason == "STREAM_TIMEOUT" {
		logger.Infof("[代理] stop_reason=STREAM_TIMEOUT，流超时，计费\n")
		return true
	}

	// 默认情况：对话继续
	logger.Infof("[代理] 未满足对话结束条件，对话继续，不计费\n")
	return false
}

// ============================================================================
// 封禁文本检测
// ============================================================================

// detectBanTextInFinalResponse 检测封禁文本是否出现在最终响应中
// 只检查最后几个有效的响应，避免误判对话过程中提到的相关内容
func (h *ProxyHandler) detectBanTextInFinalResponse(responseData []byte, banReason *string) bool {
	// 将响应数据按行分割，解析每个JSON对象
	lines := strings.Split(string(responseData), "\n")

	// 收集最后几个有效响应（最多检查最后5个）
	var lastResponses []ChatResponse
	maxResponses := 5

	// 从后往前遍历，收集最后几个有效响应
	for i := len(lines) - 1; i >= 0 && len(lastResponses) < maxResponses; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		var response ChatResponse
		if err := json.Unmarshal([]byte(line), &response); err != nil {
			continue // 跳过无效的JSON行
		}

		lastResponses = append([]ChatResponse{response}, lastResponses...) // 前插保持顺序
	}

	if len(lastResponses) == 0 {
		return false
	}

	// 检查最后几个响应中的文本内容
	var finalTexts []string
	for _, resp := range lastResponses {
		if resp.Text != "" {
			finalTexts = append(finalTexts, resp.Text)
		}

		// 也检查nodes中的内容
		for _, node := range resp.Nodes {
			if node.Content != "" {
				finalTexts = append(finalTexts, node.Content)
			}
		}
	}

	// 合并所有最终文本进行检测
	finalText := strings.Join(finalTexts, " ")

	// 检测封禁文本模式
	banPatterns := []struct {
		pattern string
		reason  string
	}{
		{
			pattern: "Request blocked. Please reach out to support@augmentcode.com if you think this was a mistake",
			reason:  "请求被阻止 - 疑似违规内容",
		},
		{
			pattern: "has been suspended. To continue, [purchase a subscription](https://app.augmentcode.com/account)",
			reason:  "账号被暂停 - 需要购买订阅",
		},
		{
			pattern: "Your account has been suspended",
			reason:  "账号被暂停",
		},
	}

	// 首先检查精确匹配的模式
	for _, bp := range banPatterns {
		if strings.Contains(finalText, bp.pattern) {
			// 进一步验证：检查响应结构是否符合封禁响应的特征
			if h.validateBanResponseStructure(lastResponses, bp.pattern) {
				*banReason = bp.reason
				logger.Infof("[代理] 在最终响应中检测到封禁文本: %s\n", bp.pattern)
				return true
			} else {
				logger.Infof("[代理] 检测到封禁文本但响应结构不符合封禁特征，可能是误判: %s\n", bp.pattern)
			}
		}
	}

	// 检查灵活的账号暂停模式：Your account [邮箱] has been suspended
	if h.detectFlexibleSuspendedPattern(finalText) {
		*banReason = "账号被封禁 - 识别到异常"
		return true
	}

	return false
}

// detectFlexibleSuspendedPattern 检测灵活的账号暂停模式
// 使用正则表达式精确匹配：Your account [邮箱] has been suspended
func (h *ProxyHandler) detectFlexibleSuspendedPattern(text string) bool {
	// 使用正则表达式精确匹配账号暂停响应格式
	// 匹配：Your account [邮箱地址] has been suspended. To continue
	pattern := `Your account\s+[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}\s+has been suspended\.\s*To continue`

	matched, err := regexp.MatchString(pattern, text)
	if err != nil {
		logger.Infof("[代理] 正则表达式匹配失败: %v\n", err)
		return false
	}

	if matched {
		// 进一步验证：确保这是一个简短的封禁响应，而不是长篇对话中的引用
		// 封禁响应通常很简短，不超过300字符
		if len(text) > 300 {
			logger.Infof("[代理] 检测到暂停模式但文本过长(%d字符)，可能是对话中的引用，跳过\n", len(text))
			return false
		}

		// 检查是否包含大量其他内容（避免在长对话中的误判）
		// 计算除了暂停消息外的其他内容长度
		suspendedMessageLength := len("Your account  has been suspended. To continue")
		otherContentLength := len(text) - suspendedMessageLength - 50 // 50字符容差（邮箱长度等）

		if otherContentLength > 200 {
			logger.Infof("[代理] 检测到暂停模式但包含大量其他内容(%d字符)，可能是对话引用，跳过\n", otherContentLength)
			return false
		}

		logger.Infof("[代理] 检测到精确的账号暂停模式，正则匹配成功且为简短封禁响应\n")
		return true
	}

	return false
}

// parseResponsesFromText 从响应文本中解析出ChatResponse数组
func (h *ProxyHandler) parseResponsesFromText(responseData []byte) []ChatResponse {
	lines := strings.Split(string(responseData), "\n")
	var responses []ChatResponse

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var response ChatResponse
		if err := json.Unmarshal([]byte(line), &response); err != nil {
			continue // 跳过无效的JSON行
		}

		responses = append(responses, response)
	}

	return responses
}

// validateBanResponseStructure 验证响应结构是否符合封禁响应的特征
func (h *ProxyHandler) validateBanResponseStructure(responses []ChatResponse, banText string) bool {
	if len(responses) == 0 {
		return false
	}

	// 获取最后一个响应
	lastResponse := responses[len(responses)-1]

	// 对于"Request blocked"这种特定的封禁类型，使用更严格的验证条件
	isRequestBlockedBan := strings.Contains(banText, "Request blocked. Please reach out to support@augmentcode.com")

	if isRequestBlockedBan {
		// 更严格的验证条件，避免误判正常对话中提到封禁内容的情况

		// 1. 必须有stop_reason且不为null（封禁响应通常会结束对话）
		hasStopReason := lastResponse.StopReason != nil
		if !hasStopReason {
			logger.Infof("[代理] Request blocked封禁检测 - 缺少stop_reason，可能是误判\n")
			return false
		}

		// 2. 必须包含完整的封禁消息（而不是片段或引用）
		containsFullMessage := strings.Contains(lastResponse.Text, banText)
		if !containsFullMessage {
			logger.Infof("[代理] Request blocked封禁检测 - 未包含完整封禁消息，可能是误判\n")
			return false
		}

		// 3. 文本长度必须合理（不超过500字符，避免正常对话中的引用）
		textLength := len(lastResponse.Text)
		isReasonableLength := textLength > 0 && textLength < 500
		if !isReasonableLength {
			logger.Infof("[代理] Request blocked封禁检测 - 文本长度异常(%d字符)，可能是误判\n", textLength)
			return false
		}

		// 4. nodes结构必须简单（不超过1个节点，真正的封禁响应结构简单）
		hasSimpleNodes := len(lastResponse.Nodes) <= 1
		if !hasSimpleNodes {
			logger.Infof("[代理] Request blocked封禁检测 - nodes结构复杂(%d个节点)，可能是误判\n", len(lastResponse.Nodes))
			return false
		}

		// 5. 不能包含大量其他内容（避免正常对话中的引用）
		// 检查是否包含其他大量文本内容
		otherContentLength := textLength - len(banText)
		hasMinimalOtherContent := otherContentLength < 100 // 除了封禁消息外，其他内容不超过100字符
		if !hasMinimalOtherContent {
			logger.Infof("[代理] Request blocked封禁检测 - 包含大量其他内容(%d字符)，可能是误判\n", otherContentLength)
			return false
		}

		logger.Infof("[代理] Request blocked封禁响应结构验证通过 - stop_reason存在: %v, 包含完整消息: %v, 文本长度合理: %v, 简单nodes: %v, 最少其他内容: %v\n",
			hasStopReason, containsFullMessage, isReasonableLength, hasSimpleNodes, hasMinimalOtherContent)

		return true
	}

	// 对于其他类型的封禁响应，使用原有的验证逻辑
	// 检查stop_reason
	hasStopReason := lastResponse.StopReason != nil

	// 检查文本长度（封禁消息通常比较简短）
	textLength := len(lastResponse.Text)
	isShortText := textLength > 0 && textLength < 500 // 封禁消息通常不会太长

	// 检查是否包含完整的封禁消息（而不是片段）
	containsFullMessage := strings.Contains(lastResponse.Text, banText)

	// 节点验证
	hasSimpleNodes := len(lastResponse.Nodes) <= 1
	if !hasSimpleNodes {
		logger.Infof("[代理] 其他封禁响应结构验证  - nodes结构复杂(%d个节点)，可能是误判\n", len(lastResponse.Nodes))
		return false
	}

	// 综合判断
	isValidBanResponse := hasStopReason && (isShortText || containsFullMessage)

	logger.Infof("[代理] 其他封禁响应结构验证 - stop_reason存在: %v, 文本简短: %v, 包含完整消息: %v, 最终判断: %v\n",
		hasStopReason, isShortText, containsFullMessage, isValidBanResponse)

	return isValidBanResponse
}

// ============================================================================
// AuthSession 刷新
// ============================================================================

// tryRefreshTokenByAuthSession 尝试通过AuthSession刷新TOKEN
// 参数：ctx - 上下文，token - TOKEN对象
// 返回：是否刷新成功，错误信息
func (h *ProxyHandler) tryRefreshTokenByAuthSession(ctx context.Context, token *database.Token) (bool, error) {
	if token == nil {
		return false, fmt.Errorf("TOKEN对象为空")
	}

	// 检查是否配置了AuthSession
	if token.AuthSession == "" {
		logger.Infof("[代理] TOKEN %s... 未配置AuthSession，跳过刷新\n", token.Token[:min(8, len(token.Token))])
		return false, nil
	}

	logger.Infof("[代理] 🔄 检测到TOKEN %s... 配置了AuthSession，尝试刷新TOKEN\n", token.Token[:min(8, len(token.Token))])

	// 1. 验证AuthSession是否有效
	if err := h.authSessionClient.ValidateAuthSession(token.AuthSession); err != nil {
		logger.Infof("[代理] ❌ AuthSession验证失败: %v\n", err)
		return false, fmt.Errorf("AuthSession验证失败: %v", err)
	}

	// 2. 通过AuthSession获取新的TOKEN、租户地址和新的AuthSession
	tenantURL, accessToken, _, newAuthSession, err := h.authSessionClient.AuthDevice(token.AuthSession)
	if err != nil {
		logger.Infof("[代理] ❌ 通过AuthSession获取TOKEN失败: %v\n", err)
		return false, fmt.Errorf("通过AuthSession获取TOKEN失败: %v", err)
	}

	// 3. 更新数据库中的TOKEN、租户地址和AuthSession（实现循环刷新）
	newTenantAddress := strings.TrimSuffix(tenantURL, "/") + "/"
	updateReq := &service.UpdateTokenRequest{
		Token:         accessToken,
		TenantAddress: newTenantAddress,
		AuthSession:   &newAuthSession, // 同时更新AuthSession，实现循环刷新
	}

	if _, err := h.tokenService.UpdateToken(ctx, token.ID, updateReq); err != nil {
		logger.Infof("[代理] ❌ 更新TOKEN失败: %v\n", err)
		return false, fmt.Errorf("更新TOKEN失败: %v", err)
	}

	logger.Infof("[代理] ✅ AuthSession刷新TOKEN成功，已更新数据库 - 新TOKEN: %s..., 新租户地址: %s, AuthSession已同步刷新\n",
		accessToken[:min(8, len(accessToken))], newTenantAddress)

	return true, nil
}
