package handler

import (
	"fmt"

	"augment-gateway/internal/logger"
	"augment-gateway/internal/utils"

	"github.com/gin-gonic/gin"
)

// PoolTokenLimitResponseHandler 号池TOKEN限制响应处理器
type PoolTokenLimitResponseHandler struct{}

// NewPoolTokenLimitResponseHandler 创建号池TOKEN限制响应处理器
func NewPoolTokenLimitResponseHandler() *PoolTokenLimitResponseHandler {
	return &PoolTokenLimitResponseHandler{}
}

// SendPoolTokenLimitExceededResponse 发送号池TOKEN切换次数超限的模拟响应
func (h *PoolTokenLimitResponseHandler) SendPoolTokenLimitExceededResponse(c *gin.Context, currentCount, maxCount int) {
	// 构建提示消息
	message := fmt.Sprintf("AugmentGateway系统提示:您今天已经超过号池TOKEN自动切换额度（每天%d次），当前已切换%d次。请等待24小时后再试，或者提交自己的TOKEN以获得更多使用权限。",
		maxCount, currentCount)

	// 使用流式响应工具发送消息
	helper := utils.NewStreamResponseHelper()
	helper.SendStreamMessage(c, message)

	logger.Infof("[号池TOKEN限制] 用户已超过每日切换限制，返回模拟响应: %d/%d\n", currentCount, maxCount)
}
