package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"

	"github.com/slan/server/server-biz/internal/service"
)

type wsSession struct {
	userID    string
	deviceID  string
	nodeID    string
	networkID string
}

func (s wsSession) authorized() bool {
	return s.userID != "" && s.nodeID != "" && s.networkID != ""
}

var controlWSInstanceID = newControlWSInstanceID()
var controlWSSyncOnce sync.Once
var controlWSDeliveryRetryOnce sync.Once
var defaultConnectPlanThrottle = newConnectPlanThrottle()
var defaultPeerCandidateWindow = newPeerCandidateWindow()

func startControlWSDeliveryRetryLoop(deps routerDeps) {
}

func controlErrorCode(err error) string {
	code := "INTERNAL"
	switch {
	case errors.Is(err, service.ErrInvalidArgument):
		code = "INVALID_ARGUMENT"
	case errors.Is(err, service.ErrUnauthorized):
		code = "UNAUTHORIZED"
	case errors.Is(err, service.ErrForbidden):
		code = "FORBIDDEN"
	case errors.Is(err, service.ErrNotFound):
		code = "NOT_FOUND"
	case errors.Is(err, service.ErrConflict):
		code = "CONFLICT"
	}
	return code
}

func newControlWSInstanceID() string {
	var buf [6]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "instance"
	}
	return hex.EncodeToString(buf[:])
}
