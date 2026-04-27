package impl

import (
	"strings"
	"testing"
)

func TestSanitizeDNSWildcardsAllowsSecondLevelWildcardDomains(t *testing.T) {
	got, err := sanitizeDNSWildcards([]string{
		"*.xx.com=10.0.0.2",
		"*xx.net=10.0.0.3",
		"*.*.xx.com=10.0.0.4",
	})
	if err != nil {
		t.Fatalf("sanitizeDNSWildcards returned error: %v", err)
	}
	want := []string{
		"*.xx.com=10.0.0.2",
		"*.xx.net=10.0.0.3",
		"*.*.xx.com=10.0.0.4",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("unexpected wildcards: got %v want %v", got, want)
	}
}

func TestSanitizeDNSWildcardsRejectsBroadOrInvalidRules(t *testing.T) {
	cases := [][]string{
		{"*.com=10.0.0.2"},
		{"*.xx.com=not-ip"},
		{"api.*.xx.com=10.0.0.2"},
		{"*=10.0.0.2"},
		{"*.xx=10.0.0.2"},
	}
	for _, tc := range cases {
		if got, err := sanitizeDNSWildcards(tc); err == nil {
			t.Fatalf("expected error for %v, got %v", tc, got)
		}
	}
}
