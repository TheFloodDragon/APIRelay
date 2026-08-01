package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/apirelay/apirelay/common"
	"github.com/apirelay/apirelay/common/config"
	"github.com/apirelay/apirelay/common/logger"
	"github.com/apirelay/apirelay/model"
	"github.com/apirelay/apirelay/relay/circuitbreaker"
	"github.com/apirelay/apirelay/router"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

func main() {
	// run 负责全部初始化与运行；错误统一返回，使 os.Exit 只在所有 defer 执行完后调用。
	// 直接在 run 内部用 logger.Fatal 会走 os.Exit 跳过 defer，导致异步 worker 未 flush、
	// in-flight 请求的预扣额度不归还。
	if err := run(); err != nil {
		// logger 可能尚未初始化（L() 此时返回 Nop），因此同时写 stderr 保证可见。
		logger.L().Error("apirelay exited with error", zap.Error(err))
		logger.Sync()
		fmt.Fprintf(os.Stderr, "apirelay exited with error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if err := logger.Init(cfg.Log.Level, cfg.Log.Format, cfg.Log.Path); err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	defer logger.Sync()

	if err := model.InitDatabases(&cfg.Database, cfg.LogDatabase); err != nil {
		return fmt.Errorf("init db: %w", err)
	}
	defer func() {
		if err := model.CloseDatabases(); err != nil {
			logger.L().Warn("close databases failed", zap.Error(err))
		}
	}()

	// 启动异步 worker（日志落库 + 配额结算），退出前优雅 flush。
	// defer 为 LIFO：先排空 worker，再关闭数据库。
	model.StartAsyncWorker()
	defer model.StopAsyncWorker()

	// 日志保留期清理（默认关闭，需在 log_retention 中显式开启）。
	// 停机时先停清理再排空 worker，避免清理事务与日志落库争抢 SQLite 写连接。
	model.StartLogRetentionWorker(cfg.LogRetention)
	defer model.StopLogRetentionWorker()

	// 初始化熔断器管理器
	circuitbreaker.InitManager(circuitbreaker.Config{
		FailureThreshold:   cfg.Relay.CircuitBreaker.FailureThreshold,
		SuccessThreshold:   cfg.Relay.CircuitBreaker.SuccessThreshold,
		TimeoutSeconds:     cfg.Relay.CircuitBreaker.TimeoutSeconds,
		ErrorRateThreshold: cfg.Relay.CircuitBreaker.ErrorRateThreshold,
		MinRequests:        cfg.Relay.CircuitBreaker.MinRequests,
		WindowSeconds:      cfg.Relay.CircuitBreaker.WindowSeconds,
		ChannelMaxRetries:  cfg.Relay.ChannelMaxRetries,
	})

	if err := bootstrap(cfg); err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}

	r, err := router.Setup(cfg)
	if err != nil {
		return fmt.Errorf("setup router: %w", err)
	}
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)

	// 监听 SIGINT/SIGTERM，收到信号后优雅停机。
	// defer 顺序（LIFO）：先 Shutdown 排水（在此函数体内显式执行）→ 再 StopAsyncWorker → 最后 logger.Sync，
	// 确保 in-flight 请求先排水结算，避免预扣额度未归还。
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	server := &http.Server{
		Addr:    addr,
		Handler: r,
		// 限制读取请求头的时间，避免 Slowloris 用慢速请求头长期占满连接。
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		// 不设 WriteTimeout / ReadTimeout：流式转发（SSE）需要长时间持续写出，
		// 请求整体超时由 relay.RequestTimeout 在 context 层面控制。
	}
	serverErr := make(chan error, 1)
	go func() {
		logger.L().Info("apirelay starting", zap.String("addr", addr))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		// 返回错误而非 Fatal：让 defer 链完成 worker flush 与 DB 关闭。
		return fmt.Errorf("server exited: %w", err)
	case <-ctx.Done():
		logger.L().Info("shutdown signal received, draining in-flight requests")
		stop() // 恢复默认信号处理，允许再次 Ctrl-C 强制退出
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.L().Error("graceful shutdown failed", zap.Error(err))
		} else {
			logger.L().Info("http server drained")
		}
	}
	return nil
}

// bootstrap 首次启动时创建管理员；并在配置了固定 root token 时幂等确保其存在。
func bootstrap(cfg *config.Config) error {
	count, err := model.CountUsers()
	if err != nil {
		return fmt.Errorf("count users: %w", err)
	}

	var adminID int
	if count == 0 {
		username := cfg.Auth.InitialAdminUsername
		pw := cfg.Auth.InitialAdminPassword
		if pw == "" {
			if !cfg.Auth.AllowInsecureDefaultAdmin {
				return fmt.Errorf("initial admin password is required; set APIRELAY_INITIAL_ADMIN_PASSWORD or auth.initial_admin_password")
			}
			pw = "admin123"
			logger.L().Warn("using insecure default admin password; configure auth.initial_admin_password for production",
				zap.String("username", username),
			)
		}

		admin := &model.User{Username: username, Role: model.RoleAdmin}
		if err := admin.SetPassword(pw); err != nil {
			return fmt.Errorf("set admin password: %w", err)
		}
		if err := model.CreateUser(admin); err != nil {
			return fmt.Errorf("create admin: %w", err)
		}
		adminID = admin.Id
		logger.L().Info("initial admin created", zap.String("username", username), zap.String("password", common.MaskSecret(pw)))

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
				return fmt.Errorf("create root token: %w", err)
			}
			return nil
		}
		logger.L().Info("initial root token created", zap.String("key", common.MaskSecret(plain)))
		return nil
	}

	// 非首次启动：若配置了固定 root token，则幂等确保其存在且可用。
	if cfg.Auth.InitialRootToken == "" {
		return nil
	}
	if _, err := model.GetTokenByKey(cfg.Auth.InitialRootToken); err == nil {
		logger.L().Info("configured root token already exists", zap.String("key", common.MaskSecret(cfg.Auth.InitialRootToken)))
		return nil
	}
	if admin, err := model.GetUserByUsername(cfg.Auth.InitialAdminUsername); err == nil {
		adminID = admin.Id
	} else if admin, err := model.GetUserByUsername("admin"); err == nil {
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
		return fmt.Errorf("ensure root token: %w", err)
	}
	logger.L().Info("configured root token ensured", zap.String("key", common.MaskSecret(cfg.Auth.InitialRootToken)))
	return nil
}
