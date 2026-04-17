package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os/signal"
	"syscall"

	httpapi "github.com/slan/server/server-biz/api/http"
	"github.com/slan/server/server-biz/internal/infra"
	"github.com/slan/server/server-biz/internal/service"
)

// main 是 server-biz 的进程入口。
//
// 当前实现使用本地默认配置启动 HTTP 控制面服务。
func main() {
	configPath := flag.String("config", "", "path to server-biz config yaml")
	flag.Parse()

	cfg, err := infra.LoadConfig(*configPath)
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	runtime, err := infra.InitRuntime(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer runtime.Close()

	server := &http.Server{
		Addr:    cfg.HTTP.Address,
		Handler: httpapi.NewRouter(cfg, service.NewDBServices(cfg, runtime)),
	}

	log.Printf("server-biz listening on %s", cfg.HTTP.Address)
	log.Printf("server-biz postgres connected to %s:%d/%s", cfg.Postgres.Host, cfg.Postgres.Port, cfg.Postgres.Database)
	log.Printf("server-biz redis connected to %s db=%d", cfg.Redis.Address(), cfg.Redis.Database)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
