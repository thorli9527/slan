package main

import (
	"log"
	"net/http"
	"os"

	"github.com/slan/service-biz-new/internal/biz"
)

func main() {
	addr := env("SLAN_BIZ_NEW_ADDR", ":38080")
	server := biz.NewServer()
	log.Printf("service-biz-new listening on %s", addr)
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
