package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

	servicepkg "github.com/slan/service-biz/internal/service"
)

const MaxJSONRequestBodyBytes int64 = 1 << 20

func DecodeJSON(r *http.Request, target any) error {
	return DecodeJSONWithLimit(r, target, MaxJSONRequestBodyBytes)
}

func DecodeJSONWithLimit(r *http.Request, target any, maxBytes int64) error {
	defer r.Body.Close()
	if maxBytes <= 0 {
		return fmt.Errorf("invalid json body limit")
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBytes+1))
	if err != nil {
		return err
	}
	if len(body) == 0 {
		return io.EOF
	}
	if int64(len(body)) > maxBytes {
		return fmt.Errorf("json body exceeds %d bytes", maxBytes)
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("json body contains multiple values")
		}
		return err
	}
	return nil
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
	case "deviceId":
		return "devices"
	case "networkId":
		return "networks"
	case "inviteId":
		return "invites"
	case "keyId":
		return "device-credentials"
	case "zoneId":
		return "zones"
	case "recordId":
		return "records"
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
	case "nodeId":
		return "nodes"
	case "relayNodeId":
		return "relay-nodes"
	case "punchNodeId":
		return "punch-nodes"
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
	if !strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return ""
	}
	return strings.TrimSpace(auth[7:])
}
