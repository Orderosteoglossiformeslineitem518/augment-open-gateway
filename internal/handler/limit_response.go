package handler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"augment-gateway/internal/service"
	"augment-gateway/internal/utils"

	"github.com/gin-gonic/gin"
)

// LimitResponseHandler 限制响应处理器
type LimitResponseHandler struct {
	cacheService    *service.CacheService
	userAuthService *service.UserAuthService
	streamHelper    *utils.StreamResponseHelper
}

// NewLimitResponseHandler 创建限制响应处理器
func NewLimitResponseHandler(
	cacheService *service.CacheService,
	userAuthService *service.UserAuthService,
) *LimitResponseHandler {
	return &LimitResponseHandler{
		cacheService:    cacheService,
		userAuthService: userAuthService,
		streamHelper:    utils.NewStreamResponseHelper(),
	}
}

// CheckUserTokenLimits 检查用户令牌限制状态
func (h *LimitResponseHandler) CheckUserTokenLimits(ctx context.Context, userTokenInfo *service.UserApiTokenInfo) (bool, string, error) {
	// 检查令牌是否被禁用
	if userTokenInfo.Status != "active" {
		return false, "disabled", nil
	}

	// 检查使用次数限制
	if userTokenInfo.MaxRequests > 0 && userTokenInfo.UsedRequests >= userTokenInfo.MaxRequests {
		return false, "max_requests", nil
	}

	// 检查频率限制
	allowed, err := h.cacheService.CheckUserTokenRateLimit(ctx, userTokenInfo.Token, userTokenInfo.RateLimitPerMinute)
	if err != nil {
		return false, "", fmt.Errorf("检查频率限制失败: %w", err)
	}
	if !allowed {
		return false, "rate_limit", nil
	}

	return true, "", nil
}

// GetRateLimitResetTime 获取频率限制重置时间（分钟）
func (h *LimitResponseHandler) GetRateLimitResetTime() int {
	// 频率限制每1分钟重置一次，与滑动窗口保持一致
	return 1
}

// RespondWithRateLimitMessage 返回频率限制消息（流式响应）
func (h *LimitResponseHandler) RespondWithRateLimitMessage(c *gin.Context, resetMinutes int) {
	message := fmt.Sprintf("🚫AugmentGateway系统提示：您的令牌已经超过使用频率限制，可以在%d分钟后重新使用，多次超过使用频率限制，令牌将会被冻结！", resetMinutes)
	h.streamHelper.SendStreamMessage(c, message)
}

// RespondWithMaxRequestsMessage 返回使用次数限制消息（流式响应）
func (h *LimitResponseHandler) RespondWithMaxRequestsMessage(c *gin.Context) {
	message := "🚫AugmentGateway系统提示：您已经达到当前令牌最大使用次数，请联系管理员增加令牌使用额度！"
	h.streamHelper.SendStreamMessage(c, message)
}

// RespondWithDisabledTokenMessage 返回令牌被禁用消息（根据接口类型返回不同格式）
func (h *LimitResponseHandler) RespondWithDisabledTokenMessage(c *gin.Context) {
	// 检查请求路径，判断是否为chat-stream接口
	requestPath := c.Request.URL.Path
	isChatStreamRequest := strings.HasSuffix(requestPath, "/chat-stream")

	if isChatStreamRequest {
		// chat-stream接口返回流式响应
		message := "🚫AugmentGateway系统提示：当前客户端未正常上报数据，为避免账号异常封禁，已经封禁您的令牌，请联系管理员处理！"
		h.streamHelper.SendStreamMessage(c, message)
	} else {
		// 其他接口返回空的JSON响应
		c.JSON(200, gin.H{})
	}
}

// RespondWithNoTokenAvailable 返回无可用TOKEN消息（流式响应）
func (h *LimitResponseHandler) RespondWithNoTokenAvailable(c *gin.Context) {
	message := "🚫 AugmentGateway系统提示：当前暂无可用TOKEN账号。\n\n💡 解决方案：\n1. 提交您的TOKEN账号到平台中\n2. 联系管理员分配TOKEN\n3. 等待其他用户共享TOKEN\n\n📝 提示：提交可共享TOKEN达到一定额度后才可使用共享池中的TOKEN。"
	h.streamHelper.SendStreamMessage(c, message)
}

// RespondWithServiceError 返回令牌未找到
func (h *LimitResponseHandler) RespondWithServiceError(c *gin.Context) {
	message := "🚫AugmentGateway系统提示：令牌信息未找到，请联系管理员处理！"
	h.streamHelper.SendStreamMessage(c, message)
}

// RespondWithVersionTooLow 返回版本过低消息（流式响应）
func (h *LimitResponseHandler) RespondWithVersionTooLow(c *gin.Context, currentVersion, minVersion string) {
	message := fmt.Sprintf("🚫 AugmentGateway系统提示：当前Augment插件版本过低（当前版本：%s），请升级到 %s 或更高版本以继续使用服务！",
		currentVersion, minVersion)
	h.streamHelper.SendStreamMessage(c, message)
}

// RespondWithSharedTokenNoChannel 返回共享TOKEN未绑定外部渠道消息（流式响应）
func (h *LimitResponseHandler) RespondWithSharedTokenNoChannel(c *gin.Context) {
	message := "当前账号必须设置增强渠道才可正常使用！"
	h.streamHelper.SendStreamMessage(c, message)
}

// RespondWithModelNotMapped 返回模型未配置映射消息（流式响应）
func (h *LimitResponseHandler) RespondWithModelNotMapped(c *gin.Context) {
	message := "请先配置当前模型映射，再进行使用！"
	h.streamHelper.SendStreamMessage(c, message)
}

// CacheUserTokenStatus 缓存用户令牌状态
func (h *LimitResponseHandler) CacheUserTokenStatus(ctx context.Context, tokenStr string, status string, ttl time.Duration) error {
	key := fmt.Sprintf("AUGMENT-GATEWAY:user_token_status:%s", tokenStr)
	return h.cacheService.SetString(ctx, key, status, ttl)
}

// GetCachedUserTokenStatus 获取缓存的用户令牌状态
func (h *LimitResponseHandler) GetCachedUserTokenStatus(ctx context.Context, tokenStr string) (string, bool, error) {
	key := fmt.Sprintf("AUGMENT-GATEWAY:user_token_status:%s", tokenStr)
	status, err := h.cacheService.GetString(ctx, key)
	if err != nil {
		return "", false, err
	}
	if status == "" {
		return "", false, nil
	}
	return status, true, nil
}

// ClearUserTokenStatusCache 清除用户令牌状态缓存
func (h *LimitResponseHandler) ClearUserTokenStatusCache(ctx context.Context, tokenStr string) error {
	key := fmt.Sprintf("AUGMENT-GATEWAY:user_token_status:%s", tokenStr)
	return h.cacheService.DeleteKey(ctx, key)
}
