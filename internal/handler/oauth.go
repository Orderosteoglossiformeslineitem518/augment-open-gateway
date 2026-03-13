package handler

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"augment-gateway/internal/logger"
	"augment-gateway/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

// OAuthHandler OAuth 2.0 处理器
type OAuthHandler struct {
	userAuthService *service.UserAuthService
	redisClient     *redis.Client
	frontendURL     string
}

// NewOAuthHandler 创建 OAuth 处理器
func NewOAuthHandler(userAuthService *service.UserAuthService, redisClient *redis.Client, frontendURL string) *OAuthHandler {
	return &OAuthHandler{
		userAuthService: userAuthService,
		redisClient:     redisClient,
		frontendURL:     frontendURL,
	}
}

// TokenRequest 令牌请求结构体
type TokenRequest struct {
	GrantType    string `json:"grant_type" binding:"required"`
	ClientID     string `json:"client_id" binding:"required"`
	CodeVerifier string `json:"code_verifier" binding:"required"`
	RedirectURI  string `json:"redirect_uri" binding:"required"`
	Code         string `json:"code" binding:"required"`
}

// TokenResponse 令牌响应结构体
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

// generateAuthorizationCode 生成8位随机数字授权码
func (h *OAuthHandler) generateAuthorizationCode() (string, error) {
	bytes := make([]byte, 4) // 32位随机数
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	// 将字节转换为8位数字
	var num uint32
	for i, b := range bytes {
		num |= uint32(b) << (8 * i)
	}

	// 确保是8位数字（10000000-99999999）
	code := 10000000 + (num % 90000000)
	return fmt.Sprintf("%08d", code), nil
}

// storeAuthorizationCode 存储授权码到Redis，映射到用户ID
func (h *OAuthHandler) storeAuthorizationCode(authCode, userID string) error {
	ctx := context.Background()
	key := fmt.Sprintf("oauth:auth_code:%s", authCode)
	// 授权码有效期10分钟
	return h.redisClient.Set(ctx, key, userID, 10*time.Minute).Err()
}

// getUserIDFromAuthCode 根据授权码获取用户ID
func (h *OAuthHandler) getUserIDFromAuthCode(authCode string) (string, error) {
	ctx := context.Background()
	key := fmt.Sprintf("oauth:auth_code:%s", authCode)

	userID, err := h.redisClient.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", fmt.Errorf("授权码无效或已过期")
		}
		return "", fmt.Errorf("redis查询失败: %v", err)
	}

	// 使用后立即删除授权码（一次性使用）
	h.redisClient.Del(ctx, key)

	return userID, nil
}

// Authorize 处理授权请求 - 返回授权页面
func (h *OAuthHandler) Authorize(c *gin.Context) {
	// 获取查询参数
	responseType := c.Query("response_type")
	clientID := c.Query("client_id")
	redirectURI := c.Query("redirect_uri")
	state := c.Query("state")
	scope := c.Query("scope")
	codeChallenge := c.Query("code_challenge")
	codeChallengeMethod := c.Query("code_challenge_method")
	prompt := c.Query("prompt")

	logger.Infof("[OAuth] 收到授权请求: response_type=%s, client_id=%s, redirect_uri=%s\n",
		responseType, clientID, redirectURI)

	// 验证必需参数
	if responseType != "code" {
		h.respondError(c, http.StatusBadRequest, "不支持的响应类型")
		return
	}

	if codeChallengeMethod != "S256" {
		h.respondError(c, http.StatusBadRequest, "不支持的代码挑战方法")
		return
	}

	// 返回授权页面HTML
	authPageHTML := h.generateAuthorizePage(responseType, clientID, redirectURI, state, scope, codeChallenge, codeChallengeMethod, prompt)
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, authPageHTML)
}

// ProcessAuthorize 处理实际的授权逻辑
func (h *OAuthHandler) ProcessAuthorize(c *gin.Context) {
	// 从POST请求体获取参数
	var req struct {
		ResponseType        string `json:"response_type" binding:"required"`
		ClientID            string `json:"client_id" binding:"required"`
		RedirectURI         string `json:"redirect_uri" binding:"required"`
		State               string `json:"state"`
		Scope               string `json:"scope"`
		CodeChallenge       string `json:"code_challenge" binding:"required"`
		CodeChallengeMethod string `json:"code_challenge_method" binding:"required"`
		Prompt              string `json:"prompt"`
		UserID              string `json:"user_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	logger.Infof("[OAuth] 处理授权请求: user_id=%s, client_id=%s\n", req.UserID, req.ClientID)

	// 验证用户并获取有效令牌
	userToken, err := h.getUserValidToken(req.UserID)
	if err != nil {
		h.respondError(c, http.StatusUnauthorized, fmt.Sprintf("用户验证失败: %v", err))
		return
	}

	if userToken == nil {
		h.respondError(c, http.StatusUnauthorized, "用户没有有效的令牌")
		return
	}

	logger.Infof("[OAuth] 用户令牌验证成功: %s...\n", userToken.Token[:min(8, len(userToken.Token))])

	// 生成安全的授权码
	authCode, err := h.generateAuthorizationCode()
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, "生成授权码失败")
		return
	}

	// 将授权码存储到Redis，映射到用户ID
	if err := h.storeAuthorizationCode(authCode, req.UserID); err != nil {
		h.respondError(c, http.StatusInternalServerError, "存储授权码失败")
		return
	}

	// 构建重定向URL - 使用安全的随机授权码
	redirectURL := fmt.Sprintf("%s?code=%s&tenant_url=%s&state=%s",
		req.RedirectURI,
		url.QueryEscape(authCode),
		url.QueryEscape(h.frontendURL+"/proxy/"),
		url.QueryEscape(req.State))

	logger.Infof("[OAuth] 授权成功，重定向到: %s\n", redirectURL)

	// 返回重定向URL给前端
	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"redirect_url": redirectURL,
	})
}

// Token 处理令牌交换请求
func (h *OAuthHandler) Token(c *gin.Context) {
	var req TokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	logger.Infof("[OAuth] 收到令牌交换请求: grant_type=%s, client_id=%s, code=%s...\n",
		req.GrantType, req.ClientID, req.Code[:min(8, len(req.Code))])

	// 验证必需参数
	if req.GrantType != "authorization_code" {
		h.respondError(c, http.StatusBadRequest, "不支持的授权类型")
		return
	}

	if req.Code == "" {
		h.respondError(c, http.StatusBadRequest, "缺少授权码")
		return
	}

	if req.CodeVerifier == "" {
		h.respondError(c, http.StatusBadRequest, "缺少代码验证器")
		return
	}

	// 验证授权码并获取用户ID
	logger.Infof("[OAuth] 处理令牌交换请求，授权码: %s\n", req.Code)

	// 从Redis中获取授权码对应的用户ID
	userIDStr, err := h.getUserIDFromAuthCode(req.Code)
	if err != nil {
		h.respondError(c, http.StatusUnauthorized, fmt.Sprintf("授权码无效: %v", err))
		return
	}

	logger.Infof("[OAuth] 授权码验证成功，用户ID: %s\n", userIDStr)

	// 根据用户ID获取用户令牌
	userToken, err := h.getUserValidToken(userIDStr)
	if err != nil {
		h.respondError(c, http.StatusUnauthorized, fmt.Sprintf("获取用户令牌失败: %v", err))
		return
	}

	if userToken == nil {
		h.respondError(c, http.StatusUnauthorized, "授权码对应的用户没有有效令牌")
		return
	}

	// 返回访问令牌响应
	response := TokenResponse{
		AccessToken:  userToken.Token, // 返回用户的实际令牌
		TokenType:    "Bearer",
		ExpiresIn:    0, // 0表示不过期
		RefreshToken: "",
		Scope:        "",
	}

	logger.Infof("[OAuth] 返回访问令牌: %s...\n", response.AccessToken[:min(8, len(response.AccessToken))])

	c.JSON(http.StatusOK, response)
}

// getUserValidToken 获取用户的有效令牌
func (h *OAuthHandler) getUserValidToken(userIDStr string) (*service.UserApiTokenInfo, error) {
	// 转换用户ID
	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("用户ID格式错误: %v", err)
	}

	// 获取用户信息
	user, err := h.userAuthService.GetUserByID(context.Background(), uint(userID))
	if err != nil {
		return nil, fmt.Errorf("获取用户信息失败: %v", err)
	}

	// 检查用户状态
	if !user.IsActive() {
		return nil, fmt.Errorf("用户账号已被禁用")
	}

	// 检查用户令牌状态
	if !user.IsTokenActive() {
		return nil, fmt.Errorf("用户API令牌无效或已被禁用")
	}

	// 检查是否可以发起请求
	if !user.CanMakeRequest() {
		return nil, fmt.Errorf("用户API额度已用完或无额度")
	}

	// 返回用户的API令牌信息
	return &service.UserApiTokenInfo{
		ID:                 user.ID,
		UserID:             user.ID,
		Token:              user.ApiToken,
		Status:             user.TokenStatus,
		MaxRequests:        user.MaxRequests,
		UsedRequests:       user.UsedRequests,
		RateLimitPerMinute: user.RateLimitPerMinute,
		CanUseSharedTokens: user.CanUseSharedTokens,
	}, nil
}

// generateAuthorizePage 生成授权页面HTML
func (h *OAuthHandler) generateAuthorizePage(responseType, clientID, redirectURI, state, scope, codeChallenge, codeChallengeMethod, prompt string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>ACG认证</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
            background: #f8fafc;
            min-height: 100vh;
            width: 100vw;
            display: flex;
            align-items: center;
            justify-content: center;
            margin: 0;
            padding: 20px;
        }

        .container {
            background: #fff;
            border-radius: 12px;
            padding: 60px 40px;
            border: 2px solid #e2e8f0;
            max-width: 480px;
            width: 90%%;
            text-align: center;
            animation: containerFadeIn 0.5s ease-out;
        }

        @keyframes containerFadeIn {
            0%% {
                opacity: 0;
                transform: translateY(20px);
            }
            100%% {
                opacity: 1;
                transform: translateY(0);
            }
        }

        .logo {
            font-size: 28px;
            font-weight: 700;
            color: #1e293b;
            margin-bottom: 12px;
            display: flex;
            align-items: center;
            justify-content: center;
            gap: 10px;
        }

        .logo-icon {
            width: 32px;
            height: 32px;
            flex-shrink: 0;
        }

        .subtitle {
            color: #64748b;
            margin-bottom: 40px;
            font-size: 16px;
            font-weight: 400;
        }

        .status {
            margin: 30px 0;
            padding: 16px;
            border-radius: 10px;
            font-size: 15px;
            font-weight: 500;
            transition: all 0.3s ease;
        }

        .loading {
            background: rgba(102, 126, 234, 0.1);
            color: #667eea;
            border: 1px solid rgba(102, 126, 234, 0.2);
        }

        .error {
            background: rgba(245, 108, 108, 0.1);
            color: #f56c6c;
            border: 1px solid rgba(245, 108, 108, 0.2);
        }

        .success {
            background: rgba(103, 194, 58, 0.1);
            color: #67c23a;
            border: 1px solid rgba(103, 194, 58, 0.2);
        }

        .spinner {
            display: inline-block;
            width: 16px;
            height: 16px;
            border: 2px solid rgba(102, 126, 234, 0.2);
            border-top: 2px solid #667eea;
            border-radius: 50%%;
            animation: spin 1s linear infinite;
            margin-right: 8px;
            vertical-align: middle;
        }

        @keyframes spin {
            0%% { transform: rotate(0deg); }
            100%% { transform: rotate(360deg); }
        }

        .login-hint {
            margin-top: 30px;
            padding: 16px;
            background: rgba(230, 162, 60, 0.1);
            border: 1px solid rgba(230, 162, 60, 0.3);
            border-radius: 10px;
            color: #e6a23c;
            font-size: 14px;
        }

        .login-link {
            color: #667eea;
            text-decoration: none;
            font-weight: 500;
        }

        .login-link:hover {
            text-decoration: underline;
        }

        /* 响应式设计 */
        @media (max-width: 768px) {
            .container {
                padding: 40px 30px;
                width: 95%%;
            }

            .logo {
                font-size: 24px;
            }

            .subtitle {
                font-size: 14px;
            }
        }

        @media (max-width: 480px) {
            .container {
                padding: 30px 20px;
                width: 98%%;
            }

            .logo {
                font-size: 22px;
            }

            .subtitle {
                font-size: 13px;
                margin-bottom: 30px;
            }

            .status {
                padding: 14px;
                font-size: 14px;
                margin: 20px 0;
            }

            .login-hint {
                padding: 14px;
                font-size: 13px;
                margin-top: 20px;
            }
        }

        /* 简单的加载动画 */
        .loading-dots {
            animation: loadingDots 1.5s infinite;
        }

        @keyframes loadingDots {
            0%%, 33%% { opacity: 0.3; }
            66%% { opacity: 1; }
            100%% { opacity: 0.3; }
        }

        .loading-dots:nth-child(2) { animation-delay: 0.2s; }
        .loading-dots:nth-child(3) { animation-delay: 0.4s; }
    </style>
</head>
<body>
    <div class="container">
        <div class="logo">
            <svg class="logo-icon" viewBox="0 0 1024 1024" xmlns="http://www.w3.org/2000/svg"><path d="M106.496 180.224l0 667.648c0 40.96 32.768 73.728 73.728 73.728l667.648 0c40.96 0 73.728-32.768 73.728-73.728L921.6 180.224c0-40.96-32.768-73.728-73.728-73.728L180.224 106.496C139.264 106.496 106.496 139.264 106.496 180.224zM143.36 180.224c0-20.48 16.384-36.864 36.864-36.864l667.648 0c20.48 0 36.864 16.384 36.864 36.864l0 667.648c0 20.48-16.384 36.864-36.864 36.864L180.224 884.736c-20.48 0-36.864-16.384-36.864-36.864L143.36 180.224z" fill="#1e293b"></path><path d="M307.2 696.32c4.096 0 8.192 0 12.288-4.096l0 0 147.456-110.592 0 0c4.096-4.096 8.192-8.192 8.192-16.384 0-4.096-4.096-12.288-8.192-16.384l0 0-147.456-110.592 0 0c-4.096-4.096-8.192-4.096-12.288-4.096-8.192 0-20.48 8.192-20.48 20.48 0 4.096 4.096 12.288 8.192 16.384l0 0 126.976 94.208-126.976 94.208 0 0c-4.096 4.096-8.192 8.192-8.192 16.384C290.816 688.128 299.008 696.32 307.2 696.32z" fill="#1e293b"></path><path d="M491.52 733.184l110.592 0c8.192 0 20.48-8.192 20.48-20.48 0-8.192-8.192-20.48-20.48-20.48L491.52 692.224c-8.192 0-20.48 8.192-20.48 20.48C475.136 724.992 483.328 733.184 491.52 733.184z" fill="#1e293b"></path></svg>
            <span>Augment Gateway</span>
        </div>
        <div class="subtitle">OAuth 2.0 授权验证</div>

        <div id="status" class="status loading">
            <span class="spinner"></span>
            正在验证用户身份<span class="loading-dots">.</span><span class="loading-dots">.</span><span class="loading-dots">.</span>
        </div>

        <div id="loginHint" class="login-hint" style="display: none;">
            <strong>需要登录验证</strong><br>
            请先 <a href="%s" class="login-link" target="_blank">登录 Augment Gateway</a> 后再进行授权操作
        </div>
    </div>

    <script>
        // OAuth 参数
        const oauthParams = {
            response_type: '%s',
            client_id: '%s',
            redirect_uri: '%s',
            state: '%s',
            scope: '%s',
            code_challenge: '%s',
            code_challenge_method: '%s',
            prompt: '%s'
        };

        // 状态元素
        const statusEl = document.getElementById('status');
        const loginHintEl = document.getElementById('loginHint');

        // 更新状态显示
        function updateStatus(type, message) {
            statusEl.className = 'status ' + type;
            statusEl.innerHTML = message;
        }

        // 显示登录提示
        function showLoginHint() {
            loginHintEl.style.display = 'block';
        }

        // 处理授权
        async function processAuthorization() {
            try {
                // 从 localStorage 获取用户信息
                const storedUser = localStorage.getItem('user_info');
                if (!storedUser) {
                    updateStatus('error', '❌ 未找到用户登录信息');
                    showLoginHint();
                    return;
                }

                const userInfo = JSON.parse(storedUser);
                if (!userInfo.id) {
                    updateStatus('error', '❌ 用户信息格式错误');
                    showLoginHint();
                    return;
                }

                // 显示授权处理状态
                updateStatus('loading', '<span class="spinner"></span>正在处理OAuth授权<span class="loading-dots">.</span><span class="loading-dots">.</span><span class="loading-dots">.</span>');

                // 调用后端授权API
                const response = await fetch('/process-authorize', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json'
                    },
                    body: JSON.stringify({
                        ...oauthParams,
                        user_id: userInfo.id.toString()
                    })
                });

                const result = await response.json();

                if (result.success && result.redirect_url) {
                    updateStatus('success', '✅ 授权成功，正在跳转<span class="loading-dots">.</span><span class="loading-dots">.</span><span class="loading-dots">.</span>');

                    // 延迟跳转，让用户看到成功消息
                    setTimeout(() => {
                        window.location.href = result.redirect_url;
                    }, 1000);
                } else {
                    const errorMsg = result.error_description || result.msg || '未知错误';
                    updateStatus('error', '❌ 授权失败: ' + errorMsg);
                    showLoginHint();
                }
            } catch (error) {
                console.error('授权处理失败:', error);
                updateStatus('error', '❌ 网络连接失败，请重试');
                showLoginHint();
            }
        }

        // 页面加载完成后开始处理
        document.addEventListener('DOMContentLoaded', function() {
            setTimeout(processAuthorization, 500);
        });

        // 添加页面可见性检测，防止在后台标签页中执行
        document.addEventListener('visibilitychange', function() {
            if (document.hidden) {
                console.log('页面已隐藏，暂停处理');
            } else {
                console.log('页面已显示，继续处理');
            }
        });
    </script>
</body>
</html>`, h.frontendURL, responseType, clientID, redirectURI, state, scope, codeChallenge, codeChallengeMethod, prompt)
}

// respondError 返回错误响应
func (h *OAuthHandler) respondError(c *gin.Context, statusCode int, message string) {
	logger.Infof("[OAuth] 错误响应: %d - %s\n", statusCode, message)
	c.JSON(statusCode, gin.H{
		"error":             "invalid_request",
		"error_description": message,
	})
}
