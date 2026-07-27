package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	serviceapp "github.com/slan/service-biz/internal/app"
)

func main() {
	addr := env("SLAN_BIZ_ADDR", ":39080")
	routeSet := env("SLAN_BIZ_ROUTE_SET", serviceapp.RouteSetAll)
	server := serviceapp.NewServer()
	if routeSetConsumesMQTTUpstream(routeSet) {
		go startMQTTControlUpConsumer(server)
	}
	log.Printf("service-biz listening on %s routeSet=%s", addr, routeSet)
	if err := http.ListenAndServe(addr, server.RoutesFor(routeSet)); err != nil {
		log.Fatal(err)
	}
}

func routeSetConsumesMQTTUpstream(routeSet string) bool {
	switch routeSet {
	case serviceapp.RouteSetAll, serviceapp.RouteSetApp, serviceapp.RouteSetMQTT:
		return true
	default:
		return false
	}
}

func startMQTTControlUpConsumer(server *serviceapp.Server) {
	time.Sleep(2 * time.Second)
	for {
		if err := server.Container().Services.Messaging.BrokerWebhook.StartControlUpConsumer(context.Background()); err != nil {
			log.Printf("start mqtt control/up consumer retry: %v", err)
			time.Sleep(3 * time.Second)
			continue
		}
		return
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
