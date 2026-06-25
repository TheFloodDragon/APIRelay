package router

import (
	"github.com/apirelay/apirelay/common/config"
	"github.com/apirelay/apirelay/controller"
	"github.com/apirelay/apirelay/middleware"
	"github.com/apirelay/apirelay/relay"

	"github.com/gin-gonic/gin"
)

// Setup 装配所有路由。
func Setup(cfg *config.Config) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	r.Use(middleware.Recovery())
	r.Use(middleware.RequestID())
	r.Use(middleware.AccessLog())
	r.Use(middleware.CORS())

	// 健康检查
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	registerRelayRoutes(r, cfg)
	registerAdminRoutes(r)

	return r
}

// registerRelayRoutes 注册对外协议端点（需 token 鉴权）。
func registerRelayRoutes(r *gin.Engine, cfg *config.Config) {
	relayer := relay.NewRelayer(&cfg.Relay)

	v1 := r.Group("/v1")
	v1.Use(middleware.TokenAuth())
	{
		v1.POST("/chat/completions", relayer.HandleOpenAIChat)
		v1.POST("/messages", relayer.HandleAnthropicMessages)
		v1.POST("/responses", relayer.HandleResponses)
	}
}

// registerAdminRoutes 注册管理后台 API（MVP 暂未加管理鉴权，阶段5补充）。
func registerAdminRoutes(r *gin.Engine) {
	api := r.Group("/api")
	{
		api.GET("/dashboard", controller.Dashboard)

		api.GET("/channels", controller.ListChannels)
		api.POST("/channels", controller.CreateChannel)
		api.PUT("/channels/:id", controller.UpdateChannel)
		api.DELETE("/channels/:id", controller.DeleteChannel)

		api.GET("/tokens", controller.ListTokens)
		api.POST("/tokens", controller.CreateToken)
		api.DELETE("/tokens/:id", controller.DeleteToken)

		api.GET("/logs", controller.ListLogs)
	}
}
