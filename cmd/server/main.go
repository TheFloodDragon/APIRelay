package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/yourusername/apirelay/internal/api"
	"github.com/yourusername/apirelay/internal/model"
	"github.com/yourusername/apirelay/pkg/config"
)

func main() {
	configPath := flag.String("config", "", "配置文件路径")
	flag.Parse()

	// 加载配置
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 初始化数据库
	if cfg.Database.Type != "sqlite" {
		log.Fatalf("当前版本仅支持 SQLite 数据库")
	}

	if err := model.InitDB(cfg.Database.Path); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	defer model.CloseDB()

	// 设置路由
	r := api.SetupRouter(model.DB, cfg)

	// 启动服务
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("APIRelay 启动成功，监听地址: %s", addr)
	log.Printf("管理接口: http://%s/api", addr)
	log.Printf("OpenAI兼容接口: http://%s/v1", addr)

	if err := r.Run(addr); err != nil {
		log.Fatalf("启动服务失败: %v", err)
	}
}
