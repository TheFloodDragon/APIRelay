package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed all:dist
var distFS embed.FS

// Register 将内嵌前端静态资源挂载到 gin engine。
// 非 /api、/v1、/healthz 的 GET 请求都回退到 index.html（SPA 路由）。
func Register(r *gin.Engine) {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		return
	}
	fileServer := http.FileServer(http.FS(sub))

	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path
		if c.Request.Method != http.MethodGet ||
			strings.HasPrefix(p, "/api") ||
			strings.HasPrefix(p, "/v1") ||
			strings.HasPrefix(p, "/healthz") {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "not found"})
			return
		}

		// 静态文件存在则直接返回，否则回退到 index.html
		clean := strings.TrimPrefix(p, "/")
		if clean == "" {
			clean = "index.html"
		}
		if _, err := fs.Stat(sub, clean); err != nil {
			c.Request.URL.Path = "/"
		}
		fileServer.ServeHTTP(c.Writer, c.Request)
	})
}
