package router

import (
	"github.com/apirelay/apirelay/common/config"
	"github.com/apirelay/apirelay/controller"
	"github.com/apirelay/apirelay/middleware"
	"github.com/apirelay/apirelay/relay"
	"github.com/apirelay/apirelay/web"

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

	// 内嵌前端（SPA fallback）
	web.Register(r)

	return r
}

// registerRelayRoutes 注册对外协议端点（需 token 鉴权）。
func registerRelayRoutes(r *gin.Engine, cfg *config.Config) {
	relayer := relay.NewRelayer(&cfg.Relay)

	v1 := r.Group("/v1")
	v1.Use(middleware.TokenAuth())
	{
		// OpenAI 兼容端点
		v1.GET("/models", relayer.HandleListModels)
		v1.POST("/chat/completions", relayer.HandleOpenAIChat)
		
		// Anthropic 端点
		v1.POST("/messages", relayer.HandleAnthropicMessages)
		
		// OpenAI Responses 端点
		v1.POST("/responses", relayer.HandleResponses)
	}
}

// registerAdminRoutes 注册管理后台 API。
func registerAdminRoutes(r *gin.Engine) {
	// 公开：登录
	r.POST("/api/auth/login", controller.AdminLogin)

	// 受保护：需会话鉴权
	api := r.Group("/api")
	api.Use(controller.AdminAuth())
	{
		api.POST("/auth/logout", controller.AdminLogout)
		api.GET("/auth/me", controller.CurrentUser)

		api.GET("/dashboard", controller.Dashboard)

		api.GET("/channel-types", controller.ChannelTypes)
		api.GET("/protocols", controller.ListProtocols)
		api.GET("/channels", controller.ListChannels)
		api.POST("/channels", controller.CreateChannel)
		api.POST("/channels/reorder", controller.ReorderChannels)
		api.PUT("/channels/:id", controller.UpdateChannel)
		api.DELETE("/channels/:id", controller.DeleteChannel)
		api.GET("/channels/:id/models", controller.ProbeChannelModels)
		api.POST("/channels/probe-models", controller.ProbeModelsByConfig)
		api.POST("/channels/:id/test", controller.TestChannelModel)
		api.POST("/channels/test", controller.TestChannelByConfig)

		api.GET("/models", controller.ListAggregatedModels)

		api.GET("/settings/protocol-rules", controller.GetProtocolRules)
		api.PUT("/settings/protocol-rules", controller.UpdateProtocolRules)
		api.GET("/settings/model-prices", controller.GetModelPrices)
		api.PUT("/settings/model-prices", controller.UpdateModelPrices)

		api.GET("/tokens", controller.ListTokens)
		api.POST("/tokens", controller.CreateToken)
		api.DELETE("/tokens/:id", controller.DeleteToken)

		api.GET("/logs", controller.ListLogs)
	}
}
