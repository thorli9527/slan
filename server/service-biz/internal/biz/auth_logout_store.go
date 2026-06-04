package biz

import (
	"context"
	"strings"
)

func (s *Store) LogoutSessions(accessToken, deviceToken string) error {
	accessToken = strings.TrimSpace(accessToken)
	deviceToken = strings.TrimSpace(deviceToken)
	if accessToken == "" && deviceToken == "" {
		return errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	logoutUserID := ""
	if accessToken != "" {
		for sessionID, session := range s.sessions {
			if session.Token == accessToken {
				logoutUserID = session.UserID
				delete(s.sessions, sessionID)
			}
		}
	}
	if logoutUserID != "" {
		for sessionID, session := range s.deviceSessions {
			if session.UserID != logoutUserID || session.State != "active" {
				continue
			}
			session.State = "revoked"
			s.deviceSessions[sessionID] = session
			delete(s.deviceSessionByToken, session.DeviceToken)
		}
	}
	if deviceToken != "" {
		if sessionID, ok := s.deviceSessionByToken[deviceToken]; ok {
			if session, ok := s.deviceSessions[sessionID]; ok {
				session.State = "revoked"
				s.deviceSessions[sessionID] = session
			}
			delete(s.deviceSessionByToken, deviceToken)
		}
	}
	if err := s.persistPostgresLogoutTxLocked(ctx, postgresTx, accessToken, deviceToken, logoutUserID); err != nil {
		return err
	}
	postgresTx = nil
	return nil
}
