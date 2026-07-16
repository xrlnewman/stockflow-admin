package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/xrlnewman/stockflow-admin/server/internal/config"
	"github.com/xrlnewman/stockflow-admin/server/internal/platform/cache"
	"github.com/xrlnewman/stockflow-admin/server/internal/platform/database"
	"github.com/xrlnewman/stockflow-admin/server/internal/platform/store"
	"github.com/xrlnewman/stockflow-admin/server/internal/transport/httpapi"
)

func main() {
	cfg := config.Load()
	st := store.NewMemoryStore()
	deps := httpapi.Dependencies{}
	if db, err := database.Open(cfg.DatabaseDSN); err != nil {
		slog.Warn("MySQL 未连接，使用内存演示模式", "error", err)
	} else {
		pingCtx, pingCancel := context.WithTimeout(context.Background(), 3*time.Second)
		pingErr := database.Ping(pingCtx, db)
		pingCancel()
		if pingErr != nil {
			slog.Warn("MySQL 未连接，使用内存演示模式", "error", pingErr)
			_ = db.Close()
		} else {
			persistence := database.NewSQLPersistence(db)
			loadCtx, loadCancel := context.WithTimeout(context.Background(), 5*time.Second)
			snapshot, loadErr := persistence.Load(loadCtx)
			loadCancel()
			if loadErr != nil {
				slog.Error("MySQL 持久化数据回载失败，停止启动", "error", loadErr)
				_ = db.Close()
				os.Exit(1)
			}
			st.Restore(snapshot)
			st.SetPersistence(persistence)
			deps.DB = db
			defer db.Close()
			slog.Info("MySQL 持久化数据已回载", "orders", len(snapshot.Orders), "events", len(snapshot.Events), "audits", len(snapshot.Audits), "reviews", len(snapshot.Reviews), "proofs", len(snapshot.Proofs))
		}
	}
	redisLocker := cache.NewRedisLocker(cfg.RedisAddr, cfg.RedisDB)
	redisCtx, redisCancel := context.WithTimeout(context.Background(), 2*time.Second)
	if err := redisLocker.Ping(redisCtx); err != nil {
		slog.Warn("Redis 未连接，使用无锁演示模式", "error", err)
	} else {
		deps.Redis = redisLocker
	}
	redisCancel()
	server := &http.Server{Addr: cfg.Addr, Handler: httpapi.NewRouterWithDeps(cfg, st, deps), ReadHeaderTimeout: 5 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		slog.Info("StockFlow API 已启动", "addr", cfg.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP 服务异常", "error", err)
			os.Exit(1)
		}
	}()
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
}
