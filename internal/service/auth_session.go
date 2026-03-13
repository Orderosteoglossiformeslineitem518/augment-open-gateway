package service

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

// HTTPClientInterface HTTP客户端接口
type HTTPClientInterface interface {
	Do(req *http.Request) (*http.Response, error)
}

// AuthSessionClient Auth Session API客户端接口
type AuthSessionClient interface {
	ValidateAuthSession(authSession string) error
	AuthDevice(authSession string) (tenantURL, accessToken, email, newAuthSession string, err error)
	AuthAppLogin(authSession string) (appCookieSession string, err error)
	GetSubscriptionInfo(appCookieSession string) (subscriptionInfo map[string]interface{}, err error)
	GetUserBanReason(authSession string) (banReason string, err error)
}

// DefaultAuthSessionClient 默认的Auth Session API客户端实现
type DefaultAuthSessionClient struct {
	noRedirectHTTPClient HTTPClientInterface // 不跟随重定向的HTTP客户端
}

// NewAuthSessionClient 创建新的Auth Session客户端
func NewAuthSessionClient(noRedirectHTTPClient HTTPClientInterface) AuthSessionClient {
	return &DefaultAuthSessionClient{
		noRedirectHTTPClient: noRedirectHTTPClient,
	}
}

// PKCEPair PKCE密钥对
type PKCEPair struct {
	CodeVerifier  string
	CodeChallenge string
}

// generatePKCEPair 生成PKCE密钥对
func (c *DefaultAuthSessionClient) generatePKCEPair() (*PKCEPair, error) {
	// 生成 code verifier (32字节随机数据)
	verifierBytes := make([]byte, 32)
	if _, err := rand.Read(verifierBytes); err != nil {
		return nil, fmt.Errorf("生成code verifier失败: %v", err)
	}
	codeVerifier := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(verifierBytes)

	// 生成 code challenge
	challengeBytes := sha256.Sum256([]byte(codeVerifier))
	codeChallenge := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(challengeBytes[:])

	return &PKCEPair{
		CodeVerifier:  codeVerifier,
		CodeChallenge: codeChallenge,
	}, nil
}

// generateAuthURL 生成授权URL（使用 /auth/continue 接口）
func (c *DefaultAuthSessionClient) generateAuthURL(codeChallenge string) (authURL, state string) {
	// 生成 42 字节随机状态值
	stateBytes := make([]byte, 42)
	if _, err := rand.Read(stateBytes); err != nil {
		state = uuid.New().String()
	} else {
		state = base64.RawURLEncoding.EncodeToString(stateBytes)
	}

	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("code_challenge", codeChallenge)
	params.Set("client_id", "v")
	params.Set("state", state)
	params.Set("prompt", "login")
	params.Set("redirect_uri", "vscode://augment.vscode-augment/auth/result")

	authURL = "https://auth.augmentcode.com/auth/continue?" + params.Encode()
	return authURL, state
}

// InitialState 从 /auth/continue 响应中解析的初始状态
type InitialState struct {
	NeedsSignup bool `json:"needs_signup"`
	Error       *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
	ClientCode *struct {
		Code      string `json:"code"`
		State     string `json:"state"`
		TenantURL string `json:"tenant_url"`
	} `json:"client_code"`
	Email string `json:"email"`
}

// getAuthContinue 通过 /auth/continue 获取授权码和租户URL，同时返回新的session cookie和邮箱
func (c *DefaultAuthSessionClient) getAuthContinue(authURL, authSession string) (authCode, tenantURL, email, newAuthSession string, err error) {
	// 创建请求
	req, err := http.NewRequest("GET", authURL, nil)
	if err != nil {
		return "", "", "", "", fmt.Errorf("创建请求失败: %v", err)
	}

	// 设置请求头
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	req.Header.Set("Cookie", "session="+authSession)

	// 发送请求
	resp, err := c.noRedirectHTTPClient.Do(req)
	if err != nil {
		return "", "", "", "", fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	log.Printf("[Auth Session] getAuthContinue: 响应状态码: %d", resp.StatusCode)

	// 从响应中提取新的session cookie
	newAuthSession = authSession // 默认使用原session
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "session" {
			newAuthSession = cookie.Value
			log.Printf("[Auth Session] getAuthContinue: 获取到新的session cookie, new_session_length: %d", len(newAuthSession))
			break
		}
	}

	// 读取响应体
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return "", "", "", "", fmt.Errorf("读取响应失败: %v", err)
	}
	responseText := buf.String()

	log.Printf("[Auth Session] getAuthContinue: 响应体长度: %d", len(responseText))

	// 解析 window.__INITIAL_STATE__ JSON 对象
	// 使用贪婪匹配到 </script> 之前，因为嵌套 JSON 对象会导致非贪婪匹配提前截断
	re := regexp.MustCompile(`window\.__INITIAL_STATE__\s*=\s*(\{[^<]+\})`)
	matches := re.FindStringSubmatch(responseText)
	if len(matches) < 2 {
		log.Printf("[Auth Session] getAuthContinue: 未匹配到INITIAL_STATE, response_length: %d, response_preview: %s", len(responseText), responseText[:min(500, len(responseText))])
		return "", "", "", "", fmt.Errorf("SESSION无效或账号被封禁")
	}

	// 清理 JSON 字符串（移除可能的分号和空白）
	jsonStr := strings.TrimSpace(matches[1])
	jsonStr = strings.TrimSuffix(jsonStr, ";")

	log.Printf("[Auth Session] getAuthContinue: 解析到的JSON长度: %d, 内容预览: %s", len(jsonStr), jsonStr[:min(500, len(jsonStr))])

	var initialState InitialState
	if err := json.Unmarshal([]byte(jsonStr), &initialState); err != nil {
		log.Printf("[Auth Session] getAuthContinue: JSON解析失败, error: %v, json_preview: %s", err, jsonStr[:min(1000, len(jsonStr))])
		return "", "", "", "", fmt.Errorf("解析初始状态失败: %v", err)
	}

	// 检查是否返回了错误信息
	if initialState.NeedsSignup {
		log.Printf("[Auth Session] getAuthContinue: 服务器返回needs_signup=true，Session可能无效或已过期")
		if initialState.Error != nil {
			log.Printf("[Auth Session] getAuthContinue: 错误类型: %s, 错误信息: %s", initialState.Error.Type, initialState.Error.Message)
		}
		return "", "", "", "", fmt.Errorf("Session无效或已过期")
	}

	// 检查 client_code 是否存在
	if initialState.ClientCode == nil {
		log.Printf("[Auth Session] getAuthContinue: client_code为空")
		if initialState.Error != nil {
			log.Printf("[Auth Session] getAuthContinue: 错误类型: %s, 错误信息: %s", initialState.Error.Type, initialState.Error.Message)
		}
		return "", "", "", "", fmt.Errorf("Session无效或已过期")
	}

	authCode = initialState.ClientCode.Code
	tenantURL = initialState.ClientCode.TenantURL
	email = initialState.Email

	log.Printf("[Auth Session] getAuthContinue: 解析结果 - authCode长度: %d, tenantURL: %s, email: %s", len(authCode), tenantURL, email)

	if authCode == "" || tenantURL == "" {
		log.Printf("[Auth Session] getAuthContinue: 未找到授权码或租户URL, 完整JSON: %s", jsonStr)
		return "", "", "", "", fmt.Errorf("未找到授权码或租户URL")
	}

	return authCode, tenantURL, email, newAuthSession, nil
}

// getAccessToken 获取访问令牌
func (c *DefaultAuthSessionClient) getAccessToken(tenantURL, authCode, codeVerifier string) (accessToken string, err error) {
	// 构造token端点URL
	tokenURL := strings.TrimSuffix(tenantURL, "/") + "/token"

	// 构造请求体（匹配 VSCode 插件格式）
	payload := map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     "v",
		"code_verifier": codeVerifier,
		"redirect_uri":  "vscode://augment.vscode-augment/auth/result",
		"code":          authCode,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("序列化请求体失败: %v", err)
	}

	// 创建请求
	req, err := http.NewRequest("POST", tokenURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %v", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	resp, err := c.noRedirectHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 解析响应
	var tokenResponse map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResponse); err != nil {
		return "", fmt.Errorf("解析响应失败: %v", err)
	}

	if accessTokenInterface, ok := tokenResponse["access_token"]; ok {
		if accessToken, ok := accessTokenInterface.(string); ok {
			return accessToken, nil
		}
	}

	return "", fmt.Errorf("未找到访问令牌")
}

// AuthDevice 通过Auth Session获取Tenant URL、Token和Email，同时返回刷新后的Auth Session
func (c *DefaultAuthSessionClient) AuthDevice(authSession string) (tenantURL, accessToken, email, newAuthSession string, err error) {
	log.Printf("[Auth Session] 开始Auth Device流程, auth_session_length: %d", len(authSession))

	// 1. 生成PKCE密钥对
	pkcePair, err := c.generatePKCEPair()
	if err != nil {
		return "", "", "", "", fmt.Errorf("生成PKCE密钥对失败: %v", err)
	}

	// 2. 生成授权URL
	authURL, _ := c.generateAuthURL(pkcePair.CodeChallenge)

	// 3. 获取授权码和租户URL，同时获取新的session和邮箱
	authCode, tenantURL, email, newAuthSession, err := c.getAuthContinue(authURL, authSession)
	if err != nil {
		return "", "", "", "", fmt.Errorf("获取授权码失败: %v", err)
	}

	// 4. 获取访问令牌
	accessToken, err = c.getAccessToken(tenantURL, authCode, pkcePair.CodeVerifier)
	if err != nil {
		return "", "", "", "", fmt.Errorf("获取访问令牌失败: %v", err)
	}

	log.Printf("[Auth Session] Auth Device流程完成, tenant_url: %s, email: %s, token_length: %d, new_session_length: %d", tenantURL, email, len(accessToken), len(newAuthSession))

	return tenantURL, accessToken, email, newAuthSession, nil
}

// ValidateAuthSession 验证Auth Session是否有效
func (c *DefaultAuthSessionClient) ValidateAuthSession(authSession string) error {
	// 生成PKCE密钥对
	pkcePair, err := c.generatePKCEPair()
	if err != nil {
		return fmt.Errorf("生成PKCE密钥对失败: %v", err)
	}

	// 生成授权URL
	authURL, _ := c.generateAuthURL(pkcePair.CodeChallenge)

	// 尝试通过 /auth/continue 获取授权码，如果成功则说明session有效
	_, _, _, newSession, err := c.getAuthContinue(authURL, authSession)
	if err != nil {
		return fmt.Errorf("auth Session验证失败: %v", err)
	}

	// 如果返回的session与原session相同，说明没有获取到新的session，可能无效
	if newSession == authSession {
		return fmt.Errorf("auth Session无效：未获取到新的session cookie")
	}

	log.Println("[Auth Session] Auth Session验证成功")
	return nil
}

// getLoginRedirectURL 获取登录重定向URL和session cookie
func (c *DefaultAuthSessionClient) getLoginRedirectURL() (redirectURL string, sessionCookie string, err error) {
	req, err := http.NewRequest("GET", "https://app.augmentcode.com/login", nil)
	if err != nil {
		return "", "", fmt.Errorf("创建请求失败: %v", err)
	}

	// 设置完整的请求头，模拟浏览器行为
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Safari/537.36")

	// 使用不跟随重定向的HTTP客户端发送请求
	resp, err := c.noRedirectHTTPClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		bodyBytes, _ := io.ReadAll(resp.Body)
		log.Printf("[Auth Session] getLoginRedirectURL 失败, status_code: %d, response_body: %s", resp.StatusCode, string(bodyBytes))
		return "", "", fmt.Errorf("期望状态码302，实际得到%d", resp.StatusCode)
	}

	redirectURL = resp.Header.Get("Location")
	if redirectURL == "" {
		return "", "", fmt.Errorf("未找到重定向URL")
	}

	// 提取_session cookie
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "_session" {
			sessionCookie = cookie.Value
			break
		}
	}

	return redirectURL, sessionCookie, nil
}

// getAuthorizeRedirectURL 获取授权重定向URL
func (c *DefaultAuthSessionClient) getAuthorizeRedirectURL(authSession, redirectURL string) (authorizeURL string, err error) {
	req, err := http.NewRequest("GET", redirectURL, nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %v", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
	req.Header.Set("Cookie", "session="+authSession)

	// 使用不跟随重定向的HTTP客户端发送请求
	resp, err := c.noRedirectHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	authorizeURL = resp.Header.Get("Location")
	if authorizeURL == "" {
		return "", fmt.Errorf("未找到授权URL")
	}

	return authorizeURL, nil
}

// getAuthCallbackURL 获取认证回调URL，携带session cookie
func (c *DefaultAuthSessionClient) getAuthCallbackURL(callbackURL string, sessionCookie string) (newSessionCookie string, err error) {
	// 如果是相对路径，补全为完整URL
	if strings.HasPrefix(callbackURL, "/") {
		callbackURL = "https://app.augmentcode.com" + callbackURL
	}

	req, err := http.NewRequest("GET", callbackURL, nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %v", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	// 携带从getLoginRedirectURL获取的session cookie
	if sessionCookie != "" {
		req.Header.Set("Cookie", "_session="+sessionCookie)
	}

	// 使用不跟随重定向的HTTP客户端发送请求
	resp, err := c.noRedirectHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 从Set-Cookie头中提取_session cookie
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "_session" {
			return cookie.Value, nil
		}
	}

	return "", fmt.Errorf("未找到_session cookie")
}

// AuthAppLogin 通过Auth Session获取App Cookie Session
func (c *DefaultAuthSessionClient) AuthAppLogin(authSession string) (appCookieSession string, err error) {
	log.Printf("[Auth Session] 开始Auth App Login流程, auth_session_length: %d", len(authSession))

	// 1. 获取登录重定向URL和session cookie
	redirectURL, sessionCookie, err := c.getLoginRedirectURL()
	if err != nil {
		return "", fmt.Errorf("获取登录重定向URL失败: %v", err)
	}

	// 2. 通过 /auth/continue 获取刷新后的session
	pkcePair, err := c.generatePKCEPair()
	if err != nil {
		return "", fmt.Errorf("生成PKCE密钥对失败: %v", err)
	}
	authURL, _ := c.generateAuthURL(pkcePair.CodeChallenge)
	_, _, _, newAuthSession, err := c.getAuthContinue(authURL, authSession)
	if err != nil {
		return "", fmt.Errorf("获取刷新session失败: %v", err)
	}

	// 3. 获取授权重定向URL，使用新的session
	authorizeURL, err := c.getAuthorizeRedirectURL(newAuthSession, redirectURL)
	if err != nil {
		return "", fmt.Errorf("获取授权重定向URL失败: %v", err)
	}

	// 4. 获取认证回调URL，携带session cookie
	appCookieSession, err = c.getAuthCallbackURL(authorizeURL, sessionCookie)
	if err != nil {
		return "", fmt.Errorf("获取认证回调失败: %v", err)
	}

	log.Printf("[Auth Session] Auth App Login流程完成, session_length: %d", len(appCookieSession))

	return appCookieSession, nil
}

// GetSubscriptionInfo 获取订阅信息
func (c *DefaultAuthSessionClient) GetSubscriptionInfo(appCookieSession string) (subscriptionInfo map[string]interface{}, err error) {
	log.Printf("[Auth Session] 开始获取订阅信息, session_length: %d", len(appCookieSession))

	req, err := http.NewRequest("GET", "https://app.augmentcode.com/api/subscription", nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
	req.Header.Set("Cookie", "_session="+appCookieSession)

	resp, err := c.noRedirectHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("请求失败，状态码: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	log.Println("[Auth Session] 获取订阅信息完成")

	return result, nil
}

// GetUserBanReason 通过AuthSession获取用户封禁原因
func (c *DefaultAuthSessionClient) GetUserBanReason(authSession string) (banReason string, err error) {
	log.Printf("[Auth Session] 开始获取用户封禁原因, auth_session_length: %d", len(authSession))

	// 1. 使用authSession换取新的App Cookie Session
	appCookieSession, err := c.AuthAppLogin(authSession)
	if err != nil {
		return "", fmt.Errorf("通过Auth Session获取App Cookie Session失败: %v", err)
	}

	// 2. 使用新的App Cookie Session调用接口获取封禁信息
	req, err := http.NewRequest("GET", "https://app.augmentcode.com/api/user", nil)
	if err != nil {
		return "", fmt.Errorf("创建查询用户封禁信息请求失败: %v", err)
	}

	// 设置请求头
	req.Header.Set("Cookie", "_session="+appCookieSession)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	req.Header.Set("Referer", "https://app.augmentcode.com/account/subscription")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	// 发送请求
	resp, err := c.noRedirectHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("查询用户封禁信息请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取用户封禁信息响应失败: %v", err)
	}

	// 解析JSON响应
	var userInfo struct {
		Suspensions []struct {
			Evidence string `json:"evidence"`
		} `json:"suspensions"`
	}

	if err := json.Unmarshal(body, &userInfo); err != nil {
		return "", fmt.Errorf("解析用户封禁信息响应失败: %v", err)
	}

	// 如果suspensions为空，说明账号未封禁
	if len(userInfo.Suspensions) == 0 {
		return "远程响应账号状态正常，可能为系统误封，请重新提交账号使用！", nil
	}

	// 提取所有封禁原因
	var reasons []string
	for _, suspension := range userInfo.Suspensions {
		if suspension.Evidence != "" {
			reasons = append(reasons, suspension.Evidence)
		}
	}

	if len(reasons) == 0 {
		return "远程响应账号状态正常，可能为系统误封，请重新提交账号使用！", nil
	}

	log.Printf("[Auth Session] 获取用户封禁原因完成, reasons_count: %d", len(reasons))

	return strings.Join(reasons, "; "), nil
}
