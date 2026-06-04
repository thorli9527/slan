package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/slan/server/server-wire/internal/bizclient"
	"github.com/slan/server/server-wire/internal/config"
	"github.com/slan/server/server-wire/internal/model"
	"github.com/slan/server/server-wire/internal/service"
	"github.com/slan/server/server-wire/internal/store"
)

// ListenAndServe 初始化存储、biz 客户端和 HTTP 路由，然后启动 server-wire。
func ListenAndServe(cfg config.Config) error {
	st := openStore(cfg)
	svc := service.NewWithBiz(st, bizclient.New(cfg.BizInternalURL, cfg.BizInternalToken))
	server := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: newMux(svc, cfg.BizInternalToken),
	}
	return server.ListenAndServe()
}

// openStore 根据配置选择 Postgres 或内存 Store。Postgres 不可用时降级到内存。
func openStore(cfg config.Config) store.Store {
	if strings.TrimSpace(cfg.PostgresDSN) == "" {
		return store.NewMemoryStore()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	st, err := store.NewPostgresStore(ctx, cfg.PostgresDSN)
	if err != nil {
		log.Printf("server-wire postgres unavailable, falling back to memory store: %v", err)
		return store.NewMemoryStore()
	}
	log.Printf("server-wire using postgres store")
	return st
}

// newMux 注册 server-wire 对外和内部 HTTP API。
func newMux(svc *service.Service, _ ...string) *http.ServeMux {
	mux := http.NewServeMux()
	registerRoutes(mux, svc)
	return mux
}

func registerRoutes(mux *http.ServeMux, svc *service.Service) {
	mux.HandleFunc("/healthz", handleHealthz)
	mux.HandleFunc("/peers/register", func(w http.ResponseWriter, r *http.Request) {
		handleRegisterPeer(svc, w, r)
	})
	mux.HandleFunc("/peers/endpoints", func(w http.ResponseWriter, r *http.Request) {
		handleUpdateEndpoints(svc, w, r)
	})
	mux.HandleFunc("/peers/path-health", func(w http.ResponseWriter, r *http.Request) {
		handleReportPathHealth(svc, w, r)
	})
	mux.HandleFunc("/peers/derp-health", func(w http.ResponseWriter, r *http.Request) {
		handleReportDerpHealth(svc, w, r)
	})
	mux.HandleFunc("/peers/active-path", func(w http.ResponseWriter, r *http.Request) {
		handleUpdateActivePath(svc, w, r)
	})
	mux.HandleFunc("/peers/", func(w http.ResponseWriter, r *http.Request) {
		handleGetPeerRoutes(svc, w, r)
	})
	mux.HandleFunc("/relay/tickets", func(w http.ResponseWriter, r *http.Request) {
		handleIssueRelayTicket(svc, w, r)
	})
	mux.HandleFunc("/derp/map", func(w http.ResponseWriter, r *http.Request) {
		handleGetDerpMap(svc, w, r)
	})
	mux.HandleFunc("/derp/tickets", func(w http.ResponseWriter, r *http.Request) {
		handleIssueDerpTicket(svc, w, r)
	})
	mux.HandleFunc("/path-plan", func(w http.ResponseWriter, r *http.Request) {
		handlePathPlan(svc, w, r)
	})
	mux.HandleFunc("/internal/wire/ticket-key-status", handleTicketKeyStatus)
	mux.HandleFunc("/internal/wire/peers/", func(w http.ResponseWriter, r *http.Request) {
		handleInternalPeerRoutes(svc, w, r)
	})
	mux.HandleFunc("/internal/wire/networks/", func(w http.ResponseWriter, r *http.Request) {
		handleInternalNetworkRoutes(svc, w, r)
	})
}

func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"service": "server-wire",
	})
}

func handleTicketKeyStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, store.TicketKeyStatus())
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func decodeJSON(r *http.Request, target any) error {
	return json.NewDecoder(r.Body).Decode(target)
}

func handleRegisterPeer(svc *service.Service, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req model.RegisterPeerRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err)
		return
	}
	resp, err := svc.RegisterPeer(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "register_failed", err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func handleUpdateEndpoints(svc *service.Service, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req model.UpdateEndpointsRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err)
		return
	}
	peer, err := svc.UpdateEndpoints(req)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"peer": peer})
}

func handleReportPathHealth(svc *service.Service, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req model.ReportPathHealthRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err)
		return
	}
	peer, err := svc.ReportPathHealth(req)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"peer": peer})
}

func handleUpdateActivePath(svc *service.Service, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req model.UpdateActivePathRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err)
		return
	}
	peer, err := svc.UpdateActivePath(req)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"peer": peer})
}

func handleReportDerpHealth(svc *service.Service, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req model.ReportDerpHealthRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err)
		return
	}
	peer, err := svc.ReportDerpHealth(req)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"peer": peer})
}

func handleIssueRelayTicket(svc *service.Service, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req model.IssueRelayTicketRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err)
		return
	}
	resp, err := svc.IssueRelayTicket(req)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func handlePathPlan(svc *service.Service, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req model.PathPlanRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err)
		return
	}
	plan, err := svc.BuildPathPlan(req)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

func handleGetDerpMap(svc *service.Service, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	resp, err := svc.GetDerpMap()
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func handleIssueDerpTicket(svc *service.Service, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req model.IssueDerpTicketRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err)
		return
	}
	resp, err := svc.IssueDerpTicket(req)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func handleGetPeerRoutes(svc *service.Service, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/peers/")
	if path == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if strings.HasSuffix(path, "/runtime-config") {
		peerID := strings.TrimSuffix(path, "/runtime-config")
		peerID = strings.TrimSuffix(peerID, "/")
		view, err := svc.GetPeerRuntimeConfig(peerID)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, view)
		return
	}
	peerID := strings.TrimSuffix(path, "/")
	peer, err := svc.GetPeer(peerID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"peer": peer})
}

func handleInternalPeerRoutes(svc *service.Service, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/internal/wire/peers/")
	switch {
	case strings.HasSuffix(path, "/authz"):
		peerID := strings.TrimSuffix(path, "/authz")
		peerID = strings.TrimSuffix(peerID, "/")
		view, err := svc.GetPeerAuthz(peerID)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, view)
	case strings.HasSuffix(path, "/runtime-config"):
		peerID := strings.TrimSuffix(path, "/runtime-config")
		peerID = strings.TrimSuffix(peerID, "/")
		view, err := svc.GetPeerRuntimeConfig(peerID)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, view)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func handleInternalNetworkRoutes(svc *service.Service, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/internal/wire/networks/")
	if !strings.HasSuffix(path, "/topology") {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	networkID := strings.TrimSuffix(path, "/topology")
	networkID = strings.TrimSuffix(networkID, "/")
	view, err := svc.GetNetworkTopology(networkID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func writeError(w http.ResponseWriter, status int, code string, err error) {
	writeJSON(w, status, map[string]any{
		"code":    code,
		"message": err.Error(),
	})
}

func writeStoreError(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, store.ErrPeerNotFound) {
		status = http.StatusNotFound
	}
	writeError(w, status, "request_failed", err)
}
