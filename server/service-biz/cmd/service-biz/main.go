package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	serviceapp "github.com/slan/service-biz/internal/app"
)

func main() {
	addr := env("SLAN_BIZ_ADDR", ":39080")
	routeSet := env("SLAN_BIZ_ROUTE_SET", serviceapp.RouteSetAll)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	appServer := serviceapp.NewServer()
	var background sync.WaitGroup
	startBackground := func(run func()) {
		background.Add(1)
		go func() {
			defer background.Done()
			run()
		}()
	}
	if routeSetConsumesMQTTUpstream(routeSet) {
		startBackground(func() { startMQTTControlUpConsumer(ctx, appServer) })
	}
	if routeSetPublishesNetworkVersions(routeSet) {
		startBackground(func() { startNetworkVersionPublisher(ctx, appServer) })
		startBackground(func() { startNetworkEventDeliveryRetry(ctx, appServer) })
		startBackground(func() { startExpiredBootstrapKeyCleanup(ctx, appServer) })
	}
	httpServer := &http.Server{Addr: addr, Handler: appServer.RoutesFor(routeSet)}
	serverErr := make(chan error, 1)
	log.Printf("service-biz listening on %s routeSet=%s", addr, routeSet)
	go func() { serverErr <- httpServer.ListenAndServe() }()
	select {
	case err := <-serverErr:
		if err != nil && err != http.ErrServerClosed {
			log.Printf("service-biz HTTP server stopped: %v", err)
		}
		stop()
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("service-biz graceful shutdown failed: %v", err)
	}
	backgroundDone := make(chan struct{})
	go func() {
		background.Wait()
		close(backgroundDone)
	}()
	select {
	case <-backgroundDone:
	case <-shutdownCtx.Done():
		log.Printf("service-biz background shutdown timed out: %v", shutdownCtx.Err())
	}
}

func startExpiredBootstrapKeyCleanup(ctx context.Context, server *serviceapp.Server) {
	cleanup := func() {
		operationCtx, cancel := context.WithTimeout(ctx, time.Minute)
		deleted, err := server.Container().Services.Devices.BootstrapAuth.CleanupExpiredDeviceBootstrapKeys(operationCtx)
		cancel()
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			log.Printf("expired device bootstrap key cleanup failed: %v", err)
			return
		}
		log.Printf("expired device bootstrap key cleanup deleted=%d", deleted)
	}
	cleanup()
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cleanup()
		}
	}
}

func startNetworkEventDeliveryRetry(ctx context.Context, server *serviceapp.Server) {
	retry := func() {
		operationCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		result, err := server.Container().Services.Messaging.BrokerWebhook.RetryNetworkEventDeliveries(operationCtx)
		cancel()
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			log.Printf("network event delivery retry partial failure: %v", err)
		}
		log.Printf(
			"network event delivery retry scanned=%d republished=%d fallbackPublished=%d cleaned=%d",
			result.Scanned,
			result.Republished,
			result.FallbackPublished,
			result.Cleaned,
		)
	}
	retry()
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			retry()
		}
	}
}

func routeSetPublishesNetworkVersions(routeSet string) bool {
	return routeSet == serviceapp.RouteSetAll || routeSet == serviceapp.RouteSetApp
}

func startNetworkVersionPublisher(ctx context.Context, server *serviceapp.Server) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		operationCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		result, err := server.Container().Services.Network.CoreAccess.PushNetworkVersionHeartbeats(operationCtx)
		cancel()
		if ctx.Err() != nil {
			return
		}
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

func startMQTTControlUpConsumer(ctx context.Context, server *serviceapp.Server) {
	if !waitContext(ctx, 2*time.Second) {
		return
	}
	for {
		if err := server.Container().Services.Messaging.BrokerWebhook.StartControlUpConsumer(ctx); err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("start mqtt control/up consumer retry: %v", err)
			if !waitContext(ctx, 3*time.Second) {
				return
			}
			continue
		}
		return
	}
}

func waitContext(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
