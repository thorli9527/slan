package api

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeJSONRejectsOversizedBody(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("POST", "/", strings.NewReader(strings.Repeat("x", int(MaxJSONRequestBodyBytes)+1)))
	var target map[string]any
	if err := DecodeJSON(req, &target); err == nil {
		t.Fatal("expected oversized body error")
	}
}

func TestDecodeJSONRejectsTrailingValue(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("POST", "/", strings.NewReader(`{"ok":true} {"extra":true}`))
	var target map[string]any
	if err := DecodeJSON(req, &target); err == nil {
		t.Fatal("expected trailing value error")
	}
}

func TestDecodeJSONEmptyBodyReturnsEOF(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("POST", "/", http.NoBody)
	var target map[string]any
	if err := DecodeJSON(req, &target); !errors.Is(err, io.EOF) {
		t.Fatalf("expected EOF, got %v", err)
	}
}
