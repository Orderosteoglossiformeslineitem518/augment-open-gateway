package handler

import (
	"net/http"
	"time"

	"augment-gateway/internal/config"
	"augment-gateway/internal/proxy"
	"augment-gateway/internal/service"

	"github.com/gin-gonic/gin"
)

// Response 统一响应结构体
type Response struct {
	Code int         `json:"code"` // 响应状态码，200表示成功，非200表示错误
	Msg  string      `json:"msg"`  // 提示信息，用于描述响应结果或错误原因
	Data interface{} `json:"data"` // 响应数据，成功时为具体数据，失败时可为空或包含错误详情
}

// ResponseSuccess 返回成功响应
func ResponseSuccess(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code: 200,
		Msg:  "操作成功",
		Data: data,
	})
}

// ResponseSuccessWithMsg 返回带自定义消息的成功响应
func ResponseSuccessWithMsg(c *gin.Context, msg string, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code: 200,
		Msg:  msg,
		Data: data,
	})
}

// ResponseError 返回错误响应
func ResponseError(c *gin.Context, code int, msg string) {
	c.JSON(http.StatusOK, Response{
		Code: code,
		Msg:  msg,
		Data: nil,
	})
}

// ResponseErrorWithData 返回带数据的错误响应
func ResponseErrorWithData(c *gin.Context, code int, msg string, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code: code,
		Msg:  msg,
		Data: data,
	})
}

// Handlers 处理器集合
type Handlers struct {
	Auth               *AuthHandler
	UserAuth           *UserAuthHandler // 用户认证处理器（注册/登录）
	Token              *TokenHandler
	Stats              *StatsHandler
	Proxy              *ProxyHandler
	ProxyInfo          *ProxyInfoHandler
	RequestRecord      *RequestRecordHandler
	Notification       *NotificationHandler
	OAuth              *OAuthHandler
	InvitationCode     *InvitationCodeHandler     // 邀请码处理器
	TokenAllocation    *TokenAllocationHandler    // TOKEN分配处理器
	ExternalChannel    *ExternalChannelHandler    // 外部渠道处理器
	Plugin             *PluginHandler             // 插件处理器
	SystemAnnouncement *SystemAnnouncementHandler // 系统公告处理器
	SystemConfig       *SystemConfigHandler       // 系统配置处理器
	System             *SystemHandler             // 系统信息处理器
	Monitor            *MonitorHandler            // 监测处理器
	RemoteModel        *RemoteModelHandler        // 远程模型处理器
}

// NewHandlers 创建处理器集合
func NewHandlers(services *service.Services, cfg *config.Config) *Handlers {
	// 创建代理服务
	proxyService := proxy.NewProxyService(cfg)

	// 创建不跟随重定向的HTTP客户端
	noRedirectHTTPClient := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // 不跟随重定向
		},
		Timeout: 30 * time.Second,
	}

	// 创建Auth Session客户端
	authSessionClient := service.NewAuthSessionClient(noRedirectHTTPClient)

	// 设置AuthSessionClient到TokenService（用于定时刷新AuthSession）
	services.Token.SetAuthSessionClient(authSessionClient)

	return &Handlers{
		Auth:               NewAuthHandler(services.Auth),
		UserAuth:           NewUserAuthHandler(services.UserAuth, services.Turnstile),
		Token:              NewTokenHandler(services.Token, services.ProxyInfo, authSessionClient, cfg),
		Stats:              NewStatsHandler(services.Stats),
		Proxy:              NewProxyHandler(proxyService, services.Token, services.UserAuth, services.Cache, services.Stats, services.BanRecord, services.RequestRecord, services.ConversationID, authSessionClient, services.UserUsageStats, services.ExternalChannel, services.SharedToken, services.RemoteModel, cfg),
		ProxyInfo:          NewProxyInfoHandler(services.ProxyInfo, services.UserAuth, services.Turnstile),
		RequestRecord:      NewRequestRecordHandler(services.RequestRecord),
		Notification:       NewNotificationHandler(services.Notification),
		OAuth:              NewOAuthHandler(services.UserAuth, services.Redis.GetClient(), cfg.Frontend.URL),
		InvitationCode:     NewInvitationCodeHandler(services.InvitationCode),
		TokenAllocation:    NewTokenAllocationHandler(services.TokenAllocation, services.UserUsageStats, services.Cache, services.Turnstile, services.Token, authSessionClient, services.ProxyInfo),
		ExternalChannel:    NewExternalChannelHandler(services.ExternalChannel),
		Plugin:             NewPluginHandler(services.Plugin),
		SystemAnnouncement: NewSystemAnnouncementHandler(services.SystemAnnouncement),
		SystemConfig:       NewSystemConfigHandler(services.SystemConfig),
		System:             NewSystemHandler(),
		Monitor:            NewMonitorHandler(services.Monitor),
		RemoteModel:        NewRemoteModelHandler(services.RemoteModel),
	}
}
