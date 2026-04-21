package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	httpapi "github.com/slan/server/server-biz/api/http"
	"github.com/slan/server/server-biz/configs"
	"github.com/slan/server/server-biz/internal/service/impl"
)

// main 是 server-biz 的进程入口。
//
// 当前实现使用本地默认配置启动 HTTP 控制面服务。
func main() {
	configPath := flag.String("config", "", "path to server-biz config yaml")
	flag.Parse()

	cfg, err := configs.LoadConfig(*configPath)
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	runtime, err := configs.InitRuntime(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer runtime.Close()

	authService, deviceService, networkService, nodeService, bootstrapService, tokenVerifier, controlChannel, controlSync, messageDelivery, opsService := impl.NewDBServices(cfg, runtime)
	deps := httpapi.NewRouterDeps(
		authService,
		deviceService,
		networkService,
		nodeService,
		bootstrapService,
		tokenVerifier,
		controlChannel,
		controlSync,
		messageDelivery,
		opsService,
	)

	publicServer := &http.Server{
		Addr:    cfg.HTTP.Address,
		Handler: httpapi.NewPublicRouter(cfg, deps),
	}
	opsServer := &http.Server{
		Addr:    cfg.HTTP.OpsAddress,
		Handler: httpapi.NewOpsRouter(cfg, deps),
	}

	errCh := make(chan error, 2)
	go func() {
		if err := publicServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("public http: %w", err)
		}
	}()
	go func() {
		if err := opsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("ops http: %w", err)
		}
	}()

	log.Printf("server-biz public listening on %s", cfg.HTTP.Address)
	log.Printf("server-biz ops listening on %s", cfg.HTTP.OpsAddress)
	log.Printf("server-biz postgres connected to %s:%d/%s", cfg.Postgres.Host, cfg.Postgres.Port, cfg.Postgres.Database)
	log.Printf("server-biz redis connected to %s db=%d", cfg.Redis.Address(), cfg.Redis.Database)

	select {
	case <-ctx.Done():
	case err := <-errCh:
		log.Fatal(err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = publicServer.Shutdown(shutdownCtx)
	_ = opsServer.Shutdown(shutdownCtx)
	if err := shutdownCtx.Err(); err != nil && err != context.DeadlineExceeded {
		log.Printf("server-biz shutdown context error: %v", err)
	}
}
