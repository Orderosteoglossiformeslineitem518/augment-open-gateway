package handler

import (
	"strconv"

	"augment-gateway/internal/service"

	"github.com/gin-gonic/gin"
)

// RequestRecordHandler 请求记录处理器
type RequestRecordHandler struct {
	requestRecordService *service.RequestRecordService
}

// NewRequestRecordHandler 创建请求记录处理器
func NewRequestRecordHandler(requestRecordService *service.RequestRecordService) *RequestRecordHandler {
	return &RequestRecordHandler{
		requestRecordService: requestRecordService,
	}
}

// List 获取请求记录列表
func (h *RequestRecordHandler) List(c *gin.Context) {
	// 解析查询参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	pathSearch := c.Query("path_search")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	records, total, err := h.requestRecordService.ListRequestRecords(c.Request.Context(), page, pageSize, pathSearch)
	if err != nil {
		ResponseError(c, 500, "获取请求记录列表失败")
		return
	}

	ResponseSuccess(c, gin.H{
		"data": records,
		"pagination": gin.H{
			"page":        page,
			"page_size":   pageSize,
			"total":       total,
			"total_pages": (total + int64(pageSize) - 1) / int64(pageSize),
		},
	})
}

// Search 搜索请求记录
func (h *RequestRecordHandler) Search(c *gin.Context) {
	pathKeyword := c.Query("path")
	if pathKeyword == "" {
		ResponseError(c, 400, "请求路径关键词不能为空")
		return
	}

	records, err := h.requestRecordService.SearchRequestRecords(c.Request.Context(), pathKeyword)
	if err != nil {
		ResponseError(c, 500, "搜索请求记录失败")
		return
	}

	ResponseSuccess(c, records)
}
