package bizclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPunchRegistrationAndHeartbeat(t *testing.T) {
	requests := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Slan-Internal-Token") != "wire-token" {
			t.Fatalf("internal token missing")
		}
		requests = append(requests, r.Method+" "+r.URL.Path)
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := New(server.URL, "wire-token")
	if err := client.UpsertPunchNode(context.Background(), PunchNode{NodeID: "punch-1", Host: "203.0.113.10", UDPPort: 29130}); err != nil {
		t.Fatal(err)
	}
	if err := client.HeartbeatPunchNode(context.Background(), "punch-1", true); err != nil {
		t.Fatal(err)
	}
	want := []string{"PUT /internal/wire/admin/punch-nodes", "POST /internal/wire/admin/punch-nodes/punch-1/heartbeat"}
	if len(requests) != len(want) || requests[0] != want[0] || requests[1] != want[1] {
		t.Fatalf("requests = %#v", requests)
	}
}
