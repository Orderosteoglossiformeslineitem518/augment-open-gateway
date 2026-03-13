package handler

import (
	"strconv"

	"augment-gateway/internal/service"

	"github.com/gin-gonic/gin"
)

// StatsHandler 统计处理器
type StatsHandler struct {
	statsService *service.StatsService
}

// NewStatsHandler 创建统计处理器
func NewStatsHandler(statsService *service.StatsService) *StatsHandler {
	return &StatsHandler{
		statsService: statsService,
	}
}

// Overview 获取统计概览
func (h *StatsHandler) Overview(c *gin.Context) {
	stats, err := h.statsService.GetOverview(c.Request.Context())
	if err != nil {
		ResponseError(c, 500, "获取统计概览失败")
		return
	}

	ResponseSuccess(c, stats)
}

// GetTokenStats 获取指定token的统计
func (h *StatsHandler) TokenStats(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		ResponseError(c, 400, "Token ID不能为空")
		return
	}

	stats, err := h.statsService.GetTokenStats(c.Request.Context(), id)
	if err != nil {
		ResponseError(c, 404, "获取Token统计失败")
		return
	}

	ResponseSuccess(c, stats)
}

// Usage 获取使用历史
func (h *StatsHandler) Usage(c *gin.Context) {
	tokenIDStr := c.Query("token_id")
	daysStr := c.DefaultQuery("days", "30")

	days, err := strconv.Atoi(daysStr)
	if err != nil || days < 1 || days > 365 {
		days = 30
	}

	if tokenIDStr != "" {
		// 获取指定token的使用历史
		if tokenIDStr == "" {
			ResponseError(c, 400, "Token ID不能为空")
			return
		}

		usage, err := h.statsService.GetUsageHistory(c.Request.Context(), tokenIDStr, days)
		if err != nil {
			ResponseError(c, 500, "获取使用历史失败")
			return
		}

		ResponseSuccess(c, usage)
	} else {
		// 获取top tokens
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "5"))
		if limit < 1 || limit > 100 {
			limit = 10
		}

		topTokens, err := h.statsService.GetTopTokens(c.Request.Context(), limit)
		if err != nil {
			ResponseError(c, 500, "获取热门Token失败")
			return
		}

		ResponseSuccess(c, topTokens)
	}
}

// Trend 获取请求趋势
func (h *StatsHandler) Trend(c *gin.Context) {
	daysStr := c.DefaultQuery("days", "7")
	days, err := strconv.Atoi(daysStr)
	if err != nil || days < 1 || days > 365 {
		days = 7
	}

	trend, err := h.statsService.GetRequestTrend(c.Request.Context(), days)
	if err != nil {
		ResponseError(c, 500, "获取请求趋势失败")
		return
	}

	ResponseSuccess(c, trend)
}

// Cleanup 清理旧日志
func (h *StatsHandler) Cleanup(c *gin.Context) {
	daysStr := c.DefaultQuery("days", "30")
	days, err := strconv.Atoi(daysStr)
	if err != nil || days < 1 {
		ResponseError(c, 400, "天数参数格式错误")
		return
	}

	deletedCount, err := h.statsService.CleanupOldLogs(c.Request.Context(), days)
	if err != nil {
		ResponseError(c, 500, "清理旧日志失败")
		return
	}

	ResponseSuccessWithMsg(c, "清理完成", gin.H{
		"deleted_count": deletedCount,
	})
}
