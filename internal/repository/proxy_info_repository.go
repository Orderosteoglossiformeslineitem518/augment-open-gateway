package repository

import (
	"augment-gateway/internal/database"
	"augment-gateway/internal/logger"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// ProxyInfoRepository 代理信息仓库接口
type ProxyInfoRepository interface {
	Create(proxyInfo *database.ProxyInfo) error
	GetByID(id uint) (*database.ProxyInfo, error)
	List(page, pageSize int, status string, userID *int) ([]*database.ProxyInfo, int64, error)
	Update(proxyInfo *database.ProxyInfo) error
	Delete(id uint) error
	GetUserSubmissionCount(userID int, date time.Time) (int64, error)
	GetUserSubmissions(userID int, page, pageSize int) ([]*database.ProxyInfo, int64, error)
	UpdateStatus(id uint, status, description string) error
	UpdateProxy(id uint, status, description, proxyURL string) error
	GetPendingCount() (int64, error)
	GetValidCount() (int64, error)
	ExistsByProxyURL(proxyURL string) (bool, error)
	GetTokenCountByProxyAddress(proxyAddress string) (int64, error)
	GetLeastUsedProxyAddress() (string, error)
}

// proxyInfoRepository 代理信息仓库实现
type proxyInfoRepository struct {
	db *gorm.DB
}

// NewProxyInfoRepository 创建代理信息仓库
func NewProxyInfoRepository(db *gorm.DB) ProxyInfoRepository {
	return &proxyInfoRepository{
		db: db,
	}
}

// Create 创建代理信息
func (r *proxyInfoRepository) Create(proxyInfo *database.ProxyInfo) error {
	return r.db.Create(proxyInfo).Error
}

// GetByID 根据ID获取代理信息
func (r *proxyInfoRepository) GetByID(id uint) (*database.ProxyInfo, error) {
	var proxyInfo database.ProxyInfo
	err := r.db.Where("id = ?", id).First(&proxyInfo).Error
	if err != nil {
		return nil, err
	}

	// 手动加载关联的用户信息
	proxyInfos := []*database.ProxyInfo{&proxyInfo}
	r.loadUsers(proxyInfos)

	return &proxyInfo, nil
}

// List 获取代理信息列表
func (r *proxyInfoRepository) List(page, pageSize int, status string, userID *int) ([]*database.ProxyInfo, int64, error) {
	var proxyInfos []*database.ProxyInfo
	var total int64

	query := r.db.Model(&database.ProxyInfo{})

	// 状态过滤
	if status != "" {
		query = query.Where("status = ?", status)
	}

	// 用户过滤
	if userID != nil {
		query = query.Where("user_id = ?", *userID)
	}

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("获取代理信息总数失败: %w", err)
	}

	// 分页查询
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&proxyInfos).Error; err != nil {
		return nil, 0, fmt.Errorf("获取代理信息列表失败: %w", err)
	}

	// 手动加载关联的用户信息（批量查询优化）
	r.loadUsers(proxyInfos)

	return proxyInfos, total, nil
}

// Update 更新代理信息
func (r *proxyInfoRepository) Update(proxyInfo *database.ProxyInfo) error {
	return r.db.Save(proxyInfo).Error
}

// Delete 删除代理信息
func (r *proxyInfoRepository) Delete(id uint) error {
	return r.db.Delete(&database.ProxyInfo{}, id).Error
}

// GetUserSubmissionCount 获取用户在指定日期的提交次数
func (r *proxyInfoRepository) GetUserSubmissionCount(userID int, date time.Time) (int64, error) {
	var count int64
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	err := r.db.Model(&database.ProxyInfo{}).
		Where("user_id = ? AND created_at >= ? AND created_at < ?", userID, startOfDay, endOfDay).
		Count(&count).Error

	return count, err
}

// GetUserSubmissions 获取用户的代理提交记录
func (r *proxyInfoRepository) GetUserSubmissions(userID int, page, pageSize int) ([]*database.ProxyInfo, int64, error) {
	var proxyInfos []*database.ProxyInfo
	var total int64

	query := r.db.Model(&database.ProxyInfo{}).Where("user_id = ?", userID)

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("获取用户代理提交总数失败: %w", err)
	}

	// 分页查询
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&proxyInfos).Error; err != nil {
		return nil, 0, fmt.Errorf("获取用户代理提交列表失败: %w", err)
	}

	// 加载关联的用户信息（虽然查询的是特定用户，但为了保持数据完整性）
	r.loadUsers(proxyInfos)

	return proxyInfos, total, nil
}

// UpdateStatus 更新代理状态
func (r *proxyInfoRepository) UpdateStatus(id uint, status, description string) error {
	return r.db.Model(&database.ProxyInfo{}).Where("id = ?", id).Updates(map[string]any{
		"status":      status,
		"description": description,
		"updated_at":  time.Now(),
	}).Error
}

// UpdateProxy 更新代理信息（包括状态、描述和代理地址）
func (r *proxyInfoRepository) UpdateProxy(id uint, status, description, proxyURL string) error {
	updates := map[string]any{
		"status":      status,
		"description": description,
		"updated_at":  time.Now(),
	}

	// 只有当proxyURL不为空时才更新代理地址
	if proxyURL != "" {
		updates["proxy_url"] = proxyURL
	}

	return r.db.Model(&database.ProxyInfo{}).Where("id = ?", id).Updates(updates).Error
}

// GetPendingCount 获取待审核代理数量
func (r *proxyInfoRepository) GetPendingCount() (int64, error) {
	var count int64
	err := r.db.Model(&database.ProxyInfo{}).Where("status = ?", "pending").Count(&count).Error
	return count, err
}

// GetValidCount 获取有效代理数量
func (r *proxyInfoRepository) GetValidCount() (int64, error) {
	var count int64
	err := r.db.Model(&database.ProxyInfo{}).Where("status = ?", "valid").Count(&count).Error
	return count, err
}

// ExistsByProxyURL 检查代理地址是否已存在
func (r *proxyInfoRepository) ExistsByProxyURL(proxyURL string) (bool, error) {
	var count int64
	err := r.db.Model(&database.ProxyInfo{}).Where("proxy_url = ?", proxyURL).Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("检查代理地址是否存在失败: %w", err)
	}
	return count > 0, nil
}

// GetTokenCountByProxyAddress 统计使用指定代理地址的TOKEN数量
func (r *proxyInfoRepository) GetTokenCountByProxyAddress(proxyAddress string) (int64, error) {
	// 从代理地址中提取域名部分，用于匹配租户地址
	// 例如：https://pure-hedgehog-19.deno.dev/ -> pure-hedgehog-19.deno.dev
	domain := strings.TrimPrefix(proxyAddress, "https://")
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimSuffix(domain, "/")

	// 查询租户地址包含该域名的TOKEN数量
	// 使用LIKE查询匹配包含该域名的租户地址
	var count int64
	err := r.db.Table("tokens").
		Where("tenant_address LIKE ? AND status = 'active'", "%"+domain+"%").
		Count(&count).Error

	if err != nil {
		return 0, fmt.Errorf("查询TOKEN使用统计失败: %w", err)
	}

	return count, nil
}

// GetLeastUsedProxyAddress 获取使用次数最少的代理地址
func (r *proxyInfoRepository) GetLeastUsedProxyAddress() (string, error) {
	// 分开查询的实现步骤：
	// 1. 查询A: 从 tokens 表中获取每个 domain 的使用次数
	// 2. 查询B: 从 proxy_infos 表中获取所有状态为 'valid' 的代理及其创建时间
	// 3. 应用层处理: 遍历所有有效代理，查找各自使用次数，按"次数优先，时间其次"规则找出最优

	// 查询A: 获取每个域名的使用次数
	type DomainUsage struct {
		Domain string `json:"domain"`
		Count  int64  `json:"count"`
	}

	var domainUsages []DomainUsage
	queryA := `
		SELECT
			SUBSTRING_INDEX(SUBSTRING_INDEX(tenant_address, '//', -1), '/', 1) as domain,
			COUNT(*) as count
		FROM tokens
		WHERE status = 'active' AND tenant_address LIKE 'https://%'
		GROUP BY domain
	`

	err := r.db.Raw(queryA).Scan(&domainUsages).Error
	if err != nil {
		return "", fmt.Errorf("查询域名使用统计失败: %w", err)
	}

	// 构建域名使用次数映射
	domainCountMap := make(map[string]int64)
	for _, usage := range domainUsages {
		domainCountMap[usage.Domain] = usage.Count
	}

	// 查询B: 获取所有有效的代理信息
	type ValidProxy struct {
		ProxyURL  string    `json:"proxy_url"`
		CreatedAt time.Time `json:"created_at"`
	}

	var validProxies []ValidProxy
	queryB := `
		SELECT proxy_url, created_at
		FROM proxy_infos
		WHERE status = 'valid'
		ORDER BY created_at ASC
	`

	err = r.db.Raw(queryB).Scan(&validProxies).Error
	if err != nil {
		return "", fmt.Errorf("查询有效代理列表失败: %w", err)
	}

	if len(validProxies) == 0 {
		return "", fmt.Errorf("没有找到有效的代理地址")
	}

	// 应用层处理: 找出使用次数最少的代理
	var bestProxy ValidProxy
	minUsage := int64(-1)

	for _, proxy := range validProxies {
		// 从代理URL中提取域名
		domain := r.extractDomainFromProxyURL(proxy.ProxyURL)
		usage := domainCountMap[domain] // 如果不存在，默认为0

		// 按"次数优先，时间其次"的规则选择最优代理
		if minUsage == -1 || usage < minUsage || (usage == minUsage && proxy.CreatedAt.Before(bestProxy.CreatedAt)) {
			minUsage = usage
			bestProxy = proxy
		}
	}

	return bestProxy.ProxyURL, nil
}

// extractDomainFromProxyURL 从代理URL中提取域名
func (r *proxyInfoRepository) extractDomainFromProxyURL(proxyURL string) string {
	domain := strings.TrimPrefix(proxyURL, "https://")
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimSuffix(domain, "/")
	return domain
}

// loadUsers 批量加载用户信息（优化N+1查询问题）
func (r *proxyInfoRepository) loadUsers(proxyInfos []*database.ProxyInfo) {
	// 收集所有需要查询的UserID
	var userIDs []int
	userIDMap := make(map[int][]*database.ProxyInfo) // 支持多个代理关联同一个用户

	for _, proxyInfo := range proxyInfos {
		if proxyInfo.UserID != nil {
			userIDs = append(userIDs, *proxyInfo.UserID)
			userIDMap[*proxyInfo.UserID] = append(userIDMap[*proxyInfo.UserID], proxyInfo)
		}
	}

	// 如果没有需要查询的用户，直接返回
	if len(userIDs) == 0 {
		return
	}

	// 去重userIDs
	uniqueIDs := make([]int, 0, len(userIDMap))
	for id := range userIDMap {
		uniqueIDs = append(uniqueIDs, id)
	}

	// 一次性查询所有相关的用户
	var users []database.User
	if err := r.db.Where("id IN ?", uniqueIDs).Find(&users).Error; err != nil {
		// 查询失败时记录错误但不影响主流程
		logger.Warnf("批量查询用户失败: %v", err)
		return
	}

	// 将查询结果匹配到对应的代理信息
	for i := range users {
		if proxyInfoList, exists := userIDMap[int(users[i].ID)]; exists {
			for _, proxyInfo := range proxyInfoList {
				proxyInfo.User = &users[i]
			}
		}
	}
}
