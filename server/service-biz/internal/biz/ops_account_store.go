package biz

import (
	"fmt"
	"strings"
	"time"
)

const (
	defaultOpsAdminEmail    = "admin1"
	defaultOpsAdminPassword = "admin1"
)

func (s *Store) seedOpsDefaultsLocked(now int64) {
	if len(s.operators) == 0 {
		operator := OperatorUser{
			OperatorID:   "op-000001",
			Name:         "超级管理员",
			Email:        defaultOpsAdminEmail,
			Role:         "super_admin",
			Status:       "active",
			PasswordHash: hashPassword(defaultOpsAdminPassword),
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		s.operators[operator.OperatorID] = operator
		s.operatorByEmail[operator.Email] = operator.OperatorID
		s.nextOperatorSeq = 2
	}
	if len(s.opsPlans) == 0 {
		for _, plan := range []OpsPlan{
			{Code: "free", Name: "免费版", OwnDeviceLimit: 5, InvitedDeviceLimit: 5, TotalDeviceLimit: 10, RelayMonthlyGB: 50, RelayBandwidthMbps: 5, RelayThrottleMbps: 5, P2PUnlimited: true, Status: "active"},
			{Code: "pro", Name: "专业版", OwnDeviceLimit: 30, InvitedDeviceLimit: 100, TotalDeviceLimit: 130, RelayMonthlyGB: 1024, RelayBandwidthMbps: 100, RelayThrottleMbps: 100, P2PUnlimited: true, CustomDomain: true, ACL: true, MonthlyPrice: 39, YearlyPrice: 299, Status: "active"},
			{Code: "enterprise", Name: "企业版", OwnDeviceLimit: 500, InvitedDeviceLimit: 500, TotalDeviceLimit: 1000, RelayMonthlyGB: 10240, RelayBandwidthMbps: 1000, RelayThrottleMbps: 1000, P2PUnlimited: true, CustomDomain: true, ACL: true, DedicatedRelay: true, AuditLog: true, APIAccess: true, YearlyPrice: 2999, Status: "active"},
		} {
			s.opsPlans[plan.Code] = plan
		}
	}
	if len(s.products) == 0 {
		s.addProductLocked(Product{Name: "专业版月付", Type: "plan", PlanCode: "pro", Period: "monthly", ValidDays: 31, RelayTrafficGB: 1024, RelayBandwidthMbps: 100, ListPrice: 39, SalePrice: 39, Currency: "CNY", AutoRenew: true, Status: "active", Description: "专业版 1 个月"})
		s.addProductLocked(Product{Name: "专业版年付", Type: "plan", PlanCode: "pro", Period: "yearly", ValidDays: 365, RelayTrafficGB: 1024, RelayBandwidthMbps: 100, ListPrice: 468, SalePrice: 299, Currency: "CNY", AutoRenew: true, Status: "active", Description: "专业版 1 年"})
		s.addProductLocked(Product{Name: "企业版年付", Type: "plan", PlanCode: "enterprise", Period: "yearly", ValidDays: 365, RelayTrafficGB: 10240, RelayBandwidthMbps: 1000, ListPrice: 2999, SalePrice: 2999, Currency: "CNY", AutoRenew: false, Status: "active", Description: "企业版基础年付"})
	}
	if len(s.relayNodes) == 0 {
		if node, ok := defaultOpsRelayNode(); ok {
			s.addRelayNodeLocked(node)
		}
	}
	if len(s.punchNodes) == 0 {
		for _, node := range configuredPunchNodes() {
			s.addPunchNodeLocked(node)
		}
	}
}

func defaultOpsRelayNode() (OpsRelayNode, bool) {
	for _, candidate := range configuredRelayCandidates() {
		if candidate.Transport != "udp" || strings.TrimSpace(candidate.Address) == "" {
			continue
		}
		return OpsRelayNode{Name: "默认 UDP Relay", Region: defaultString(candidate.RegionID, "local"), Transport: "relay_udp", PublicAddr: relayCandidatePublicAddress(candidate.Address), MaxBandwidthMbps: 1000, MonthlyTrafficGB: 10240, MaxSessions: 10000, Status: "active", Health: "healthy"}, true
	}
	return OpsRelayNode{}, false
}

func (s *Store) LoginOperator(email, password string) (OperatorAuthResponse, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || strings.TrimSpace(password) == "" {
		return OperatorAuthResponse{}, errBadRequest
	}
	token, err := secureTokenHex(32)
	if err != nil {
		return OperatorAuthResponse{}, err
	}
	sessionID, err := secureTokenHex(12)
	if err != nil {
		return OperatorAuthResponse{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	operatorID, ok := s.operatorByEmail[email]
	if !ok {
		return OperatorAuthResponse{}, errNotFound
	}
	operator := s.operators[operatorID]
	if operator.Status != "active" || !verifyPasswordHash(operator.PasswordHash, password) {
		return OperatorAuthResponse{}, errBadRequest
	}
	now := time.Now().Unix()
	operator.LastLoginAt = now
	operator.UpdatedAt = now
	s.operators[operator.OperatorID] = operator
	session := OperatorSession{SessionID: "opsess-" + sessionID, OperatorID: operator.OperatorID, Token: token, CreatedAt: now, ExpiresAt: now + int64((24 * time.Hour).Seconds())}
	s.operatorSessions[session.SessionID] = session
	return OperatorAuthResponse{Operator: operator, Session: session}, nil
}

func (s *Store) OperatorByToken(token string) (OperatorUser, error) {
	token = strings.TrimSpace(strings.TrimPrefix(token, "Bearer "))
	if token == "" {
		return OperatorUser{}, errUnauthorized
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().Unix()
	for _, session := range s.operatorSessions {
		if session.Token != token {
			continue
		}
		if session.ExpiresAt < now {
			return OperatorUser{}, errUnauthorized
		}
		operator, ok := s.operators[session.OperatorID]
		if !ok || operator.Status != "active" {
			return OperatorUser{}, errUnauthorized
		}
		return operator, nil
	}
	return OperatorUser{}, errUnauthorized
}

func (s *Store) ChangeOwnOperatorPassword(operatorID, oldPassword, newPassword string) error {
	if strings.TrimSpace(operatorID) == "" || strings.TrimSpace(oldPassword) == "" || strings.TrimSpace(newPassword) == "" {
		return errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	operator, ok := s.operators[operatorID]
	if !ok {
		return errNotFound
	}
	if !verifyPasswordHash(operator.PasswordHash, oldPassword) {
		return errBadRequest
	}
	operator.PasswordHash = hashPassword(newPassword)
	operator.UpdatedAt = time.Now().Unix()
	s.operators[operator.OperatorID] = operator
	return nil
}

func (s *Store) ListOperators() []OperatorUser {
	s.mu.Lock()
	defer s.mu.Unlock()
	return sortedValues(s.operators, func(a, b OperatorUser) bool { return a.Email < b.Email })
}

func (s *Store) UpsertOperator(operator OperatorUser) (OperatorUser, error) {
	operator.Email = strings.ToLower(strings.TrimSpace(operator.Email))
	operator.Name = strings.TrimSpace(operator.Name)
	operator.Role = defaultString(operator.Role, "operator")
	operator.Status = defaultString(operator.Status, "active")
	if operator.Email == "" || operator.Name == "" {
		return OperatorUser{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().Unix()
	if strings.TrimSpace(operator.OperatorID) == "" {
		if _, exists := s.operatorByEmail[operator.Email]; exists {
			return OperatorUser{}, errConflict
		}
		operator.OperatorID = fmt.Sprintf("op-%06d", s.nextOperatorSeq)
		s.nextOperatorSeq++
		operator.PasswordHash = hashPassword(defaultString(operator.PasswordHash, defaultOpsAdminPassword))
		operator.CreatedAt = now
	} else {
		existing, ok := s.operators[operator.OperatorID]
		if !ok {
			return OperatorUser{}, errNotFound
		}
		if existing.Email != operator.Email {
			if _, exists := s.operatorByEmail[operator.Email]; exists {
				return OperatorUser{}, errConflict
			}
			delete(s.operatorByEmail, existing.Email)
		}
		operator.PasswordHash = existing.PasswordHash
		operator.CreatedAt = existing.CreatedAt
		operator.LastLoginAt = existing.LastLoginAt
	}
	operator.UpdatedAt = now
	s.operators[operator.OperatorID] = operator
	s.operatorByEmail[operator.Email] = operator.OperatorID
	return operator, nil
}

func (s *Store) SetOperatorPassword(operatorID, newPassword string) error {
	operatorID = strings.TrimSpace(operatorID)
	newPassword = strings.TrimSpace(newPassword)
	if operatorID == "" || newPassword == "" {
		return errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	operator, ok := s.operators[operatorID]
	if !ok {
		return errNotFound
	}
	operator.PasswordHash = hashPassword(newPassword)
	operator.UpdatedAt = time.Now().Unix()
	s.operators[operatorID] = operator
	return nil
}
