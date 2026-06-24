package main

import (
	"log"
	"net/http"
	"os"

	serviceapp "github.com/slan/service-biz/internal/app"
)

func main() {
	addr := env("SLAN_BIZ_ADDR", ":39080")
	routeSet := env("SLAN_BIZ_ROUTE_SET", serviceapp.RouteSetAll)
	server := serviceapp.NewServer()
	log.Printf("service-biz listening on %s routeSet=%s", addr, routeSet)
	if err := http.ListenAndServe(addr, server.RoutesFor(routeSet)); err != nil {
		log.Fatal(err)
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
