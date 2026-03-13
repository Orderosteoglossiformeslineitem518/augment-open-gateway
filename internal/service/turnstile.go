package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"augment-gateway/internal/config"
	"augment-gateway/internal/logger"
)

// TurnstileService Turnstile验证服务接口
type TurnstileService interface {
	VerifyToken(token, remoteIP string) (*TurnstileResponse, error)
}

// turnstileService Turnstile验证服务实现
type turnstileService struct {
	config *config.TurnstileConfig
	client *http.Client
}

// TurnstileRequest Turnstile验证请求结构
type TurnstileRequest struct {
	Secret   string `json:"secret"`
	Response string `json:"response"`
	RemoteIP string `json:"remoteip,omitempty"`
}

// TurnstileResponse Turnstile验证响应结构
type TurnstileResponse struct {
	Success     bool     `json:"success"`
	ChallengeTS string   `json:"challenge_ts,omitempty"`
	Hostname    string   `json:"hostname,omitempty"`
	ErrorCodes  []string `json:"error-codes,omitempty"`
	Action      string   `json:"action,omitempty"`
	CData       string   `json:"cdata,omitempty"`
}

// NewTurnstileService 创建Turnstile验证服务
func NewTurnstileService(config *config.TurnstileConfig) TurnstileService {
	return &turnstileService{
		config: config,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// VerifyToken 验证Turnstile token
func (s *turnstileService) VerifyToken(token, remoteIP string) (*TurnstileResponse, error) {
	// 如果未启用Turnstile验证，直接返回成功
	if !s.config.Enabled {
		return &TurnstileResponse{
			Success: true,
		}, nil
	}

	// 检查是否为测试token（本地开发用）
	if token == "XXXX.DUMMY.TOKEN.XXXX" {
		return &TurnstileResponse{
			Success: true,
		}, nil
	}

	// 构建验证请求
	requestData := TurnstileRequest{
		Secret:   s.config.SecretKey,
		Response: token,
		RemoteIP: remoteIP,
	}

	// 序列化请求数据
	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return nil, fmt.Errorf("序列化请求数据失败: %w", err)
	}

	// 发送验证请求到Cloudflare
	logger.Infof("[Turnstile] 发送验证请求，IP: %s\n", remoteIP)
	resp, err := s.client.Post(
		"https://challenges.cloudflare.com/turnstile/v0/siteverify",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		logger.Infof("[Turnstile] 发送验证请求失败: %v\n", err)
		return nil, fmt.Errorf("发送验证请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Infof("[Turnstile] 读取响应失败: %v\n", err)
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	logger.Infof("[Turnstile] 收到响应: %s\n", string(body))

	// 解析响应
	var turnstileResp TurnstileResponse
	if err := json.Unmarshal(body, &turnstileResp); err != nil {
		logger.Infof("[Turnstile] 解析响应失败: %v\n", err)
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if !turnstileResp.Success {
		logger.Infof("[Turnstile] 验证失败，错误代码: %v\n", turnstileResp.ErrorCodes)
	} else {
		logger.Infof("[Turnstile] 验证成功\n")
	}

	return &turnstileResp, nil
}
