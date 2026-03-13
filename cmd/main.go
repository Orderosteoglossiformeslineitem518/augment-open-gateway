package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"augment-gateway/internal/config"
	"augment-gateway/internal/database"
	"augment-gateway/internal/handler"
	"augment-gateway/internal/logger"
	"augment-gateway/internal/middleware"
	"augment-gateway/internal/service"

	"github.com/gin-gonic/gin"
)

func main() {
	// 加载配置
	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("加载配置失败: %v", err)
	}

	// 初始化日志
	logger.Init(&logger.Config{
		Enabled: cfg.Log.Enabled,
		Level:   cfg.Log.Level,
		Format:  cfg.Log.Format,
	})
	defer logger.Sync()

	// 设置Gin模式
	gin.SetMode(cfg.Server.Mode)

	// 初始化数据库
	db, err := database.InitMySQL(&cfg.Database.MySQL)
	if err != nil {
		logger.Fatalf("初始化MySQL失败: %v", err)
	}

	// 初始化Redis
	rdb, err := database.InitRedis(&cfg.Redis)
	if err != nil {
		logger.Fatalf("初始化Redis失败: %v", err)
	}

	// 初始化服务
	services := service.NewServices(db, rdb, cfg)

	// 创建默认管理员用户
	err = services.Auth.CreateDefaultAdmin()
	if err != nil {
		logger.Warnf("创建默认管理员失败: %v", err)
	}

	// 初始化处理器
	handlers := handler.NewHandlers(services, cfg)

	// 启动定时任务
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// AuthSession定时刷新（已关闭：当前session刷新会返回401导致token被封禁）
	// go services.Token.StartAuthSessionRefreshScheduler(ctx)
	// 共享账号积分消耗定时任务（每3天）- 根据配置决定是否启动
	if cfg.Proxy.ScheduleTaskEnabled {
		go services.ScheduleTask.StartScheduleTaskScheduler(ctx)
	} else {
		log.Println("[定时任务] SCHEDULE_TASK_ENABLED=false，共享账号积分消耗定时任务已禁用")
	}

	// 启动渠道监测定时任务
	go services.Monitor.StartMonitorScheduler(ctx)

	// 启动时同步远程模型列表（异步，不阻塞启动）
	go func() {
		newCount, err := services.RemoteModel.SyncFromRemoteAPI()
		if err != nil {
			logger.Warnf("[启动] 远程模型同步失败: %v", err)
		} else {
			logger.Infof("[启动] 远程模型同步完成，新增 %d 个模型", newCount)
		}
	}()

	// 创建路由
	router := setupRouter(cfg, handlers)

	// 创建HTTP服务器
	server := &http.Server{
		Addr:         cfg.Server.GetServerAddr(),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// 启动服务器
	go func() {
		logger.Infof("服务器启动在 %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(http.ErrServerClosed, err) {
			logger.Fatalf("服务器启动失败: %v", err)
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("正在关闭服务器...")

	// 优雅关闭
	cancel() // 取消定时任务

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Fatalf("服务器强制关闭: %v", err)
	}

	logger.Info("服务器已关闭")
}

// setupRouter 设置路由
func setupRouter(cfg *config.Config, handlers *handler.Handlers) *gin.Engine {
	router := gin.New()

	// 添加中间件
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(middleware.CORS(cfg))

	// OAuth 2.0 授权码流程路由（根路径下，不需要认证）
	// 注意：必须在通配符路由之前定义，确保优先匹配
	router.GET("/authorize", handlers.OAuth.Authorize)
	router.POST("/process-authorize", handlers.OAuth.ProcessAuthorize)

	// 用户令牌代理路由 - 统一的代理转发入口
	// 用户携带令牌 -> 验证令牌 -> 负载均衡选择TOKEN -> 转发到对应租户
	router.Any("/proxy/*path", func(c *gin.Context) {
		path := c.Param("path")
		// 如果是OAuth token请求，交给OAuth处理器
		if (path == "/token" || path == "token") && c.Request.Method == "POST" {
			handlers.OAuth.Token(c)
			return
		}
		// 其他路径交给代理处理
		handlers.Proxy.ForwardWithUserToken(c)
	})

	// 公告客户端接口（不需要认证，不通过代理转发）
	router.Any("/notifications/read", handlers.Notification.ReadNotifications)
	router.POST("/notifications/mark-as-read", handlers.Notification.MarkAsRead)

	// API路由组
	api := router.Group(cfg.Frontend.APIPrefix)
	{
		// 认证路由（不需要登录）
		auth := api.Group("/auth")
		{
			auth.POST("/login", handlers.Auth.Login)
			auth.POST("/logout", handlers.Auth.Logout)
		}

		// 开放接口（不需要管理后台鉴权，但需要特殊的Authorization头）
		open := api.Group("/tokens")
		{
			open.POST("/batch-import", handlers.Token.BatchImport)
		}

		// 用户认证路由（不需要登录）
		userAuth := api.Group("/user-auth")
		{
			userAuth.POST("/register", handlers.UserAuth.Register)
			userAuth.POST("/login", handlers.UserAuth.Login)
			userAuth.POST("/logout", handlers.UserAuth.Logout)
			userAuth.POST("/refresh", handlers.UserAuth.Refresh) // 刷新令牌
		}

		// 邀请码验证接口（公开，不需要登录）
		api.GET("/invitation-codes/validate", handlers.InvitationCode.Validate)

		// 系统公告公开接口（不需要登录）
		api.GET("/system-announcements/published", handlers.SystemAnnouncement.GetPublishedAnnouncements)

		// 系统信息公开接口（不需要登录）
		api.GET("/system/version", handlers.System.GetVersion)

		// 用户功能路由（需要用户身份验证）
		user := api.Group("/user")
		user.Use(handlers.UserAuth.UserAuthMiddleware()) // 用户身份验证中间件
		{
			// 用户信息
			user.GET("/me", handlers.UserAuth.Me)
			user.PUT("/profile", handlers.UserAuth.UpdateProfile)
			user.POST("/change-password", handlers.UserAuth.ChangePassword)
			user.POST("/regenerate-token", handlers.UserAuth.RegenerateAPIToken)

			// 用户设置
			user.GET("/settings", handlers.UserAuth.GetUserSettings)
			user.PUT("/settings", handlers.UserAuth.UpdateUserSettings)

			// 代理相关
			user.POST("/proxy/submit", handlers.ProxyInfo.SubmitProxy)
			user.GET("/proxy/submissions", handlers.ProxyInfo.GetUserSubmissions)
			user.GET("/proxy/check-limit", handlers.ProxyInfo.CheckSubmissionLimit)

			// TOKEN分配列表
			user.GET("/token-allocations", handlers.TokenAllocation.GetUserAllocations)

			// 使用统计
			user.GET("/usage-stats", handlers.TokenAllocation.GetUserUsageStats)
			user.GET("/usage-stats/overview", handlers.TokenAllocation.GetUserStatsOverview)

			// TOKEN账号统计
			user.GET("/token-account-stats", handlers.TokenAllocation.GetUserTokenAccountStats)

			// 用户TOKEN账号管理
			user.POST("/tokens/submit", handlers.TokenAllocation.UserSubmitToken)
			user.POST("/tokens/:token_id/disable", handlers.TokenAllocation.UserDisableToken)
			user.DELETE("/tokens/:token_id", handlers.TokenAllocation.UserDeleteToken)
			user.POST("/tokens/:token_id/switch", handlers.TokenAllocation.UserSwitchToken)
			user.GET("/tokens/available-for-switch", handlers.TokenAllocation.GetUserAvailableTokensForSwitch)

			// TOKEN增强功能
			user.POST("/tokens/:token_id/enhance", handlers.TokenAllocation.UserEnhanceToken)
			user.DELETE("/tokens/:token_id/enhance", handlers.TokenAllocation.UserRemoveTokenEnhance)
			user.GET("/tokens/:token_id/enhance", handlers.TokenAllocation.GetTokenEnhanceInfo)

			// 外部渠道管理
			externalChannels := user.Group("/external-channels")
			{
				externalChannels.POST("", handlers.ExternalChannel.Create)
				externalChannels.GET("", handlers.ExternalChannel.GetList)
				externalChannels.GET("/internal-models", handlers.ExternalChannel.GetInternalModels)
				externalChannels.POST("/fetch-models", handlers.ExternalChannel.FetchModels)
				externalChannels.GET("/:id", handlers.ExternalChannel.GetByID)
				externalChannels.PUT("/:id", handlers.ExternalChannel.Update)
				externalChannels.DELETE("/:id", handlers.ExternalChannel.Delete)
				externalChannels.POST("/:id/test", handlers.ExternalChannel.Test)
				externalChannels.GET("/:id/usage-stats", handlers.ExternalChannel.GetUsageStats)
			}

			// 插件下载
			plugins := user.Group("/plugins")
			{
				plugins.GET("", handlers.Plugin.GetList)
				plugins.GET("/:id/download", handlers.Plugin.Download)
			}

			// 系统公告（用户端）
			user.GET("/announcements", handlers.SystemAnnouncement.GetPublishedAnnouncementsWithUnread)
			user.PUT("/announcements/mark-read", handlers.SystemAnnouncement.MarkAnnouncementsAsRead)

			// 渠道监测
			monitor := user.Group("/monitor")
			{
				monitor.GET("/configs", handlers.Monitor.GetList)
				monitor.POST("/configs", handlers.Monitor.Create)
				monitor.GET("/configs/:id", handlers.Monitor.GetDetail)
				monitor.PUT("/configs/:id", handlers.Monitor.Update)
				monitor.DELETE("/configs/:id", handlers.Monitor.Delete)
				monitor.PATCH("/configs/:id/status", handlers.Monitor.ToggleStatus)
				monitor.POST("/configs/:id/trigger", handlers.Monitor.TriggerCheck)
				monitor.GET("/channels/:channel_id/models", handlers.Monitor.GetChannelModels)
			}
		}

		// 需要认证的路由
		protected := api.Group("")
		protected.Use(handlers.Auth.AuthMiddleware())
		{
			// 用户信息
			protected.GET("/auth/me", handlers.Auth.Me)
			protected.PUT("/auth/profile", handlers.Auth.UpdateProfile)

			// Token管理
			tokens := protected.Group("/tokens")
			{
				tokens.GET("", handlers.Token.List)
				tokens.POST("", handlers.Token.Create)
				tokens.GET("/:id", handlers.Token.Get)
				tokens.PUT("/:id", handlers.Token.Update)
				tokens.DELETE("/:id", handlers.Token.Delete)
				// 获取TOKEN使用用户列表
				tokens.GET("/:id/users", handlers.Token.GetTokenUsers)
				// 获取TOKEN封禁原因（通过AuthSession调用远程API）
				tokens.GET("/:id/ban-reason", handlers.Token.GetBanReason)
				tokens.GET("/validate", handlers.Token.Validate)
				tokens.GET("/stats", handlers.Token.Stats)
				// 批量刷新AuthSession
				tokens.POST("/batch-refresh-auth-session", handlers.Token.BatchRefreshAuthSession)
			}

			// 用户管理（管理员）
			users := protected.Group("/users")
			{
				users.GET("", handlers.UserAuth.ListUsers)
				users.PUT("/:id", handlers.UserAuth.AdminUpdateUser)
				users.POST("/:id/ban", handlers.UserAuth.BanUser)
				users.POST("/:id/unban", handlers.UserAuth.UnbanUser)
				users.POST("/:id/toggle-shared", handlers.UserAuth.ToggleSharedPermission)
			}

			// 统计信息
			stats := protected.Group("/stats")
			{
				stats.GET("/overview", handlers.Stats.Overview)
				stats.GET("/trend", handlers.Stats.Trend)
				stats.GET("/tokens/:id", handlers.Stats.TokenStats)
				stats.GET("/usage", handlers.Stats.Usage)
				stats.POST("/cleanup", handlers.Stats.Cleanup)
			}

			// 请求记录管理（需要管理员权限）
			requestRecords := protected.Group("/request-records")
			{
				requestRecords.GET("", handlers.RequestRecord.List)
				requestRecords.GET("/search", handlers.RequestRecord.Search)
			}

			// 代理管理（需要管理员权限）
			proxies := protected.Group("/proxies")
			{
				proxies.GET("", handlers.ProxyInfo.ListProxies)
				proxies.POST("", handlers.ProxyInfo.CreateProxy)
				proxies.PUT("/:id/status", handlers.ProxyInfo.UpdateProxyStatus)
				proxies.POST("/:id/approve", handlers.ProxyInfo.ApproveProxy)
				proxies.POST("/:id/reject", handlers.ProxyInfo.RejectProxy)
				proxies.DELETE("/:id", handlers.ProxyInfo.DeleteProxy)
			}

			// 远程模型管理（需要管理员权限）
			remoteModels := protected.Group("/remote-models")
			{
				remoteModels.GET("", handlers.RemoteModel.GetList)
				remoteModels.POST("/sync", handlers.RemoteModel.SyncModels)
				remoteModels.PUT("/:id/passthrough", handlers.RemoteModel.UpdatePassthroughConfig)
				remoteModels.POST("/:id/set-default", handlers.RemoteModel.SetDefaultModel)
				remoteModels.DELETE("/:id", handlers.RemoteModel.DeleteModel)
			}

			// 公告管理（需要管理员权限）
			notifications := protected.Group("/notifications")
			{
				notifications.GET("", handlers.Notification.ListNotifications)
				notifications.POST("", handlers.Notification.CreateNotification)
				notifications.GET("/:id", handlers.Notification.GetNotification)
				notifications.PUT("/:id", handlers.Notification.UpdateNotification)
				notifications.DELETE("/:id", handlers.Notification.DeleteNotification)
				notifications.POST("/:id/enable", handlers.Notification.EnableNotification)
				notifications.POST("/:id/disable", handlers.Notification.DisableNotification)
			}

			// 邀请码管理（需要管理员权限）
			invitationCodes := protected.Group("/invitation-codes")
			{
				invitationCodes.GET("", handlers.InvitationCode.List)
				invitationCodes.POST("/generate", handlers.InvitationCode.Generate)
				invitationCodes.DELETE("/:id", handlers.InvitationCode.Delete)
			}

			// 系统公告管理（需要管理员权限）
			systemAnnouncements := protected.Group("/system-announcements")
			{
				systemAnnouncements.GET("", handlers.SystemAnnouncement.ListAnnouncements)
				systemAnnouncements.POST("", handlers.SystemAnnouncement.CreateAnnouncement)
				systemAnnouncements.GET("/:id", handlers.SystemAnnouncement.GetAnnouncement)
				systemAnnouncements.PUT("/:id", handlers.SystemAnnouncement.UpdateAnnouncement)
				systemAnnouncements.DELETE("/:id", handlers.SystemAnnouncement.DeleteAnnouncement)
				systemAnnouncements.POST("/:id/publish", handlers.SystemAnnouncement.PublishAnnouncement)
				systemAnnouncements.POST("/:id/cancel", handlers.SystemAnnouncement.CancelAnnouncement)
			}

			// 系统配置管理（需要管理员权限）
			systemConfig := protected.Group("/system-config")
			{
				systemConfig.GET("", handlers.SystemConfig.GetSystemConfig)
				systemConfig.PUT("", handlers.SystemConfig.UpdateSystemConfig)
				systemConfig.GET("/stats", handlers.SystemConfig.GetSystemStats)
			}
		}
	}

	// 静态文件服务 - 修复前端资源访问
	router.Static("/assets", cfg.Frontend.StaticPath+"/assets")
	router.StaticFile("/icon.svg", cfg.Frontend.StaticPath+"/icon.svg")
	router.StaticFile("/logo.svg", cfg.Frontend.StaticPath+"/logo.svg")
	router.StaticFile("/favicon.ico", cfg.Frontend.StaticPath+"/favicon.ico")
	router.StaticFile("/demo.png", cfg.Frontend.StaticPath+"/demo.png")

	// 管理后台路由 - 确保 /admin 路径下的所有请求都返回前端页面
	router.GET("/admin", func(c *gin.Context) {
		c.File(cfg.Frontend.StaticPath + "/index.html")
	})
	router.GET("/admin/*path", func(c *gin.Context) {
		c.File(cfg.Frontend.StaticPath + "/index.html")
	})

	// 前端页面服务 - 所有非API路由都返回 index.html (SPA)
	router.NoRoute(func(c *gin.Context) {
		// 如果是API请求，返回404
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(404, gin.H{"error": "API endpoint not found"})
			return
		}
		// 否则返回前端页面
		c.File(cfg.Frontend.StaticPath + "/index.html")
	})

	return router
}
