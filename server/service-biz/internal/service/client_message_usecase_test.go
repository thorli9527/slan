package service

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestSendClientMessageRejectsOversizedBodyBeforeRepositoryAccess(t *testing.T) {
	t.Parallel()

	service := ClientMessageService{}
	_, err := service.SendClientMessage(context.Background(), SendClientMessageInput{
		NetworkID:      "network-a",
		FromDeviceID:   "device-a",
		TargetDeviceID: "device-b",
		Body:           strings.Repeat("x", maxClientMessageBodyBytes+1),
	})
	if !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("expected invalid argument, got %v", err)
	}
}
