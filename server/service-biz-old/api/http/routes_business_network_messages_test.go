package httpapi

import (
	"errors"
	"testing"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/service"
)

func TestAuthorizeClientMessageAllowsActiveTargetAndOwnedSender(t *testing.T) {
	assignments := []dto.NetworkAssignment{
		{DeviceID: "mac-a", UserID: "user-a", Status: "active"},
		{DeviceID: "iphone-b", UserID: "user-b", Status: "active"},
	}

	if err := authorizeClientMessage(assignments, "user-a", "iphone-b", "mac-a"); err != nil {
		t.Fatalf("expected message to be authorized, got %v", err)
	}
}

func TestAuthorizeClientMessageRejectsMissingOrDisabledTarget(t *testing.T) {
	assignments := []dto.NetworkAssignment{
		{DeviceID: "mac-a", UserID: "user-a", Status: "active"},
		{DeviceID: "iphone-b", UserID: "user-b", Status: "disabled"},
	}

	if err := authorizeClientMessage(assignments, "user-a", "iphone-b", "mac-a"); !errors.Is(err, service.ErrForbidden) {
		t.Fatalf("expected forbidden for disabled target, got %v", err)
	}
	if err := authorizeClientMessage(assignments, "user-a", "missing-device", "mac-a"); !errors.Is(err, service.ErrForbidden) {
		t.Fatalf("expected forbidden for missing target, got %v", err)
	}
}

func TestAuthorizeClientMessageRejectsUnownedSender(t *testing.T) {
	assignments := []dto.NetworkAssignment{
		{DeviceID: "mac-a", UserID: "user-a", Status: "active"},
		{DeviceID: "iphone-b", UserID: "user-b", Status: "active"},
	}

	if err := authorizeClientMessage(assignments, "user-a", "iphone-b", "iphone-b"); !errors.Is(err, service.ErrForbidden) {
		t.Fatalf("expected forbidden for unowned sender, got %v", err)
	}
}

func TestClientMessagePayloadUsesFinalControlData(t *testing.T) {
	payload := clientMessagePayload(
		"client-msg-1",
		"net-a",
		"mac-a",
		"iphone-b",
		"hello",
		map[string]any{"kind": "text"},
		1234,
	)

	if payload["messageId"] != "client-msg-1" || payload["networkId"] != "net-a" {
		t.Fatalf("unexpected message identity: %#v", payload)
	}
	if payload["fromDeviceId"] != "mac-a" || payload["targetDeviceId"] != "iphone-b" || payload["body"] != "hello" {
		t.Fatalf("unexpected message fields: %#v", payload)
	}
	metadata, ok := payload["metadata"].(map[string]any)
	if !ok || metadata["kind"] != "text" {
		t.Fatalf("unexpected metadata: %#v", payload["metadata"])
	}
	if payload["sentAtMs"] != int64(1234) {
		t.Fatalf("unexpected sentAtMs: %#v", payload["sentAtMs"])
	}
}
