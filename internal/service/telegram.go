package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"augment-gateway/internal/logger"
)

// TelegramService Telegram消息推送服务
type TelegramService struct {
	botToken string
	chatID   string
	enabled  bool
	client   *http.Client
}

// NewTelegramService 创建Telegram推送服务
func NewTelegramService(botToken, chatID string, enabled bool) *TelegramService {
	return &TelegramService{
		botToken: botToken,
		chatID:   chatID,
		enabled:  enabled,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// IsEnabled 是否启用Telegram推送
func (s *TelegramService) IsEnabled() bool {
	return s.enabled
}

// sendMessageRequest Telegram sendMessage 请求体
type sendMessageRequest struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode,omitempty"`
}

// SendMessage 发送文本消息到默认Telegram群组
func (s *TelegramService) SendMessage(text string) error {
	return s.SendMessageTo(s.chatID, text)
}

// SendMessageTo 发送文本消息到指定Telegram群组
func (s *TelegramService) SendMessageTo(chatID, text string) error {
	if !s.enabled || s.botToken == "" || chatID == "" {
		return nil
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", s.botToken)

	body := sendMessageRequest{
		ChatID:    chatID,
		Text:      text,
		ParseMode: "Markdown",
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("序列化请求体失败: %w", err)
	}

	resp, err := s.client.Post(url, "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("发送Telegram消息失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Telegram API返回非200状态码: %d, body: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// SendMessageAsync 异步发送消息到默认群组（不阻塞调用方）
func (s *TelegramService) SendMessageAsync(text string) {
	s.SendMessageToAsync(s.chatID, text)
}

// SendMessageToAsync 异步发送消息到指定群组（不阻塞调用方）
func (s *TelegramService) SendMessageToAsync(chatID, text string) {
	if !s.enabled {
		return
	}
	go func() {
		if err := s.SendMessageTo(chatID, text); err != nil {
			logger.Warnf("[Telegram] 发送消息失败: %v", err)
		}
	}()
}
