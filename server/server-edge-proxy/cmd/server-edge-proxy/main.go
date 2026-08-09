package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	apiUpstream, err := url.Parse(requiredEnv("SLAN_EDGE_PROXY_API_UPSTREAM"))
	if err != nil || (apiUpstream.Scheme != "http" && apiUpstream.Scheme != "https") || apiUpstream.Host == "" {
		return fmt.Errorf("invalid SLAN_EDGE_PROXY_API_UPSTREAM")
	}
	mqttUpstream := requiredEnv("SLAN_EDGE_PROXY_MQTT_UPSTREAM")
	if _, _, err := net.SplitHostPort(mqttUpstream); err != nil {
		return fmt.Errorf("invalid SLAN_EDGE_PROXY_MQTT_UPSTREAM: %w", err)
	}
	apiServer := &http.Server{
		Addr:              envOr("SLAN_EDGE_PROXY_API_LISTEN_ADDR", ":28080"),
		Handler:           apiProxyHandler(apiUpstream),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       90 * time.Second,
	}
	mqttListener, err := net.Listen("tcp", envOr("SLAN_EDGE_PROXY_MQTT_LISTEN_ADDR", ":1883"))
	if err != nil {
		return fmt.Errorf("listen MQTT proxy: %w", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	errCh := make(chan error, 2)
	go func() {
		log.Printf("API proxy listening on %s upstream=%s", apiServer.Addr, apiUpstream.Redacted())
		if err := apiServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()
	go func() {
		log.Printf("MQTT proxy listening on %s upstream=%s", mqttListener.Addr(), mqttUpstream)
		errCh <- serveTCPProxy(ctx, mqttListener, mqttUpstream)
	}()
	select {
	case <-ctx.Done():
	case err := <-errCh:
		stop()
		return err
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = mqttListener.Close()
	return apiServer.Shutdown(shutdownCtx)
}

func apiProxyHandler(upstream *url.URL) http.Handler {
	proxy := httputil.NewSingleHostReverseProxy(upstream)
	proxy.Transport = &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          256,
		MaxIdleConnsPerHost:   128,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
		log.Printf("API proxy error: %v", err)
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"status":"ok"}`)
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/api/app/") && r.URL.Path != "/api/device-auth/token" {
			http.NotFound(w, r)
			return
		}
		proxy.ServeHTTP(w, r)
	})
}

func serveTCPProxy(ctx context.Context, listener net.Listener, upstream string) error {
	for {
		client, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		go proxyTCPConnection(ctx, client, upstream)
	}
}

func proxyTCPConnection(ctx context.Context, client net.Conn, upstream string) {
	defer client.Close()
	server, err := (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext(ctx, "tcp", upstream)
	if err != nil {
		log.Printf("MQTT upstream dial failed: %v", err)
		return
	}
	defer server.Close()
	var wg sync.WaitGroup
	wg.Add(2)
	copyHalf := func(dst, src net.Conn) {
		defer wg.Done()
		_, _ = io.Copy(dst, src)
		if tcp, ok := dst.(*net.TCPConn); ok {
			_ = tcp.CloseWrite()
		}
	}
	go copyHalf(server, client)
	go copyHalf(client, server)
	wg.Wait()
}

func requiredEnv(key string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		log.Fatalf("%s is required", key)
	}
	return value
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
