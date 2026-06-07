package api

import (
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/TheFloodDragon/APIRelay/internal/api/handler"
	"github.com/TheFloodDragon/APIRelay/internal/api/middleware"
	"github.com/TheFloodDragon/APIRelay/internal/repository"
	relayclient "github.com/TheFloodDragon/APIRelay/internal/relay/client"
	relaycontroller "github.com/TheFloodDragon/APIRelay/internal/relay/controller"
	"github.com/TheFloodDragon/APIRelay/internal/router"
	"github.com/TheFloodDragon/APIRelay/internal/scheduler"
	"github.com/TheFloodDragon/APIRelay/internal/service"
	"github.com/TheFloodDragon/APIRelay/internal/ui"
	"github.com/TheFloodDragon/APIRelay/pkg/config"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SetupRouter 设置路由
func SetupRouter(db *gorm.DB, cfg *config.Config) *gin.Engine {
	gin.SetMode(cfg.Server.Mode)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.LoggerMiddleware())

	if cfg.CORS.Enabled {
		r.Use(middleware.CORSMiddleware())
	}

	// 仓库层
	channelRepo := repository.NewChannelRepository(db)
	modelRepo := repository.NewModelRepository(db)
	keyRepo := repository.NewAPIKeyRepository(db)
	logRepo := repository.NewLogRepository(db)

	// 服务层
	channelService := service.NewChannelService(channelRepo, modelRepo)
	schedulerService := scheduler.NewScheduler(channelRepo, cfg.Scheduler.Strategy)

	// 模型路由器（聚合中转站核心）
	modelRouter := router.NewModelRouter(modelRepo)

	// 处理器
	systemHandler := handler.NewSystemHandler()
	channelHandler := handler.NewChannelHandler(channelService)
	modelHandler := handler.NewModelHandler(modelRepo)
	keyHandler := handler.NewKeyHandler(keyRepo)
	logHandler := handler.NewLogHandler(logRepo)
	relayHTTPClient := relayclient.NewHTTPClient()
	relayController := relaycontroller.NewRelayController(
		schedulerService,
		modelRouter,
		relayHTTPClient,
		logRepo,
		modelRepo,
	)
	routeHandler := handler.NewRouteHandler(modelRouter)

	// 根路径和前端静态资源
	setupStaticRoutes(r, cfg)

	// 管理API
	apiGroup := r.Group("/api")
	{
		// 健康检查无需认证
		apiGroup.GET("/system/health", systemHandler.Health)

		// 其他管理接口需要认证
		apiGroup.Use(middleware.AuthMiddleware())

		// 系统管理
		apiGroup.GET("/system/info", systemHandler.Info)

		// 渠道管理
		apiGroup.GET("/channels", channelHandler.GetChannels)
		apiGroup.POST("/channels", channelHandler.CreateChannel)
		apiGroup.PUT("/channels/reorder", channelHandler.ReorderChannels)
		apiGroup.GET("/channels/:id", channelHandler.GetChannel)
		apiGroup.PUT("/channels/:id", channelHandler.UpdateChannel)
		apiGroup.DELETE("/channels/:id", channelHandler.DeleteChannel)
		apiGroup.POST("/channels/:id/models", channelHandler.FetchModels)
		apiGroup.POST("/channels/:id/test", channelHandler.TestChannel)

		// 模型管理
		apiGroup.GET("/models", modelHandler.GetModels)
		apiGroup.GET("/models/available", modelHandler.GetAvailableModels)
		apiGroup.DELETE("/models/:id", modelHandler.DeleteModel)

		// API密钥管理
		apiGroup.GET("/keys", keyHandler.GetKeys)
		apiGroup.POST("/keys", keyHandler.CreateKey)
		apiGroup.DELETE("/keys/:id", keyHandler.DeleteKey)

		// 日志查询
		apiGroup.GET("/logs", logHandler.GetLogs)

		// 模型路由管理（聚合中转站功能）
		apiGroup.GET("/routes", routeHandler.GetAllRoutes)
		apiGroup.POST("/routes/aliases", routeHandler.SetAlias)
		apiGroup.DELETE("/routes/aliases/:alias", routeHandler.DeleteAlias)
		apiGroup.POST("/routes/redirects", routeHandler.SetRedirect)
		apiGroup.DELETE("/routes/redirects/:source", routeHandler.DeleteRedirect)
		apiGroup.POST("/routes/groups", routeHandler.SetGroup)
		apiGroup.DELETE("/routes/groups/:group", routeHandler.DeleteGroup)
		apiGroup.POST("/routes/reload", routeHandler.ReloadRoutes)
	}

	// OpenAI兼容API
	v1Group := r.Group("/v1")
	v1Group.Use(middleware.APIKeyAuthMiddleware(keyRepo))
	{
		v1Group.GET("/models", relayController.GetModels)
		v1Group.POST("/responses", relayController.Responses)
		v1Group.POST("/chat/completions", relayController.ChatCompletions)
		v1Group.POST("/completions", relayController.Completions)
		v1Group.POST("/embeddings", relayController.Embeddings)
	}

	return r
}

func setupStaticRoutes(r *gin.Engine, cfg *config.Config) {
	staticPath := cfg.Server.StaticPath
	indexPath := filepath.Join(staticPath, "index.html")

	// 开发/部署时如果提供了外部 static_path，优先使用外部文件，方便热替换前端。
	if _, err := os.Stat(indexPath); err == nil {
		setupExternalStaticRoutes(r, staticPath, indexPath)
		return
	}

	// 发布构建时，GitHub Actions 会把 web/dist 嵌入二进制，实现前后端一体。
	if embeddedFS, ok := ui.EmbeddedFS(); ok {
		setupEmbeddedStaticRoutes(r, embeddedFS)
		return
	}

	setupStatusRoute(r)
}

func setupStatusRoute(r *gin.Engine) {
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"name":    "APIRelay",
			"version": "1.0.0",
			"status":  "running",
		})
	})
}

func setupExternalStaticRoutes(r *gin.Engine, staticPath, indexPath string) {
	assetsPath := filepath.Join(staticPath, "assets")
	if _, err := os.Stat(assetsPath); err == nil {
		r.Static("/assets", assetsPath)
	}

	r.GET("/", func(c *gin.Context) {
		c.File(indexPath)
	})

	r.NoRoute(func(c *gin.Context) {
		requestPath := c.Request.URL.Path
		if isAPIRoute(requestPath) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}

		filePath := filepath.Join(staticPath, strings.TrimPrefix(requestPath, "/"))
		if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
			c.File(filePath)
			return
		}

		c.File(indexPath)
	})
}

func setupEmbeddedStaticRoutes(r *gin.Engine, embeddedFS fs.FS) {
	if assetsFS, err := fs.Sub(embeddedFS, "assets"); err == nil {
		r.StaticFS("/assets", http.FS(assetsFS))
	}

	r.GET("/", func(c *gin.Context) {
		serveEmbeddedFile(c, embeddedFS, "index.html")
	})

	r.NoRoute(func(c *gin.Context) {
		requestPath := c.Request.URL.Path
		if isAPIRoute(requestPath) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}

		filePath := strings.TrimPrefix(requestPath, "/")
		if filePath != "" && serveEmbeddedFile(c, embeddedFS, filePath) {
			return
		}

		serveEmbeddedFile(c, embeddedFS, "index.html")
	})
}

func serveEmbeddedFile(c *gin.Context, embeddedFS fs.FS, filePath string) bool {
	file, err := embeddedFS.Open(filePath)
	if err != nil {
		return false
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil || info.IsDir() {
		return false
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return false
	}

	contentType := mime.TypeByExtension(filepath.Ext(filePath))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Data(http.StatusOK, contentType, data)
	return true
}

func isAPIRoute(path string) bool {
	return strings.HasPrefix(path, "/api/") ||
		strings.HasPrefix(path, "/v1/") ||
		path == "/models" ||
		path == "/responses" ||
		path == "/chat/completions" ||
		path == "/completions" ||
		path == "/embeddings"
}
