package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/TheFloodDragon/APIRelay/internal/api"
	"github.com/TheFloodDragon/APIRelay/internal/model"
	"github.com/TheFloodDragon/APIRelay/internal/repository"
	"github.com/TheFloodDragon/APIRelay/internal/service"
	"github.com/TheFloodDragon/APIRelay/pkg/config"
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

	// 初始化仓库层
	channelRepo := repository.NewChannelRepository(model.DB)

	// 启动健康检查服务
	healthChecker := service.NewHealthChecker(channelRepo, cfg.Scheduler.HealthCheckInterval)
	healthChecker.Start()
	defer healthChecker.Stop()

	// 设置路由
	r := api.SetupRouter(model.DB, cfg)

	// 启动服务
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("APIRelay 启动成功，监听地址: %s", addr)
	log.Printf("管理接口: http://%s/api", addr)
	log.Printf("OpenAI兼容接口: http://%s/v1", addr)

	// 优雅关闭
	go func() {
		if err := r.Run(addr); err != nil {
			log.Fatalf("启动服务失败: %v", err)
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("正在关闭服务...")
}
