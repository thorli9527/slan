package main

import (
	"testing"

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
		{routeSet: serviceapp.RouteSetWeb, want: false},
		{routeSet: serviceapp.RouteSetOps, want: false},
	}
	for _, test := range tests {
		if got := routeSetConsumesMQTTUpstream(test.routeSet); got != test.want {
			t.Fatalf("routeSetConsumesMQTTUpstream(%q) = %t, want %t", test.routeSet, got, test.want)
		}
	}
}
