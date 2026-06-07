package ui

import (
	"embed"
	"io/fs"
)

// Assets 保存构建后的前端静态资源。
//
// CI 构建会先执行 web 构建，然后把 web/dist 复制到 internal/ui/assets，
// 最终由 go:embed 打进后端二进制，实现前后端一体发布。
// 本地未构建前端时，assets 目录中只有占位文件，后端会自动回退到外部 static_path 或 JSON 状态页。
//
//go:embed all:assets
var Assets embed.FS

// EmbeddedFS 返回嵌入式前端文件系统。若没有 index.html，说明当前二进制未嵌入真实前端。
func EmbeddedFS() (fs.FS, bool) {
	if _, err := Assets.Open("assets/index.html"); err != nil {
		return nil, false
	}

	subFS, err := fs.Sub(Assets, "assets")
	if err != nil {
		return nil, false
	}

	return subFS, true
}
