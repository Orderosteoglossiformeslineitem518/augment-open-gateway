package proxy

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"augment-gateway/internal/config"
	"augment-gateway/internal/database"
	"augment-gateway/internal/logger"

	"golang.org/x/net/proxy"
)

// ProxyService 代理服务
type ProxyService struct {
	client     *http.Client
	config     *config.Config
	clientPool map[string]*http.Client // 客户端池，key为代理配置的hash
	poolMutex  sync.RWMutex            // 保护客户端池的读写锁
}

// NewProxyService 创建代理服务
func NewProxyService(cfg *config.Config) *ProxyService {
	logger.Infof("  - Timeout: %v", cfg.Proxy.Timeout)
	logger.Infof("  - MaxIdleConns: %d", cfg.Proxy.MaxIdleConns)
	logger.Infof("  - MaxIdleConnsPerHost: %d", cfg.Proxy.MaxIdleConnsPerHost)
	logger.Infof("  - IdleConnTimeout: %v", cfg.Proxy.IdleConnTimeout)

	// 创建优化的HTTP传输层，减少连接建立开销
	transport := &http.Transport{
		MaxIdleConns:        cfg.Proxy.MaxIdleConns,
		MaxIdleConnsPerHost: cfg.Proxy.MaxIdleConnsPerHost,
		IdleConnTimeout:     cfg.Proxy.IdleConnTimeout,
		// 禁用HTTP/2，强制使用HTTP/1.1
		ForceAttemptHTTP2: false,
		// 设置TLS配置，禁用HTTP/2
		TLSNextProto: make(map[string]func(authority string, c *tls.Conn) http.RoundTripper),
		// 优化连接建立
		DisableCompression: false, // 启用压缩以减少传输量
		MaxConnsPerHost:    100,   // 每个主机的最大连接数
	}

	client := &http.Client{
		Timeout:   cfg.Proxy.Timeout,
		Transport: transport,
	}

	return &ProxyService{
		client:     client,
		config:     cfg,
		clientPool: make(map[string]*http.Client),
		poolMutex:  sync.RWMutex{},
	}
}

// ProxyRequest 代理请求结构
type ProxyRequest struct {
	Token         *database.Token
	Method        string
	Path          string
	Headers       http.Header
	Body          []byte
	ClientIP      string
	UserAgent     string
	TenantAddress string
	SessionID     string
}

// ProxyResponse 代理响应结构
type ProxyResponse struct {
	StatusCode   int
	Headers      http.Header
	Body         []byte
	Size         int64
	Latency      time.Duration
	ErrorMessage string
}

// hasProxyConfigured 检查TOKEN是否配置了代理
func (p *ProxyService) hasProxyConfigured(token *database.Token) bool {
	return token != nil && token.ProxyURL != nil && *token.ProxyURL != ""
}

// getClientPoolKey 生成客户端池的key
func (p *ProxyService) getClientPoolKey(token *database.Token) string {
	if !p.hasProxyConfigured(token) {
		return "direct" // 直连使用固定key
	}
	return *token.ProxyURL // 使用代理URL作为key
}

// createHTTPClientWithProxy 根据代理配置获取或创建HTTP客户端（使用连接池优化）
func (p *ProxyService) createHTTPClientWithProxy(token *database.Token, timeout time.Duration) (*http.Client, error) {
	// 生成客户端池的key
	poolKey := p.getClientPoolKey(token)

	// 先尝试从池中获取现有客户端
	p.poolMutex.RLock()
	if existingClient, exists := p.clientPool[poolKey]; exists {
		p.poolMutex.RUnlock()
		// 更新超时时间（如果需要）
		existingClient.Timeout = timeout
		return existingClient, nil
	}
	p.poolMutex.RUnlock()

	// 池中没有，需要创建新客户端
	p.poolMutex.Lock()
	defer p.poolMutex.Unlock()

	// 双重检查，防止并发创建
	if existingClient, exists := p.clientPool[poolKey]; exists {
		existingClient.Timeout = timeout
		return existingClient, nil
	}

	// 创建新的HTTP客户端
	client, err := p.createNewHTTPClient(token, timeout)
	if err != nil {
		return nil, err
	}

	// 存储到池中
	p.clientPool[poolKey] = client
	logger.Infof("[代理服务] 创建并缓存新的HTTP客户端，池key: %s, 当前池大小: %d", poolKey, len(p.clientPool))

	return client, nil
}

// createNewHTTPClient 创建新的HTTP客户端（内部方法）
func (p *ProxyService) createNewHTTPClient(token *database.Token, timeout time.Duration) (*http.Client, error) {
	// 克隆基础transport
	transport := p.client.Transport.(*http.Transport).Clone()

	// 检查token是否配置了代理
	if p.hasProxyConfigured(token) {
		proxyURLStr := *token.ProxyURL
		logger.Infof("[代理服务] TOKEN %s... using proxy: %s",
			token.Token[:min(8, len(token.Token))], proxyURLStr)

		proxyURL, err := url.Parse(proxyURLStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse proxy URL: %w", err)
		}

		switch proxyURL.Scheme {
		case "http", "https":
			// HTTP代理
			transport.Proxy = http.ProxyURL(proxyURL)
			logger.Infof("[代理路径替换] Configured HTTP proxy: %s", proxyURL.Host)

		case "socks5":
			// SOCKS5代理
			dialer, err := proxy.SOCKS5("tcp", proxyURL.Host, nil, proxy.Direct)
			if err != nil {
				return nil, fmt.Errorf("failed to create SOCKS5 dialer: %w", err)
			}

			// 创建自定义的DialContext函数
			transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			}
			logger.Infof("[代理路径替换] Configured SOCKS5 proxy: %s", proxyURL.Host)

		default:
			return nil, fmt.Errorf("unsupported proxy scheme: %s", proxyURL.Scheme)
		}
	}

	// 确保禁用HTTP/2
	transport.ForceAttemptHTTP2 = false
	transport.TLSNextProto = make(map[string]func(authority string, c *tls.Conn) http.RoundTripper)

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}, nil
}

// ForwardStreamWithCapture 流式转发请求到目标服务器并捕获响应内容
func (p *ProxyService) ForwardStreamWithCapture(ctx context.Context, req *ProxyRequest, responseWriter http.ResponseWriter) ([]byte, error) {
	startTime := time.Now()

	// 构建目标URL
	targetURL, err := p.buildTargetURL(req.TenantAddress, req.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to build target URL: %w", err)
	}

	// 根据token的proxy_url配置创建特定的http client
	client, err := p.createHTTPClientWithProxy(req.Token, 30*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client with proxy: %w", err)
	}

	// 创建HTTP请求
	httpReq, err := p.createHTTPRequest(ctx, req, targetURL)
	if err != nil {
		return nil, fmt.Errorf("创建HTTP请求失败: %w", err)
	}

	logger.Infof("[代理服务] 发送流式请求到 %s", targetURL)

	// 发送请求
	resp, err := client.Do(httpReq)
	if err != nil {
		latency := time.Since(startTime)
		logger.Errorf("[代理服务] 流式请求失败 - 耗时: %v, 错误: %v", latency, err)
		return nil, fmt.Errorf("转发流式请求失败: %w", err)
	}
	defer resp.Body.Close()

	logger.Infof("[代理服务] 收到流式响应: %d %s", resp.StatusCode, resp.Status)

	// 检查401或403状态码，表示TOKEN被封禁
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		logger.Warnf("[代理服务] 检测到%d状态码，TOKEN被封禁", resp.StatusCode)
		return nil, fmt.Errorf("token banned: received %d status code", resp.StatusCode)
	}

	// 如果是502错误，读取一部分响应体并打印详细信息
	if resp.StatusCode == http.StatusBadGateway {
		p.logBadGatewayStreamResponse(req, resp, time.Since(startTime))
	}

	// 确保响应写入器支持刷新
	flusher, ok := responseWriter.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("response writer does not support flushing")
	}

	// 创建带超时的上下文用于数据传输
	streamCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	// 使用动态缓冲区进行流式复制，同时捕获内容
	// 针对AI对话场景优化，小缓冲区降低延迟
	bufferSize := 16 * 1024 // 16KB缓冲区，适合AI流式对话
	if strings.Contains(req.Path, "chat") || strings.Contains(req.Path, "stream") {
		bufferSize = 8 * 1024 // AI对话场景使用8KB更低延迟
	}
	buffer := make([]byte, bufferSize)
	var capturedContent []byte // 捕获的响应内容
	totalBytes := int64(0)
	lastDataTime := time.Now()

	logger.Infof("[代理服务] 使用缓冲区大小: %d bytes, 路径: %s (带内容捕获)", bufferSize, req.Path)

	// --- 新增变量 ---
	// 用于确保响应头只被写入一次
	var headersWritten bool

	for {
		select {
		case <-streamCtx.Done():
			if errors.Is(streamCtx.Err(), context.Canceled) {
				logger.Infof("[代理服务] 客户端取消流式传输 - 总字节数: %d, 耗时: %v", totalBytes, time.Since(startTime))
				return capturedContent, nil
			}
			logger.Warnf("[代理服务] 流式传输超时: %v", streamCtx.Err())
			return capturedContent, streamCtx.Err()
		default:
		}

		if time.Since(lastDataTime) > 2*time.Minute {
			logger.Infof("[代理服务] 流式传输空闲超时")
			break
		}

		// 从远端读取数据
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			lastDataTime = time.Now()
			totalBytes += int64(n)

			// 捕获响应内容
			capturedContent = append(capturedContent, buffer[:n]...)

			// 在写入第一块数据之前，先检查并写入响应头
			if !headersWritten {
				// 复制响应头到客户端，但排除与流式传输冲突的头部
				for key, values := range resp.Header {
					// 跳过与 Transfer-Encoding: chunked 冲突的头部
					if strings.ToLower(key) == "content-length" {
						continue
					}
					for _, value := range values {
						responseWriter.Header().Add(key, value)
					}
				}
				// 强制设置流式响应头，确保所有响应都是流式的
				responseWriter.Header().Set("Transfer-Encoding", "chunked")
				responseWriter.Header().Set("Cache-Control", "no-cache")
				responseWriter.Header().Set("Connection", "keep-alive")
				// 设置响应状态码
				responseWriter.WriteHeader(resp.StatusCode)
				headersWritten = true // 标记为已写入
			}

			// 写入到客户端
			if _, writeErr := responseWriter.Write(buffer[:n]); writeErr != nil {
				if strings.Contains(writeErr.Error(), "broken pipe") || strings.Contains(writeErr.Error(), "connection reset") {
					logger.Infof("[代理服务] Client disconnected, total bytes: %d, duration: %v", totalBytes, time.Since(startTime))
					return capturedContent, nil
				}
				logger.Errorf("[代理服务] Failed to write to client: %v", writeErr)
				return capturedContent, writeErr
			}

			// 立即刷新数据到客户端
			flusher.Flush()
		}

		if err != nil {
			if err == io.EOF {
				// --- 处理边缘情况：响应体为空 ---
				if !headersWritten {
					// 复制响应头到客户端，但排除与流式传输冲突的头部
					for key, values := range resp.Header {
						// 跳过与 Transfer-Encoding: chunked 冲突的头部
						if strings.ToLower(key) == "content-length" {
							continue
						}
						for _, value := range values {
							responseWriter.Header().Add(key, value)
						}
					}
					responseWriter.Header().Set("Transfer-Encoding", "chunked")
					responseWriter.Header().Set("Cache-Control", "no-cache")
					responseWriter.Header().Set("Connection", "keep-alive")
					responseWriter.WriteHeader(resp.StatusCode)
				}
				// --- 边缘情况处理结束 ---
				logger.Infof("[代理服务] 流式传输成功完成 - 总字节数: %d, 耗时: %v", totalBytes, time.Since(startTime))
				break
			}
			if errors.Is(err, context.Canceled) {
				logger.Infof("[代理服务] 客户端取消流式传输 - 总字节数: %d, 耗时: %v", totalBytes, time.Since(startTime))
				return capturedContent, nil
			}
			logger.Errorf("[代理服务] 流式读取错误: %v", err)
			return capturedContent, fmt.Errorf("failed to read stream data: %w", err)
		}
	}

	// 确保所有数据都已刷新到客户端
	flusher.Flush()
	time.Sleep(100 * time.Millisecond)

	return capturedContent, nil
}

// buildTargetURL 构建目标URL
func (p *ProxyService) buildTargetURL(tenantAddress, path string) (string, error) {

	// 确保租户地址以http://或https://开头
	if !strings.HasPrefix(tenantAddress, "http://") && !strings.HasPrefix(tenantAddress, "https://") {
		tenantAddress = "https://" + tenantAddress
	}

	// 解析租户地址
	baseURL, err := url.Parse(tenantAddress)
	if err != nil {
		return "", fmt.Errorf("invalid tenant address: %w", err)
	}

	// 清理路径 - 移除/proxy前缀
	//originalPath := path
	path = strings.TrimPrefix(path, "/proxy")
	if path == "" {
		path = "/"
	}

	// 根据Augment API文档，需要保持原始路径结构
	// 如果路径不是以/开头，添加/
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	// 分离路径和查询参数
	var targetPath, rawQuery string
	if idx := strings.Index(path, "?"); idx != -1 {
		targetPath = path[:idx]
		rawQuery = path[idx+1:]
	} else {
		targetPath = path
	}

	// 手动构建完整URL，确保正确拼接基础路径和目标路径
	// 获取基础URL的路径部分，移除尾部斜杠
	basePath := strings.TrimSuffix(baseURL.Path, "/")

	// 清理目标路径，移除开头斜杠
	cleanTargetPath := strings.TrimPrefix(targetPath, "/")

	// 构建完整路径，处理各种边缘情况
	var finalPath string
	if basePath == "" {
		// 基础路径为空，直接使用目标路径
		if cleanTargetPath == "" {
			finalPath = "/"
		} else {
			finalPath = "/" + cleanTargetPath
		}
	} else {
		// 基础路径不为空，拼接路径
		if cleanTargetPath == "" {
			finalPath = basePath + "/"
		} else {
			finalPath = basePath + "/" + cleanTargetPath
		}
	}

	// 额外安全检查：清理可能的双斜杠（除了协议后的//）
	finalPath = strings.ReplaceAll(finalPath, "//", "/")
	// 确保路径以/开头
	if !strings.HasPrefix(finalPath, "/") {
		finalPath = "/" + finalPath
	}

	// 构建最终URL
	finalURL := &url.URL{
		Scheme:   baseURL.Scheme,
		Host:     baseURL.Host,
		Path:     finalPath,
		RawQuery: rawQuery,
	}

	result := finalURL.String()
	return result, nil
}

// createHTTPRequest 创建HTTP请求
func (p *ProxyService) createHTTPRequest(ctx context.Context, req *ProxyRequest, targetURL string) (*http.Request, error) {
	var body io.Reader
	if len(req.Body) > 0 {
		body = bytes.NewReader(req.Body)
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, targetURL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 复制原始请求头
	for key, values := range req.Headers {
		// 跳过一些不应该转发的头
		if p.shouldSkipHeader(key) {
			continue
		}
		for _, value := range values {
			httpReq.Header.Add(key, value)
		}
	}

	// 添加代理相关的头
	p.addProxyHeaders(httpReq, req)

	return httpReq, nil
}

// shouldSkipHeader 检查是否应该跳过某个头
func (p *ProxyService) shouldSkipHeader(key string) bool {
	key = strings.ToLower(key)
	skipHeaders := []string{
		"proxy-connection",
		"proxy-authenticate",
		"proxy-authorization",
		"authorization", // 跳过原始的Authorization头，我们会在addProxyHeaders中设置正确的TOKEN
		"te",
		"trailers",
		"transfer-encoding",
		"Baggage",
		"Sentry-Trace",
		// Cloudflare相关请求头过滤
		"cf-ray",                             // Cloudflare Ray ID
		"cf-visitor",                         // Cloudflare访问者信息
		"cf-connecting-ip",                   // Cloudflare连接IP
		"cf-ipcountry",                       // Cloudflare IP国家
		"cf-request-id",                      // Cloudflare请求ID
		"cf-worker",                          // Cloudflare Worker信息
		"cf-cache-status",                    // Cloudflare缓存状态
		"cf-edge-cache",                      // Cloudflare边缘缓存
		"cf-zone-id",                         // Cloudflare Zone ID
		"cf-railgun",                         // Cloudflare Railgun
		"cf-warp-tag-id",                     // Cloudflare Warp标签ID
		"cf-access-authenticated-user-email", // Cloudflare Access用户邮箱
		"cf-access-jwt-assertion",            // Cloudflare Access JWT断言
		"cf-access-client-id",                // Cloudflare Access客户端ID
		"cf-access-client-secret",            // Cloudflare Access客户端密钥
		"cf-team-domain",                     // Cloudflare团队域名
		"cf-access-token",                    // Cloudflare Access令牌
		"cf-super-bot-protection",            // Cloudflare超级机器人保护
		"cf-bot-management-verified-bot",     // Cloudflare机器人管理验证
		"cf-threat-score",                    // Cloudflare威胁评分
		"cf-mitigated",                       // Cloudflare缓解状态
		"cf-challenge-bypass",                // Cloudflare挑战绕过
	}

	return slices.Contains(skipHeaders, key)
}

// addProxyHeaders 添加代理相关的头
func (p *ProxyService) addProxyHeaders(httpReq *http.Request, req *ProxyRequest) {
	// 根据Augment API文档，需要保持特定的头部格式

	// 设置会话ID - 这是Augment API的关键头部
	if req.SessionID != "" {
		httpReq.Header.Set("x-request-session-id", req.SessionID)
	}

	// 保持原有的x-request-id如果存在
	if requestID := httpReq.Header.Get("x-request-id"); requestID == "" {
		// 如果没有request-id，生成一个新的
		httpReq.Header.Set("x-request-id", generateRequestID())
	}

	// 保持x-api-version头部
	if apiVersion := httpReq.Header.Get("x-api-version"); apiVersion == "" {
		httpReq.Header.Set("x-api-version", "2")
	}

	// 设置Authorization头部为选中的TOKEN（重要：这是租户的实际TOKEN，不是用户令牌）
	if req.Token != nil && req.Token.Token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+req.Token.Token)
	}

	// 设置目标主机头部 - 重要：需要设置为租户的实际域名
	if targetURL, err := url.Parse(req.TenantAddress); err == nil {
		httpReq.Header.Set("host", targetURL.Host)
		httpReq.Host = targetURL.Host
	}

	// 设置User-Agent - 智能处理逻辑
	p.setUserAgentHeader(httpReq, req)

	// 添加客户端IP
	if req.ClientIP != "" {
		httpReq.Header.Set("X-Forwarded-For", req.ClientIP)
		httpReq.Header.Set("X-Real-IP", req.ClientIP)
	}

	// 添加协议信息
	if httpReq.Header.Get("X-Forwarded-Proto") == "" {
		if httpReq.TLS != nil {
			httpReq.Header.Set("X-Forwarded-Proto", "https")
		} else {
			httpReq.Header.Set("X-Forwarded-Proto", "http")
		}
	}
}

// setUserAgentHeader 设置User-Agent请求头的智能处理逻辑
func (p *ProxyService) setUserAgentHeader(httpReq *http.Request, req *ProxyRequest) {
	// 首先检查是否启用自定义User-Agent功能
	if !p.config.Proxy.EnableCustomUserAgent {
		// 功能未启用，直接使用客户端原始User-Agent
		if req.UserAgent != "" {
			httpReq.Header.Set("User-Agent", req.UserAgent)
		} else {
			// 如果客户端没有User-Agent，使用配置文件中的SUBSCRIPTION_USER_AGENT值
			httpReq.Header.Set("User-Agent", p.config.Subscription.UserAgent)
		}
		return
	}

	// 功能已启用，执行智能User-Agent处理逻辑
	originalUserAgent := req.UserAgent

	// 检查原始User-Agent是否包含"intellij"字符串（不区分大小写）
	if originalUserAgent != "" && strings.Contains(strings.ToLower(originalUserAgent), "intellij") {
		// 包含"intellij"，保持使用客户端原始User-Agent
		httpReq.Header.Set("User-Agent", originalUserAgent)
	} else {
		// 不包含"intellij"，使用环境变量配置的自定义User-Agent
		customUserAgent := p.config.Subscription.UserAgent
		if customUserAgent != "" {
			httpReq.Header.Set("User-Agent", customUserAgent)
		} else {
			// 如果环境变量未配置，使用默认值
			defaultUserAgent := "Augment.vscode-augment/0.708.0 (darwin; arm64; 24.4.0) vscode/1.103.2"
			httpReq.Header.Set("User-Agent", defaultUserAgent)
		}
	}
}

// GetClientIP 获取客户端IP
func GetClientIP(r *http.Request) string {
	// 检查X-Forwarded-For头
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// 取第一个IP
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}

	// 检查X-Real-IP头
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// 检查X-Forwarded头
	if xf := r.Header.Get("X-Forwarded"); xf != "" {
		if idx := strings.Index(xf, "for="); idx != -1 {
			ip := xf[idx+4:]
			if idx := strings.Index(ip, ";"); idx != -1 {
				ip = ip[:idx]
			}
			return strings.TrimSpace(ip)
		}
	}

	// 使用RemoteAddr
	if idx := strings.LastIndex(r.RemoteAddr, ":"); idx != -1 {
		return r.RemoteAddr[:idx]
	}

	return r.RemoteAddr
}

// ValidateRequest 验证请求
func (p *ProxyService) ValidateRequest(req *ProxyRequest) error {
	if req.Token == nil || req.Token.Token == "" {
		return fmt.Errorf("token is required")
	}

	if req.TenantAddress == "" {
		return fmt.Errorf("tenant address is required")
	}

	if req.Method == "" {
		return fmt.Errorf("method is required")
	}

	return nil
}

// LogRequest 记录请求信息 - 用于系统TOKEN代理
func (p *ProxyService) LogRequest(token *database.Token, req *ProxyRequest, resp *ProxyResponse) *database.RequestLog {
	log := &database.RequestLog{
		TokenID:       &token.ID, // 系统TOKEN的ID
		UserTokenID:   nil,       // 用户令牌ID为空
		RequestID:     generateRequestID(),
		Method:        req.Method,
		Path:          req.Path,
		UserAgent:     req.UserAgent,
		ClientIP:      req.ClientIP,
		TenantAddress: req.TenantAddress,
		RequestSize:   int64(len(req.Body)),
		ResponseSize:  resp.Size,
		Latency:       resp.Latency.Microseconds(),
	}

	if resp.StatusCode > 0 {
		log.StatusCode = resp.StatusCode
	}

	if resp.ErrorMessage != "" {
		log.ErrorMessage = resp.ErrorMessage
	}

	return log
}

// LogUserTokenRequest 记录用户令牌请求信息
func (p *ProxyService) LogUserTokenRequest(userTokenID string, systemTokenID string, req *ProxyRequest, resp *ProxyResponse, externalChannelID *uint) *database.RequestLog {
	log := &database.RequestLog{
		TokenID:           &systemTokenID,    // 系统TOKEN的ID
		UserTokenID:       &userTokenID,      // 用户令牌的ID
		ExternalChannelID: externalChannelID, // 外部渠道ID，可为NULL
		RequestID:         generateRequestID(),
		Method:            req.Method,
		Path:              req.Path,
		UserAgent:         req.UserAgent,
		ClientIP:          req.ClientIP,
		TenantAddress:     req.TenantAddress,
		RequestSize:       int64(len(req.Body)),
		ResponseSize:      resp.Size,
		Latency:           resp.Latency.Microseconds(),
	}

	if resp.StatusCode > 0 {
		log.StatusCode = resp.StatusCode
	}

	if resp.ErrorMessage != "" {
		log.ErrorMessage = resp.ErrorMessage
	}

	return log
}

// generateRequestID 生成请求ID
func generateRequestID() string {
	// 生成UUID格式的请求ID
	return strings.ReplaceAll(fmt.Sprintf("%d-%s", time.Now().UnixNano(), randomHex(8)), "-", "")
}

// randomHex 生成随机十六进制字符串
func randomHex(length int) string {
	bytes := make([]byte, length/2)
	if _, err := rand.Read(bytes); err != nil {
		// 如果随机数生成失败，使用时间戳
		return fmt.Sprintf("%x", time.Now().UnixNano())[:length]
	}
	return hex.EncodeToString(bytes)
}

// logBadGatewayStreamResponse 记录502错误的详细响应信息（流式请求）
func (p *ProxyService) logBadGatewayStreamResponse(req *ProxyRequest, resp *http.Response, latency time.Duration) {
	logger.Infof("=== 502 网关错误详情 (流式) ===")
	logger.Infof("请求地址: %s%s", req.TenantAddress, req.Path)
	logger.Infof("请求方法: %s", req.Method)
	logger.Infof("TOKEN: %s...", req.Token.Token[:min(8, len(req.Token.Token))])
	logger.Infof("远程状态: %d %s", resp.StatusCode, resp.Status)
	logger.Infof("响应耗时: %v", latency)

	// 打印响应头
	logger.Infof("响应头:")
	for key, values := range resp.Header {
		for _, value := range values {
			logger.Infof("  %s: %s", key, value)
		}
	}

	// 对于流式请求，尝试读取前面的一些响应内容
	logger.Infof("读取流式响应前2000字节...")
	buffer := make([]byte, 2000)
	n, err := resp.Body.Read(buffer)
	if err != nil && err != io.EOF {
		logger.Errorf("读取响应体错误: %v", err)
	} else if n > 0 {
		logger.Infof("响应体 (前%d字节):\n%s", n, string(buffer[:n]))
	} else {
		logger.Infof("无响应体内容")
	}
	logger.Infof("=== 502错误详情结束 (流式) ===")
}
