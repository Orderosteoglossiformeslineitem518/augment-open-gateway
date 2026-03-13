package service

import (
	"augment-gateway/internal/database"
	"augment-gateway/internal/logger"
	"augment-gateway/internal/repository"
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
)

// ProxyInfoService 代理信息服务接口
type ProxyInfoService interface {
	SubmitProxy(userID int, proxyURLs []string) error
	GetUserSubmissions(userID int, page, pageSize int) ([]*database.ProxyInfo, int64, error)
	CanSubmitToday(userID int) (bool, int, error)
	ListProxies(page, pageSize int, status string, userID *int) ([]*database.ProxyInfo, int64, error)
	CreateProxy(proxyURL, description string, userID *int) error
	UpdateProxyStatus(id uint, status, description string) error
	UpdateProxy(id uint, status, description, proxyURL string) error
	ApproveProxy(id uint) error
	RejectProxy(id uint, reason string) error
	DeleteProxy(id uint) error
	ValidateProxyURL(proxyURL string) error
	GetProxyByID(id uint) (*database.ProxyInfo, error)
	CheckProxyExists(proxyURL string) (bool, error)
	GetValidProxyAddresses() ([]string, error)
	GetLeastUsedProxyAddress() (string, error)
}

// proxyInfoService 代理信息服务实现
type proxyInfoService struct {
	repo        repository.ProxyInfoRepository
	cache       *CacheService
	userAuthSvc *UserAuthService
}

// NewProxyInfoService 创建代理信息服务
func NewProxyInfoService(repo repository.ProxyInfoRepository, cache *CacheService, userAuthSvc *UserAuthService) ProxyInfoService {
	return &proxyInfoService{
		repo:        repo,
		cache:       cache,
		userAuthSvc: userAuthSvc,
	}
}

// ValidateProxyURL 验证代理地址格式
func (s *proxyInfoService) ValidateProxyURL(proxyURL string) error {
	// 检查是否为https://开头
	if !strings.HasPrefix(proxyURL, "https://") {
		return fmt.Errorf("代理地址必须以https://开头")
	}

	// 验证URL格式
	parsedURL, err := url.Parse(proxyURL)
	if err != nil {
		return fmt.Errorf("代理地址格式无效: %v", err)
	}

	// 检查域名限制
	hostname := parsedURL.Hostname()
	allowedDomains := []string{".deno.dev", ".deno.net", ".vercel.app", ".supabase.co"}

	isValidDomain := false
	for _, domain := range allowedDomains {
		if strings.HasSuffix(hostname, domain) {
			isValidDomain = true
			break
		}
	}

	if !isValidDomain {
		return fmt.Errorf("不支持的代理")
	}

	return nil
}

// CanSubmitToday 检查用户今天是否还能提交代理
func (s *proxyInfoService) CanSubmitToday(userID int) (bool, int, error) {
	ctx := context.Background()

	// 检查Redis缓存
	cacheKey := fmt.Sprintf("AUGMENT-GATEWAY:proxy_submit_count:%d:%s", userID, time.Now().Format("2006-01-02"))
	countStr, err := s.cache.GetString(ctx, cacheKey)
	if err != nil && err != redis.Nil {
		return false, 0, fmt.Errorf("获取提交次数缓存失败: %w", err)
	}

	var count int
	if countStr != "" {
		if _, err := fmt.Sscanf(countStr, "%d", &count); err != nil {
			count = 0
		}
	}

	// 如果缓存中没有，从数据库查询
	if countStr == "" {
		dbCount, err := s.repo.GetUserSubmissionCount(userID, time.Now())
		if err != nil {
			return false, 0, fmt.Errorf("获取用户提交次数失败: %w", err)
		}
		count = int(dbCount)

		// 更新缓存，过期时间到当天结束
		tomorrow := time.Now().AddDate(0, 0, 1)
		startOfTomorrow := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 0, 0, 0, 0, tomorrow.Location())
		expiration := time.Until(startOfTomorrow)

		s.cache.SetString(ctx, cacheKey, fmt.Sprintf("%d", count), expiration)
	}

	canSubmit := count < 2
	return canSubmit, count, nil
}

// SubmitProxy 提交代理地址
func (s *proxyInfoService) SubmitProxy(userID int, proxyURLs []string) error {
	// 检查单次提交数量限制
	if len(proxyURLs) > 2 {
		return fmt.Errorf("单次最多只能提交2个代理地址，您提交了%d个", len(proxyURLs))
	}

	// 检查今日提交次数
	canSubmit, currentCount, err := s.CanSubmitToday(userID)
	if err != nil {
		return err
	}

	if !canSubmit {
		return fmt.Errorf("您今天已经提交了%d次代理，每天最多只能提交2次", currentCount)
	}

	// 验证代理地址格式和重复性，同时标准化地址格式
	var normalizedURLs []string
	for _, proxyURL := range proxyURLs {
		// 自动补充末尾的斜杠
		if !strings.HasSuffix(proxyURL, "/") {
			proxyURL = proxyURL + "/"
		}

		// 验证格式
		if err := s.ValidateProxyURL(proxyURL); err != nil {
			return err
		}

		// 检查是否已存在
		exists, err := s.CheckProxyExists(proxyURL)
		if err != nil {
			return fmt.Errorf("检查代理地址重复性失败: %w", err)
		}
		if exists {
			return fmt.Errorf("代理地址 %s 已存在，请勿重复提交", proxyURL)
		}

		normalizedURLs = append(normalizedURLs, proxyURL)
	}

	// 创建代理记录
	for _, proxyURL := range normalizedURLs {
		proxyInfo := &database.ProxyInfo{
			ProxyURL: proxyURL,
			UserID:   &userID,
			Status:   "pending",
		}

		if err := s.repo.Create(proxyInfo); err != nil {
			return fmt.Errorf("创建代理记录失败: %w", err)
		}
	}

	// 更新缓存中的提交次数
	ctx := context.Background()
	cacheKey := fmt.Sprintf("AUGMENT-GATEWAY:proxy_submit_count:%d:%s", userID, time.Now().Format("2006-01-02"))
	newCount := currentCount + 1

	tomorrow := time.Now().AddDate(0, 0, 1)
	startOfTomorrow := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 0, 0, 0, 0, tomorrow.Location())
	expiration := time.Until(startOfTomorrow)

	s.cache.SetString(ctx, cacheKey, fmt.Sprintf("%d", newCount), expiration)

	return nil
}

// GetUserSubmissions 获取用户的代理提交记录
func (s *proxyInfoService) GetUserSubmissions(userID int, page, pageSize int) ([]*database.ProxyInfo, int64, error) {
	return s.repo.GetUserSubmissions(userID, page, pageSize)
}

// ListProxies 获取代理列表（管理后台使用）
func (s *proxyInfoService) ListProxies(page, pageSize int, status string, userID *int) ([]*database.ProxyInfo, int64, error) {
	return s.repo.List(page, pageSize, status, userID)
}

// CreateProxy 创建代理（管理后台使用）
func (s *proxyInfoService) CreateProxy(proxyURL, description string, userID *int) error {
	// 自动补充末尾的斜杠
	if !strings.HasSuffix(proxyURL, "/") {
		proxyURL = proxyURL + "/"
	}

	// 验证代理地址格式
	if err := s.ValidateProxyURL(proxyURL); err != nil {
		return err
	}

	// 检查是否已存在
	exists, err := s.CheckProxyExists(proxyURL)
	if err != nil {
		return fmt.Errorf("检查代理地址重复性失败: %w", err)
	}
	if exists {
		return fmt.Errorf("代理地址 %s 已存在，请勿重复添加", proxyURL)
	}

	// 创建代理记录
	proxyInfo := &database.ProxyInfo{
		ProxyURL:    proxyURL,
		UserID:      userID,
		Status:      "valid",
		Description: description,
	}

	return s.repo.Create(proxyInfo)
}

// UpdateProxyStatus 更新代理状态
func (s *proxyInfoService) UpdateProxyStatus(id uint, status, description string) error {
	return s.repo.UpdateStatus(id, status, description)
}

// UpdateProxy 更新代理信息（包括状态、描述和代理地址）
func (s *proxyInfoService) UpdateProxy(id uint, status, description, proxyURL string) error {
	// 如果提供了代理地址，需要验证格式
	if proxyURL != "" {
		// 自动补充末尾的斜杠
		if !strings.HasSuffix(proxyURL, "/") {
			proxyURL = proxyURL + "/"
		}

		if err := s.ValidateProxyURL(proxyURL); err != nil {
			return err
		}
	}

	return s.repo.UpdateProxy(id, status, description, proxyURL)
}

// ApproveProxy 审核通过代理
func (s *proxyInfoService) ApproveProxy(id uint) error {
	// 更新状态为有效
	if err := s.repo.UpdateStatus(id, "valid", "管理员审核通过"); err != nil {
		return fmt.Errorf("更新代理状态失败: %w", err)
	}

	return nil
}

// RejectProxy 审核拒绝代理
func (s *proxyInfoService) RejectProxy(id uint, reason string) error {
	description := "管理员审核拒绝"
	if reason != "" {
		description = fmt.Sprintf("管理员审核拒绝: %s", reason)
	}
	return s.repo.UpdateStatus(id, "invalid", description)
}

// DeleteProxy 删除代理
func (s *proxyInfoService) DeleteProxy(id uint) error {
	return s.repo.Delete(id)
}

// GetProxyByID 根据ID获取代理信息
func (s *proxyInfoService) GetProxyByID(id uint) (*database.ProxyInfo, error) {
	return s.repo.GetByID(id)
}

// CheckProxyExists 检查代理地址是否已存在
func (s *proxyInfoService) CheckProxyExists(proxyURL string) (bool, error) {
	return s.repo.ExistsByProxyURL(proxyURL)
}

// GetValidProxyAddresses 获取所有有效的代理地址
func (s *proxyInfoService) GetValidProxyAddresses() ([]string, error) {
	// 获取所有状态为"valid"的代理信息，不分页获取全部
	proxies, _, err := s.repo.List(1, 1000, "valid", nil)
	if err != nil {
		return nil, fmt.Errorf("获取有效代理地址失败: %w", err)
	}

	// 提取代理地址
	addresses := make([]string, 0, len(proxies))
	for _, proxy := range proxies {
		if proxy.ProxyURL != "" {
			addresses = append(addresses, strings.TrimSpace(proxy.ProxyURL))
		}
	}

	return addresses, nil
}

// GetLeastUsedProxyAddress 获取使用次数最少的代理地址
func (s *proxyInfoService) GetLeastUsedProxyAddress() (string, error) {
	// 直接调用repository层的方法，一次查询搞定
	address, err := s.repo.GetLeastUsedProxyAddress()
	if err != nil {
		return "", err
	}

	logger.Infof("[代理选择] 选择使用次数最少的代理: %s\n", address)
	return address, nil
}
