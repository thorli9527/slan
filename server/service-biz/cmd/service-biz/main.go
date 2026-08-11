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
	if err := serviceapp.ValidateRuntimeEnvironment(); err != nil {
		log.Fatal(err)
	}
	addr := env("SLAN_BIZ_ADDR", ":39080")
	routeSet := env("SLAN_BIZ_ROUTE_SET", serviceapp.RouteSetAll)
	server := serviceapp.NewServer()
	if routeSetConsumesMQTTUpstream(routeSet) {
		go startMQTTControlUpConsumer(server)
	}
	if routeSetPublishesNetworkVersions(routeSet) {
		go startNetworkVersionPublisher(server)
		go startNetworkEventDeliveryRetry(server)
		go startDeviceCredentialCleanup(server)
		go startDeviceOfflineRetry(server)
	}
	log.Printf("service-biz listening on %s routeSet=%s", addr, routeSet)
	if err := http.ListenAndServe(addr, server.RoutesFor(routeSet)); err != nil {
		log.Fatal(err)
	}
}

func startDeviceCredentialCleanup(server *serviceapp.Server) {
	run := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		deleted, err := server.Container().Services.Ops.DeviceCredentials.CleanupInvalidDeviceCredentials(ctx)
		cancel()
		if err != nil {
			log.Printf("invalid device credential cleanup failed: %v", err)
			return
		}
		if deleted > 0 {
			log.Printf("invalid device credential cleanup deleted=%d", deleted)
		}
	}
	run()
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		run()
	}
}

func startDeviceOfflineRetry(server *serviceapp.Server) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		result, err := server.Container().Services.Devices.DeviceOffline.RetryPendingDeviceOffline(ctx)
		cancel()
		if err != nil {
			log.Printf("device offline retry failed: %v", err)
			continue
		}
		if result.Scanned > 0 {
			log.Printf("device offline retry scanned=%d republished=%d revoked=%d", result.Scanned, result.Republished, result.Revoked)
		}
	}
}

func startNetworkEventDeliveryRetry(server *serviceapp.Server) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		result, err := server.Container().Services.Messaging.BrokerWebhook.RetryNetworkEventDeliveries(ctx)
		cancel()
		if err != nil {
			log.Printf("network event delivery retry partial failure: %v", err)
		}
		log.Printf("network event delivery retry scanned=%d republished=%d deleted=%d", result.Scanned, result.Republished, result.Deleted)
	}
}

func routeSetPublishesNetworkVersions(routeSet string) bool {
	return routeSet == serviceapp.RouteSetAll || routeSet == serviceapp.RouteSetApp
}

func startNetworkVersionPublisher(server *serviceapp.Server) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		result, err := server.Container().Services.Network.CoreAccess.PushNetworkVersionHeartbeats(ctx)
		cancel()
		if err != nil {
			log.Printf("periodic network version push partial failure: %v", err)
		}
		log.Printf(
			"periodic network version push scanned=%d eligible=%d published=%d skippedUnchanged=%d",
			result.ScannedNetworks,
			result.EligibleNetworks,
			result.PublishedNetworks,
			result.SkippedUnchanged,
		)
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
