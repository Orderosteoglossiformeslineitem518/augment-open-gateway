package service

import (
	"augment-gateway/internal/config"
	"augment-gateway/internal/database"
	"augment-gateway/internal/repository"

	"gorm.io/gorm"
)

// Services 服务集合
type Services struct {
	Auth                  *AuthService
	UserAuth              *UserAuthService // 用户认证服务（注册/登录）
	Token                 *TokenService
	Cache                 *CacheService
	Session               *SessionService
	Stats                 *StatsService
	LoadBalancer          *LoadBalancerService
	ProxyInfo             ProxyInfoService
	Turnstile             TurnstileService
	BanRecord             *BanRecordService
	RequestRecord         *RequestRecordService
	SubscriptionValidator *SubscriptionValidator
	Notification          *NotificationService
	ConversationID        *ConversationIDService     // conversation_id 管理服务
	InvitationCode        *InvitationCodeService     // 邀请码服务
	TokenAllocation       *TokenAllocationService    // TOKEN分配服务
	UserUsageStats        *UserUsageStatsService     // 用户使用统计服务
	ExternalChannel       *ExternalChannelService    // 外部渠道服务
	Plugin                *PluginService             // 插件服务
	SystemAnnouncement    *SystemAnnouncementService // 系统公告服务
	SystemConfig          *SystemConfigService       // 系统配置服务
	SharedToken           *SharedTokenService        // 共享TOKEN分配服务
	ScheduleTask          *ScheduleTaskService       // 定时任务服务
	Monitor               *MonitorService            // 渠道监测服务
	Telegram              *TelegramService           // Telegram推送服务
	RemoteModel           *RemoteModelService        // 远程模型服务
	Redis                 *database.RedisClient      // 添加Redis客户端访问
}

// NewServices 创建服务集合
func NewServices(db *gorm.DB, redis *database.RedisClient, cfg *config.Config) *Services {
	// 创建缓存服务
	cacheService := NewCacheService(redis)

	// 创建认证服务（管理员登录）
	authService := NewAuthService(db, cfg.Security.JWTSecret)

	// 创建用户认证服务（用户注册/登录）
	userAuthService := NewUserAuthService(db, cfg.Security.JWTSecret, &cfg.UserAuth)

	// 创建Token服务
	tokenService := NewTokenService(db, cacheService, cfg)

	// 创建会话服务
	sessionService := NewSessionService(db, cacheService)

	// 创建统计服务
	statsService := NewStatsService(db, cacheService)

	// 创建负载均衡服务
	loadBalancerService := NewLoadBalancerService(db, cacheService, tokenService)

	// 创建代理信息服务
	proxyInfoRepo := repository.NewProxyInfoRepository(db)
	proxyInfoService := NewProxyInfoService(proxyInfoRepo, cacheService, userAuthService)

	// 创建Turnstile验证服务
	turnstileService := NewTurnstileService(&cfg.Turnstile)

	// 创建封号记录服务
	banRecordService := NewBanRecordService(db)

	// 创建订阅验证服务
	subscriptionValidator := NewSubscriptionValidator(db, cacheService, &cfg.Subscription)

	// 创建请求记录服务
	requestRecordService := NewRequestRecordService(db)

	// 创建公告服务
	notificationService := NewNotificationService(db, cacheService)

	// 创建 conversation_id 管理服务
	conversationIDService := NewConversationIDService(cacheService)

	// 创建邀请码服务
	invitationCodeService := NewInvitationCodeService(db)

	// 创建TOKEN分配服务
	tokenAllocationService := NewTokenAllocationService(db)
	tokenAllocationService.SetCacheService(cacheService)

	// 创建用户使用统计服务
	userUsageStatsService := NewUserUsageStatsService(db)
	userUsageStatsService.SetCacheService(cacheService)

	// 创建外部渠道服务
	externalChannelService := NewExternalChannelService(db)
	externalChannelService.SetCacheService(cacheService)

	// 创建插件服务
	pluginService := NewPluginService(db)

	// 创建系统公告服务
	systemAnnouncementService := NewSystemAnnouncementService(db, cacheService)

	// 创建系统配置服务
	systemConfigService := NewSystemConfigService(db)

	// 创建共享TOKEN服务
	sharedTokenService := NewSharedTokenService(db)

	// 创建定时任务服务
	scheduleTaskService := NewScheduleTaskService(db, cfg)

	// 创建渠道监测服务
	monitorService := NewMonitorService(db)
	monitorService.SetCacheService(cacheService)

	// 创建Telegram推送服务
	telegramService := NewTelegramService(cfg.Telegram.BotToken, cfg.Telegram.ChatID, cfg.Telegram.Enabled)

	// 创建远程模型服务
	remoteModelService := NewRemoteModelService(db, cfg)

	// 设置邀请码服务到用户认证服务（避免循环依赖）
	userAuthService.SetInvitationCodeService(invitationCodeService)

	// 设置共享TOKEN服务到用户认证服务
	userAuthService.SetSharedTokenService(sharedTokenService)

	// 设置缓存服务到用户认证服务（用于用户设置缓存）
	userAuthService.SetCacheService(cacheService)

	// 设置安全配置到用户认证服务（用于JWT过期时间配置）
	userAuthService.SetSecurityConfig(&cfg.Security)

	// 设置远程模型服务到外部渠道服务（用于动态获取内部模型列表）
	externalChannelService.SetRemoteModelService(remoteModelService)

	return &Services{
		Auth:                  authService,
		UserAuth:              userAuthService,
		Token:                 tokenService,
		Cache:                 cacheService,
		Session:               sessionService,
		Stats:                 statsService,
		LoadBalancer:          loadBalancerService,
		ProxyInfo:             proxyInfoService,
		Turnstile:             turnstileService,
		BanRecord:             banRecordService,
		RequestRecord:         requestRecordService,
		SubscriptionValidator: subscriptionValidator,
		Notification:          notificationService,
		ConversationID:        conversationIDService,     // 设置 conversation_id 管理服务
		InvitationCode:        invitationCodeService,     // 设置邀请码服务
		TokenAllocation:       tokenAllocationService,    // 设置TOKEN分配服务
		UserUsageStats:        userUsageStatsService,     // 设置用户使用统计服务
		ExternalChannel:       externalChannelService,    // 设置外部渠道服务
		Plugin:                pluginService,             // 设置插件服务
		SystemAnnouncement:    systemAnnouncementService, // 设置系统公告服务
		SystemConfig:          systemConfigService,       // 设置系统配置服务
		SharedToken:           sharedTokenService,        // 设置共享TOKEN服务
		ScheduleTask:          scheduleTaskService,       // 设置定时任务服务
		Monitor:               monitorService,            // 设置渠道监测服务
		Telegram:              telegramService,           // 设置Telegram推送服务
		RemoteModel:           remoteModelService,        // 设置远程模型服务
		Redis:                 redis,                     // 设置Redis客户端访问
	}
}
