package service

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"augment-gateway/internal/config"
	"augment-gateway/internal/database"
	"augment-gateway/internal/logger"

	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

// SubscriptionStatus 订阅状态枚举
type SubscriptionStatus int

const (
	StatusNormal   SubscriptionStatus = 1 // 正常状态
	StatusExpired  SubscriptionStatus = 3 // 过期状态
	StatusDepleted SubscriptionStatus = 4 // 额度耗尽状态
)

// SubscriptionInfo 订阅信息结构
type SubscriptionInfo struct {
	Status               SubscriptionStatus `json:"status"`
	UsageBalanceDepleted bool               `json:"usage_balance_depleted"`
	EndDate              *time.Time         `json:"end_date"`
	SessionID            string             `json:"session_id"` // 验证时使用的会话ID
}

// SubscriptionResponse API响应结构
type SubscriptionResponse struct {
	Subscription struct {
		ActiveSubscription   *ActiveSubscription   `json:"ActiveSubscription,omitempty"`
		InactiveSubscription *InactiveSubscription `json:"InactiveSubscription,omitempty"`
	} `json:"subscription"`
}

// ActiveSubscription 活跃订阅信息
type ActiveSubscription struct {
	EndDate              *string `json:"end_date"`
	UsageBalanceDepleted bool    `json:"usage_balance_depleted"`
}

// InactiveSubscription 非活跃订阅信息
type InactiveSubscription struct{}

// SubscriptionValidator 订阅验证服务
type SubscriptionValidator struct {
	db     *gorm.DB
	client *http.Client
	cache  *CacheService
	config *config.SubscriptionConfig
}

// NewSubscriptionValidator 创建订阅验证服务
func NewSubscriptionValidator(db *gorm.DB, cache *CacheService, cfg *config.SubscriptionConfig) *SubscriptionValidator {
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 5,
			IdleConnTimeout:     30 * time.Second,
		},
	}

	return &SubscriptionValidator{
		db:     db,
		client: client,
		cache:  cache,
		config: cfg,
	}
}

// ValidationError 验证错误类型
type ValidationError struct {
	Message   string
	IsNetwork bool // 是否为网络错误
}

func (e *ValidationError) Error() string {
	return e.Message
}

// NewNetworkError 创建网络错误
func NewNetworkError(message string) *ValidationError {
	return &ValidationError{
		Message:   message,
		IsNetwork: true,
	}
}

// NewAccountError 创建账号错误
func NewAccountError(message string) *ValidationError {
	return &ValidationError{
		Message:   message,
		IsNetwork: false,
	}
}

// ValidateSubscription 验证订阅信息
func (sv *SubscriptionValidator) ValidateSubscription(tenantAddress, token string) (*SubscriptionInfo, error) {
	// 构建请求URL
	url := strings.TrimSuffix(tenantAddress, "/") + "/subscription-info"

	// 创建请求 - 使用POST方法并发送空JSON数据
	requestBody := []byte("{}")
	req, err := http.NewRequest("POST", url, bytes.NewReader(requestBody))
	if err != nil {
		return nil, NewNetworkError("账号验证失败，创建请求失败")
	}

	// 生成sessionID并设置请求头
	sessionID := sv.generateSessionID()
	sv.setRequestHeadersWithSessionID(req, tenantAddress, token, sessionID)

	// 发送请求
	resp, err := sv.client.Do(req)
	if err != nil {
		return nil, NewNetworkError("账号验证失败，网络请求失败")
	}
	defer resp.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, NewNetworkError("账号验证失败， 读取响应失败")
	}

	// 检查HTTP状态码
	if resp.StatusCode != http.StatusOK {
		switch resp.StatusCode {
		case http.StatusUnauthorized: // 401
			return nil, NewAccountError("提交账号无效，远程响应账号过期或被封禁")
		case http.StatusForbidden: // 403
			return nil, NewAccountError("提交账号无效，远程响应账号被封禁")
		default:
			return nil, NewAccountError(fmt.Sprintf("提交账号无效，服务返回错误状态码 %d", resp.StatusCode))
		}
	}

	// 解析响应
	var subscriptionResp SubscriptionResponse
	if err := json.Unmarshal(body, &subscriptionResp); err != nil {
		return nil, NewAccountError("提交账号无效， 响应数据解析失败")
	}

	// 分析订阅状态
	result, err := sv.analyzeSubscriptionStatus(&subscriptionResp)
	if err != nil {
		return nil, NewAccountError(err.Error())
	}

	// 设置sessionID
	result.SessionID = sessionID

	return result, nil
}

// setCommonRequestHeaders 设置通用请求头
func (sv *SubscriptionValidator) setCommonRequestHeaders(req *http.Request, tenantAddress, token, requestID, sessionID string) {
	// 提取主机名
	host := sv.extractHost(tenantAddress)

	// 设置必要的请求头
	req.Header.Set("host", host)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", sv.config.UserAgent)
	req.Header.Set("x-request-id", requestID)
	req.Header.Set("x-request-session-id", sessionID)
	req.Header.Set("x-api-version", "2")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("accept", "*/*")
	req.Header.Set("accept-language", "*")
	req.Header.Set("sec-fetch-mode", "cors")
}

// setRequestHeadersWithSessionID 使用指定的sessionID设置请求头
func (sv *SubscriptionValidator) setRequestHeadersWithSessionID(req *http.Request, tenantAddress, token, sessionID string) {
	// 生成请求ID
	requestID := sv.generateRequestID()

	// 使用通用方法设置请求头
	sv.setCommonRequestHeaders(req, tenantAddress, token, requestID, sessionID)
}

// extractHost 从租户地址中提取主机名
func (sv *SubscriptionValidator) extractHost(tenantAddress string) string {
	// 移除协议前缀
	host := strings.TrimPrefix(tenantAddress, "https://")
	host = strings.TrimPrefix(host, "http://")

	// 移除路径部分
	if idx := strings.Index(host, "/"); idx != -1 {
		host = host[:idx]
	}

	return host
}

// generateRequestID 生成请求ID
func (sv *SubscriptionValidator) generateRequestID() string {
	// 生成UUID格式的请求ID
	bytes := make([]byte, 16)
	rand.Read(bytes)

	// 设置版本和变体位
	bytes[6] = (bytes[6] & 0x0f) | 0x40 // 版本4
	bytes[8] = (bytes[8] & 0x3f) | 0x80 // 变体10

	return fmt.Sprintf("%x-%x-%x-%x-%x",
		bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16])
}

// generateSessionID 生成会话ID
func (sv *SubscriptionValidator) generateSessionID() string {
	// 生成UUID格式的会话ID
	bytes := make([]byte, 16)
	rand.Read(bytes)

	// 设置版本和变体位
	bytes[6] = (bytes[6] & 0x0f) | 0x40 // 版本4
	bytes[8] = (bytes[8] & 0x3f) | 0x80 // 变体10

	return fmt.Sprintf("%x-%x-%x-%x-%x",
		bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16])
}

// analyzeSubscriptionStatus 分析订阅状态
func (sv *SubscriptionValidator) analyzeSubscriptionStatus(resp *SubscriptionResponse) (*SubscriptionInfo, error) {
	info := &SubscriptionInfo{}

	// 检查是否有活跃订阅
	if resp.Subscription.ActiveSubscription != nil {
		active := resp.Subscription.ActiveSubscription

		// 设置额度耗尽状态
		info.UsageBalanceDepleted = active.UsageBalanceDepleted

		// 根据额度耗尽状态设置状态
		if active.UsageBalanceDepleted {
			info.Status = StatusDepleted
		} else {
			info.Status = StatusNormal
		}

		// 解析结束日期
		if active.EndDate != nil && *active.EndDate != "" {
			if endDate, err := time.Parse(time.RFC3339, *active.EndDate); err == nil {
				info.EndDate = &endDate
			}
		}

		return info, nil
	}

	// 检查是否有非活跃订阅
	if resp.Subscription.InactiveSubscription != nil {
		info.Status = StatusExpired
		return info, nil
	}

	return nil, fmt.Errorf("提交账号无效，无法识别的订阅状态")
}

// InvalidSubmissionRecord 无效提交记录结构
type InvalidSubmissionRecord struct {
	Timestamp      int64  `json:"timestamp"`       // 提交时间戳
	SubmissionData string `json:"submission_data"` // 用户提交的JSON数据
}

// RecordInvalidSubmission 记录无效提交到Redis（7天滑动窗口）
func (sv *SubscriptionValidator) RecordInvalidSubmission(userID uint, submissionData string) error {
	// 构建Redis key
	key := fmt.Sprintf("AUGMENT-GATEWAY:invalid_submission:%d", userID)

	// 创建无效提交记录
	record := InvalidSubmissionRecord{
		Timestamp:      time.Now().Unix(),
		SubmissionData: submissionData,
	}

	// 将记录序列化为JSON
	recordJSON, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("序列化无效提交记录失败: %w", err)
	}

	ctx := sv.cache.redis.GetClient().Context()
	now := time.Now().Unix()
	sevenDaysAgo := now - (7 * 24 * 60 * 60) // 7天前的时间戳

	// 使用Redis管道操作，确保原子性
	pipe := sv.cache.redis.GetClient().Pipeline()

	// 1. 清理7天前的记录
	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", sevenDaysAgo))

	// 2. 添加新的无效提交记录，使用时间戳作为分数
	pipe.ZAdd(ctx, key, &redis.Z{Score: float64(now), Member: string(recordJSON)})

	// 3. 设置过期时间为8天，确保数据会被清理
	pipe.Expire(ctx, key, 8*24*time.Hour)

	// 执行管道操作
	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("记录无效提交到Redis失败: %w", err)
	}

	logger.Infof("[订阅验证] 记录无效提交（7天窗口），用户ID: %d，数据: %s\n", userID, submissionData)

	return nil
}

// CheckAndBanUser 检查并封禁用户（如果7天内无效提交次数超过10次）
func (sv *SubscriptionValidator) CheckAndBanUser(userID uint) error {
	// 构建Redis key
	key := fmt.Sprintf("AUGMENT-GATEWAY:invalid_submission:%d", userID)

	ctx := sv.cache.redis.GetClient().Context()
	now := time.Now().Unix()
	sevenDaysAgo := now - (7 * 24 * 60 * 60) // 7天前的时间戳

	// 使用Redis管道操作
	pipe := sv.cache.redis.GetClient().Pipeline()

	// 1. 清理7天前的记录
	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", sevenDaysAgo))

	// 2. 获取7天内的无效提交次数
	pipe.ZCard(ctx, key)

	// 执行管道操作
	results, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("获取7天内无效提交次数失败: %w", err)
	}

	// 获取计数结果
	count := results[1].(*redis.IntCmd).Val()

	logger.Infof("[订阅验证] 用户ID: %d，7天内无效提交次数: %d\n", userID, count)

	// 如果7天内无效提交次数超过10次，封禁用户
	if count >= 10 {
		logger.Infof("[订阅验证] 用户ID: %d 因7天内提交10次无效账号被封禁\n", userID)
		return sv.db.Model(&database.User{}).
			Where("id = ?", userID).
			Update("status", "banned").Error
	}

	return nil
}
