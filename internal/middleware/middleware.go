package middleware

import (
	"augment-gateway/internal/config"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// CORS 跨域中间件
func CORS(cfg *config.Config) gin.HandlerFunc {
	if !cfg.Security.CORS.Enabled {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	corsConfig := cors.Config{
		AllowOrigins:     cfg.Security.CORS.AllowedOrigins,
		AllowMethods:     cfg.Security.CORS.AllowedMethods,
		AllowHeaders:     cfg.Security.CORS.AllowedHeaders,
		AllowCredentials: cfg.Security.CORS.AllowCredentials,
	}

	// 如果允许所有来源
	if len(corsConfig.AllowOrigins) == 1 && corsConfig.AllowOrigins[0] == "*" {
		corsConfig.AllowAllOrigins = true
		corsConfig.AllowOrigins = nil
	}

	return cors.New(corsConfig)
}
