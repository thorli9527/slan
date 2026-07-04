package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"

	servicepkg "github.com/slan/service-biz/internal/service"
)

type userSessionResolver interface {
	GetUserSession(ctx context.Context, accessToken string) (servicepkg.AuthSessionView, error)
}

func DecodeJSON(r *http.Request, target any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(target)
}

func DecodeJSONOrError(w http.ResponseWriter, r *http.Request, target any) bool {
	if err := DecodeJSON(r, target); err != nil {
		WriteError(w, servicepkg.ErrInvalidArgument)
		return false
	}
	return true
}

func DecodeJSONIfPresentOrError(w http.ResponseWriter, r *http.Request, target any) bool {
	err := DecodeJSON(r, target)
	if err == nil || errors.Is(err, io.EOF) {
		return true
	}
	WriteError(w, servicepkg.ErrInvalidArgument)
	return false
}

func PathOrQuery(r *http.Request, pathKey, queryKey string) string {
	if value := strings.TrimSpace(r.PathValue(pathKey)); value != "" {
		return value
	}
	if value := strings.TrimSpace(r.URL.Query().Get(pathKey)); value != "" {
		return value
	}
	if value := strings.TrimSpace(pathFallbackValue(r, pathKey)); value != "" {
		return value
	}
	return strings.TrimSpace(r.URL.Query().Get(queryKey))
}

func pathFallbackValue(r *http.Request, pathKey string) string {
	resource := resourceSegmentForKey(pathKey)
	if resource == "" {
		return ""
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	for index, part := range parts {
		if part != resource {
			continue
		}
		if index+1 < len(parts) {
			return strings.TrimSpace(parts[index+1])
		}
	}
	return ""
}

func resourceSegmentForKey(pathKey string) string {
	switch pathKey {
	case "userId":
		return "users"
	case "deviceId":
		return "devices"
	case "networkId":
		return "networks"
	case "inviteId":
		return "invites"
	case "keyId":
		return "device-bootstrap-keys"
	case "zoneId":
		return "zones"
	case "recordId":
		return "records"
	case "mappingId":
		return "public-mappings"
	case "groupId":
		return "device-groups"
	case "securityGroupId":
		return "security-groups"
	case "ruleId":
		return "rules"
	case "securityRuleId":
		return "security-rules"
	case "operatorId":
		return "operators"
	case "customerId":
		return "customers"
	case "planCode":
		return "plans"
	case "productId":
		return "products"
	case "productCode":
		return "products"
	case "orderId":
		return "orders"
	case "renewalId":
		return "renewals"
	case "nodeId":
		return "nodes"
	case "relayNodeId":
		return "relay-nodes"
	case "punchNodeId":
		return "punch-nodes"
	case "downloadId":
		return "client-downloads"
	}
	if strings.HasSuffix(pathKey, "Id") {
		name := strings.TrimSuffix(pathKey, "Id")
		parts := splitCamelLower(name)
		if len(parts) > 0 {
			return strings.Join(parts, "-") + "s"
		}
	}
	return ""
}

func splitCamelLower(value string) []string {
	if value == "" {
		return nil
	}
	var (
		parts []string
		start int
	)
	for index := 1; index < len(value); index++ {
		if value[index] >= 'A' && value[index] <= 'Z' {
			parts = append(parts, strings.ToLower(value[start:index]))
			start = index
		}
	}
	parts = append(parts, strings.ToLower(value[start:]))
	return slices.DeleteFunc(parts, func(item string) bool { return item == "" })
}

func SetIfEmpty(target *string, value string) {
	if *target == "" {
		*target = strings.TrimSpace(value)
	}
}

func FirstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func AccessTokenFromRequest(r *http.Request) string {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	if token := strings.TrimSpace(r.URL.Query().Get("accessToken")); token != "" {
		return token
	}
	return strings.TrimSpace(r.Header.Get("X-Access-Token"))
}

func ResolveUserIDFromRequest(r *http.Request, useCase userSessionResolver, target *string) {
	SetIfEmpty(target, PathOrQuery(r, "userId", "userId"))
	if strings.TrimSpace(*target) != "" {
		return
	}
	session, err := useCase.GetUserSession(r.Context(), AccessTokenFromRequest(r))
	if err != nil {
		return
	}
	SetIfEmpty(target, session.User.UserID)
}
