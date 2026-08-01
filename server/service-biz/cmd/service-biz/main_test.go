package main

import (
	"context"
	"testing"
	"time"

	serviceapp "github.com/slan/service-biz/internal/app"
)

func TestRouteSetConsumesMQTTUpstream(t *testing.T) {
	tests := []struct {
		routeSet string
		want     bool
	}{
		{routeSet: serviceapp.RouteSetAll, want: true},
		{routeSet: serviceapp.RouteSetApp, want: true},
		{routeSet: serviceapp.RouteSetMQTT, want: true},
		{routeSet: serviceapp.RouteSetOps, want: false},
	}
	for _, test := range tests {
		if got := routeSetConsumesMQTTUpstream(test.routeSet); got != test.want {
			t.Fatalf("routeSetConsumesMQTTUpstream(%q) = %t, want %t", test.routeSet, got, test.want)
		}
	}
}

func TestWaitContextStopsImmediatelyWhenCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	if waitContext(ctx, time.Second) {
		t.Fatal("canceled context unexpectedly completed the delay")
	}
	if elapsed := time.Since(started); elapsed >= 100*time.Millisecond {
		t.Fatalf("canceled delay returned too slowly: %s", elapsed)
	}
}
