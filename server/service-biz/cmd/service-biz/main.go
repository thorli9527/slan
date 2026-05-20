package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/slan/service-biz/internal/biz"
)

func main() {
	addr := env("SLAN_BIZ_ADDR", ":38080")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := biz.InitPostgresFromEnv(ctx)
	if err != nil {
		log.Fatalf("init postgres: %v", err)
	}
	if db != nil {
		defer db.Close()
	}
	server := biz.NewServerWithPostgres(db)
	log.Printf("service-biz listening on %s", addr)
	if err := http.ListenAndServe(addr, server.Routes()); err != nil {
		log.Fatal(err)
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
