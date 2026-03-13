package handler

import (
	"augment-gateway/internal/database"
	"augment-gateway/internal/logger"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// OrbCreditSyncClient Orb Portal 积分同步客户端
type OrbCreditSyncClient struct {
	httpClient *http.Client
}

// NewOrbCreditSyncClient 创建 Orb Portal 积分同步客户端
func NewOrbCreditSyncClient() *OrbCreditSyncClient {
	return &OrbCreditSyncClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CustomerFromLinkResponse customer_from_link 接口响应结构
type CustomerFromLinkResponse struct {
	Customer struct {
		AvailableLedgerPricingUnits []struct {
			ID                  string  `json:"id"`
			DisplayName         string  `json:"display_name"`
			Name                string  `json:"name"`
			ShortName           string  `json:"short_name"`
			IsRealWorldCurrency bool    `json:"is_real_world_currency"`
			Symbol              *string `json:"symbol"`
		} `json:"available_ledger_pricing_units"`
		ID            string `json:"id"`
		Email         string `json:"email"`
		LedgerEnabled bool   `json:"ledger_enabled"`
	} `json:"customer"`
}

// LedgerSummaryItem ledger_summary 接口响应项
type LedgerSummaryItem struct {
	AllocationID          *string `json:"allocation_id"`
	Balance               *string `json:"balance"`
	EffectiveDate         string  `json:"effective_date"`
	ExpiryDate            string  `json:"expiry_date"`
	ID                    string  `json:"id"`
	IsActive              bool    `json:"is_active"`
	MaximumInitialBalance string  `json:"maximum_initial_balance"`
	PerUnitCostBasis      string  `json:"per_unit_cost_basis"`
}

// LedgerSummaryResponse ledger_summary 接口响应结构
type LedgerSummaryResponse struct {
	CreditBlocks   []LedgerSummaryItem `json:"credit_blocks"`
	CreditsBalance string              `json:"credits_balance"`
}

// SyncCreditFromPortal 从 Portal URL 同步积分信息（兜底方案）
func (c *OrbCreditSyncClient) SyncCreditFromPortal(ctx context.Context, token *database.Token) (usedRequests int, maxRequests int, err error) {
	// 检查 Token 是否有 Portal URL
	if token.PortalURL == nil || *token.PortalURL == "" {
		return 0, 0, fmt.Errorf("TOKEN 没有配置 Portal URL")
	}

	portalURL := *token.PortalURL

	// 1. 从 Portal URL 中提取 token 参数
	portalToken, err := c.extractTokenFromPortalURL(portalURL)
	if err != nil {
		return 0, 0, fmt.Errorf("提取 Portal token 失败: %w", err)
	}

	// 2. 调用 customer_from_link 接口获取 pricing_unit_id
	pricingUnitID, customerID, err := c.getCustomerInfo(ctx, portalToken)
	if err != nil {
		return 0, 0, fmt.Errorf("获取客户信息失败: %w", err)
	}

	// 3. 调用 ledger_summary 接口获取积分余额
	remainingBalance, err := c.getLedgerSummary(ctx, customerID, pricingUnitID, portalToken)
	if err != nil {
		return 0, 0, fmt.Errorf("获取积分余额失败: %w", err)
	}

	// 4. 计算已使用的积分（总额度使用 token.MaxRequests）
	used := token.MaxRequests - int(remainingBalance)
	total := token.MaxRequests

	logger.Infof("[Orb积分同步] ✅ Portal同步成功，TOKEN: %s..., 总额: %d, 已用: %d\n",
		token.Token[:min(8, len(token.Token))], total, used)

	return used, total, nil
}

// extractTokenFromPortalURL 从 Portal URL 中提取 token 参数
func (c *OrbCreditSyncClient) extractTokenFromPortalURL(portalURL string) (string, error) {
	// 解析 URL
	parsedURL, err := url.Parse(portalURL)
	if err != nil {
		return "", fmt.Errorf("解析 Portal URL 失败: %w", err)
	}

	// 提取 token 参数
	token := parsedURL.Query().Get("token")
	if token == "" {
		return "", fmt.Errorf("Portal URL 中没有找到 token 参数")
	}

	return token, nil
}

// getCustomerInfo 调用 customer_from_link 接口获取客户信息
func (c *OrbCreditSyncClient) getCustomerInfo(ctx context.Context, portalToken string) (pricingUnitID string, customerID string, err error) {
	// 构建请求 URL
	requestURL := fmt.Sprintf("https://portal.withorb.com/api/v1/customer_from_link?token=%s", portalToken)

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "GET", requestURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置请求头（只保留必要的）
	req.Header.Set("accept", "application/json, text/plain, */*")
	req.Header.Set("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Safari/537.36")

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 检查状态码
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("请求失败，状态码: %d, 响应: %s", resp.StatusCode, string(bodyBytes))
	}

	// 读取响应
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("读取响应失败: %w", err)
	}

	// 解析响应
	var response CustomerFromLinkResponse
	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		return "", "", fmt.Errorf("解析响应失败: %w", err)
	}

	// 检查是否有 available_ledger_pricing_units
	if len(response.Customer.AvailableLedgerPricingUnits) == 0 {
		return "", "", fmt.Errorf("客户没有可用的 ledger pricing units")
	}

	// 获取第一个 pricing_unit_id
	pricingUnitID = response.Customer.AvailableLedgerPricingUnits[0].ID
	customerID = response.Customer.ID

	return pricingUnitID, customerID, nil
}

// getLedgerSummary 调用 ledger_summary 接口获取积分余额
func (c *OrbCreditSyncClient) getLedgerSummary(ctx context.Context, customerID, pricingUnitID, portalToken string) (remainingBalance float64, err error) {
	// 构建请求 URL
	requestURL := fmt.Sprintf("https://portal.withorb.com/api/v1/customers/%s/ledger_summary?pricing_unit_id=%s&token=%s",
		customerID, pricingUnitID, portalToken)

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "GET", requestURL, nil)
	if err != nil {
		return 0, fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置请求头（只保留必要的）
	req.Header.Set("accept", "application/json, text/plain, */*")
	req.Header.Set("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Safari/537.36")

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 检查状态码
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("请求失败，状态码: %d, 响应: %s", resp.StatusCode, string(bodyBytes))
	}

	// 读取响应
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("读取响应失败: %w", err)
	}

	// 解析响应
	var response LedgerSummaryResponse
	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		return 0, fmt.Errorf("解析响应失败: %w", err)
	}

	// 解析剩余额度
	var balance float64
	if _, err := fmt.Sscanf(response.CreditsBalance, "%f", &balance); err != nil {
		return 0, fmt.Errorf("解析剩余额度失败: %w", err)
	}

	return balance, nil
}
