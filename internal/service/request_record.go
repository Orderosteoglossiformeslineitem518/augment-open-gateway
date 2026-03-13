package service

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"augment-gateway/internal/database"
	"augment-gateway/internal/logger"
	"augment-gateway/internal/repository"

	"gorm.io/gorm"
)

// RequestRecordService 请求记录服务
type RequestRecordService struct {
	db         *gorm.DB
	repository repository.RequestRecordRepository
}

// NewRequestRecordService 创建请求记录服务
func NewRequestRecordService(db *gorm.DB) *RequestRecordService {
	return &RequestRecordService{
		db:         db,
		repository: repository.NewRequestRecordRepository(db),
	}
}

// RecordRequest 记录客户端请求（异步执行）
func (s *RequestRecordService) RecordRequest(req *http.Request, clientIP string) {
	// 异步执行，不阻塞主流程
	go func() {
		ctx := context.Background()
		if err := s.recordRequestSync(ctx, req, clientIP); err != nil {
			logger.Warnf("Warning: 记录请求失败: %v\n", err)
		}
	}()
}

// recordRequestSync 同步记录请求
func (s *RequestRecordService) recordRequestSync(ctx context.Context, req *http.Request, clientIP string) error {
	// 提取请求参数
	requestParams := s.extractRequestParams(req)

	// 提取请求头（过滤敏感信息）
	requestHeaders := s.extractRequestHeaders(req)

	// 创建请求记录
	record := &database.RequestRecord{
		Path:           req.URL.Path,
		Method:         req.Method,
		RequestParams:  requestParams,
		RequestHeaders: requestHeaders,
		UserAgent:      req.UserAgent(),
		ClientIP:       clientIP,
	}

	// 如果记录不存在则创建
	return s.repository.CreateIfNotExists(ctx, record)
}

// extractRequestParams 提取请求参数
func (s *RequestRecordService) extractRequestParams(req *http.Request) string {
	params := make(map[string]interface{})

	// URL查询参数
	if req.URL.RawQuery != "" {
		params["query"] = req.URL.RawQuery
	}

	// 表单参数（仅对POST/PUT等方法）
	if req.Method == "POST" || req.Method == "PUT" || req.Method == "PATCH" {
		if err := req.ParseForm(); err == nil {
			if len(req.PostForm) > 0 {
				formData := make(map[string][]string)
				for key, values := range req.PostForm {
					formData[key] = values
				}
				params["form"] = formData
			}
		}
	}

	if len(params) == 0 {
		return ""
	}

	// 转换为JSON字符串
	if data, err := json.Marshal(params); err == nil {
		return string(data)
	}

	return ""
}

// extractRequestHeaders 提取请求头（过滤敏感信息）
func (s *RequestRecordService) extractRequestHeaders(req *http.Request) string {
	headers := make(map[string]string)

	// 需要过滤的敏感头部
	sensitiveHeaders := map[string]bool{
		"authorization": true,
		"cookie":        true,
		"x-api-key":     true,
		"x-auth-token":  true,
	}

	for name, values := range req.Header {
		lowerName := strings.ToLower(name)
		if sensitiveHeaders[lowerName] {
			headers[name] = "[FILTERED]"
		} else if len(values) > 0 {
			headers[name] = values[0] // 只保存第一个值
		}
	}

	// 转换为JSON字符串
	if data, err := json.Marshal(headers); err == nil {
		return string(data)
	}

	return ""
}

// ListRequestRecords 获取请求记录列表
func (s *RequestRecordService) ListRequestRecords(ctx context.Context, page, pageSize int, pathSearch string) ([]*database.RequestRecord, int64, error) {
	return s.repository.ListWithPagination(ctx, page, pageSize, pathSearch)
}

// SearchRequestRecords 搜索请求记录
func (s *RequestRecordService) SearchRequestRecords(ctx context.Context, pathKeyword string) ([]*database.RequestRecord, error) {
	return s.repository.SearchByPath(ctx, pathKeyword)
}
