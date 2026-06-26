package main

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/apirelay/apirelay/common"
	"github.com/apirelay/apirelay/common/config"
	"github.com/apirelay/apirelay/common/logger"
	"github.com/apirelay/apirelay/model"
	"github.com/apirelay/apirelay/router"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config failed: %v\n", err)
		os.Exit(1)
	}

	if err := logger.Init(cfg.Log.Level, cfg.Log.Format, cfg.Log.Path); err != nil {
		fmt.Fprintf(os.Stderr, "init logger failed: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	if err := model.InitDB(&cfg.Database); err != nil {
		logger.L().Fatal("init db failed", zap.Error(err))
	}

	// 启动异步 worker（日志落库 + 配额结算），退出前优雅 flush
	model.StartAsyncWorker()
	defer model.StopAsyncWorker()

	bootstrap(cfg)

	r := router.Setup(cfg)
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	logger.L().Info("apirelay starting", zap.String("addr", addr))
	if err := r.Run(addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.L().Fatal("server exited", zap.Error(err))
	}
}

// bootstrap 首次启动时创建管理员；并在配置了固定 root token 时幂等确保其存在。
func bootstrap(cfg *config.Config) {
	count, err := model.CountUsers()
	if err != nil {
		logger.L().Error("count users failed", zap.Error(err))
		return
	}

	var adminID int
	if count == 0 {
		admin := &model.User{Username: "admin", Role: model.RoleAdmin}
		pw := "admin123"
		_ = admin.SetPassword(pw)
		if err := model.CreateUser(admin); err != nil {
			logger.L().Error("create admin failed", zap.Error(err))
			return
		}
		adminID = admin.Id
		logger.L().Info("initial admin created", zap.String("username", "admin"), zap.String("password", pw))

		// 首次启动：创建初始令牌（配置为空则自动生成）
		plain := cfg.Auth.InitialRootToken
		if plain == "" {
			plain = common.NewToken("sk-")
		}
		tok := &model.Token{
			UserId:    adminID,
			Name:      "root",
			Status:    model.TokenStatusEnabled,
			Group:     cfg.Relay.DefaultGroup,
			Unlimited: true,
		}
		if err := model.CreateToken(tok, plain); err != nil {
			if !errors.Is(err, gorm.ErrDuplicatedKey) {
				logger.L().Error("create root token failed", zap.Error(err))
			}
			return
		}
		logger.L().Info("initial root token created", zap.String("key", plain))
		return
	}

	// 非首次启动：若配置了固定 root token，则幂等确保其存在且可用。
	if cfg.Auth.InitialRootToken == "" {
		return
	}
	if _, err := model.GetTokenByKey(cfg.Auth.InitialRootToken); err == nil {
		return // 已存在
	}
	if admin, err := model.GetUserByUsername("admin"); err == nil {
		adminID = admin.Id
	} else {
		adminID = 1
	}
	tok := &model.Token{
		UserId:    adminID,
		Name:      "root",
		Status:    model.TokenStatusEnabled,
		Group:     cfg.Relay.DefaultGroup,
		Unlimited: true,
	}
	if err := model.CreateToken(tok, cfg.Auth.InitialRootToken); err != nil {
		logger.L().Error("ensure root token failed", zap.Error(err))
		return
	}
	logger.L().Info("configured root token ensured", zap.String("key", cfg.Auth.InitialRootToken))
}
