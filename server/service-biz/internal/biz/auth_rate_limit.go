package biz

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"
)

func (s *Store) LoginUserWithRateLimit(email, password, remoteIP string) (AuthResponse, error) {
	key := loginRateLimitKey("user", email, remoteIP)
	if err := s.checkLoginRateLimit(key, time.Now().Unix()); err != nil {
		s.RecordAuditEvent(loginAuditEvent("user", "", email, "auth.login", "rate_limited", remoteIP, err))
		return AuthResponse{}, err
	}
	auth, err := s.LoginUser(email, password)
	recordErr := s.recordLoginAttempt(key, err == nil, time.Now().Unix())
	actorID := ""
	if err == nil {
		actorID = auth.User.UserID
	}
	if recordErr != nil {
		s.RecordAuditEvent(loginAuditEvent("user", actorID, email, "auth.login", "failed", remoteIP, recordErr))
		return AuthResponse{}, recordErr
	}
	s.RecordAuditEvent(loginAuditEvent("user", actorID, email, "auth.login", auditStatusFromError(err), remoteIP, err))
	return auth, err
}

func (s *Store) LoginOperatorWithRateLimit(email, password, remoteIP string) (OperatorAuthResponse, error) {
	key := loginRateLimitKey("ops", email, remoteIP)
	if err := s.checkLoginRateLimit(key, time.Now().Unix()); err != nil {
		s.RecordAuditEvent(loginAuditEvent("operator", "", email, "ops.auth.login", "rate_limited", remoteIP, err))
		return OperatorAuthResponse{}, err
	}
	auth, err := s.LoginOperator(email, password)
	recordErr := s.recordLoginAttempt(key, err == nil, time.Now().Unix())
	actorID := ""
	if err == nil {
		actorID = auth.Operator.OperatorID
	}
	if recordErr != nil {
		s.RecordAuditEvent(loginAuditEvent("operator", actorID, email, "ops.auth.login", "failed", remoteIP, recordErr))
		return OperatorAuthResponse{}, recordErr
	}
	s.RecordAuditEvent(loginAuditEvent("operator", actorID, email, "ops.auth.login", auditStatusFromError(err), remoteIP, err))
	return auth, err
}

func (s *Store) CheckDeviceLoginPrepareRateLimit(deviceID, remoteIP string) error {
	key := loginRateLimitKey("device-login-prepare", deviceID, remoteIP)
	now := time.Now().Unix()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db != nil {
		failure, err := s.incrementPostgresLoginFailureLocked(context.Background(), key, now, prepareRateLimitMax)
		if err != nil {
			return err
		}
		s.loginFailures[key] = failure
		if failure.BlockedUntil > now {
			return errRateLimited
		}
		return nil
	}
	failure, ok := s.loginFailures[key]
	if ok && failure.BlockedUntil > now {
		return errRateLimited
	}
	if !ok || now-failure.FirstFailedAt > int64(loginRateLimitWindow.Seconds()) {
		failure = LoginFailure{Key: key, FirstFailedAt: now}
	}
	failure.FailedCount++
	failure.LastFailedAt = now
	if failure.FailedCount > prepareRateLimitMax {
		failure.BlockedUntil = now + int64(loginRateLimitBlock.Seconds())
	}
	s.loginFailures[key] = failure
	if err := s.persistPostgresLoginFailureLocked(context.Background(), failure); err != nil {
		return err
	}
	if failure.BlockedUntil > now {
		return errRateLimited
	}
	return nil
}

func (s *Store) checkLoginRateLimit(key string, now int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db != nil {
		failure, ok, err := s.loadPostgresLoginFailureByKeyLocked(context.Background(), key)
		if err != nil {
			return err
		}
		if !ok {
			delete(s.loginFailures, key)
			return nil
		}
		s.loginFailures[key] = failure
		if failure.BlockedUntil > now {
			return errRateLimited
		}
		if now-failure.FirstFailedAt > int64(loginRateLimitWindow.Seconds()) {
			delete(s.loginFailures, key)
			return s.deletePostgresLoginFailureLocked(context.Background(), key)
		}
		return nil
	}
	failure, ok := s.loginFailures[key]
	if !ok {
		return nil
	}
	if failure.BlockedUntil > now {
		return errRateLimited
	}
	if now-failure.FirstFailedAt > int64(loginRateLimitWindow.Seconds()) {
		delete(s.loginFailures, key)
		if err := s.deletePostgresLoginFailureLocked(context.Background(), key); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) recordLoginAttempt(key string, succeeded bool, now int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if succeeded {
		delete(s.loginFailures, key)
		return s.deletePostgresLoginFailureLocked(context.Background(), key)
	}
	if s.db != nil {
		failure, err := s.incrementPostgresLoginFailureLocked(context.Background(), key, now, loginRateLimitMaxFail-1)
		if err != nil {
			return err
		}
		s.loginFailures[key] = failure
		return nil
	}
	failure := s.loginFailures[key]
	if failure.Key == "" || now-failure.FirstFailedAt > int64(loginRateLimitWindow.Seconds()) {
		failure = LoginFailure{Key: key, FirstFailedAt: now}
	}
	failure.FailedCount++
	failure.LastFailedAt = now
	if failure.FailedCount >= loginRateLimitMaxFail {
		failure.BlockedUntil = now + int64(loginRateLimitBlock.Seconds())
	}
	s.loginFailures[key] = failure
	return s.persistPostgresLoginFailureLocked(context.Background(), failure)
}

func loginRateLimitKey(scope, email, remoteIP string) string {
	email = strings.ToLower(strings.TrimSpace(email))
	remoteIP = strings.TrimSpace(remoteIP)
	if remoteIP == "" {
		remoteIP = "unknown"
	}
	return strings.TrimSpace(scope) + "|" + email + "|" + remoteIP
}

func loginAuditEvent(actorType, actorID, email, action, status, remoteIP string, err error) AuditEvent {
	details := map[string]string{}
	if err != nil {
		details["error"] = err.Error()
	}
	return AuditEvent{
		ActorType:    actorType,
		ActorID:      actorID,
		ActorEmail:   email,
		Action:       action,
		ResourceType: "auth",
		Status:       status,
		RemoteIP:     remoteIP,
		Details:      details,
	}
}

func auditStatusFromError(err error) string {
	if err == nil {
		return "succeeded"
	}
	return "failed"
}

func clientIPFromRequest(r *http.Request) string {
	for _, header := range []string{"X-Forwarded-For", "X-Real-IP"} {
		value := strings.TrimSpace(r.Header.Get(header))
		if value == "" {
			continue
		}
		if header == "X-Forwarded-For" {
			value = strings.TrimSpace(strings.Split(value, ",")[0])
		}
		if ip := net.ParseIP(value); ip != nil {
			return ip.String()
		}
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil {
		if ip := net.ParseIP(host); ip != nil {
			return ip.String()
		}
	}
	if ip := net.ParseIP(strings.TrimSpace(r.RemoteAddr)); ip != nil {
		return ip.String()
	}
	return ""
}
