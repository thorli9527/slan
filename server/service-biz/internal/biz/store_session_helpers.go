package biz

import (
	"fmt"
)

func (s *Store) createSessionLocked(userID string, now int64) UserSession {
	sessionSeq := s.nextUserSessionSeq
	s.nextUserSessionSeq++
	session := UserSession{
		SessionID: fmt.Sprintf("session-%06d", sessionSeq),
		UserID:    userID,
		Token:     fmt.Sprintf("token-%s-%d-%06d", userID, now, sessionSeq),
		CreatedAt: now,
		ExpiresAt: now + int64(userSessionTTL.Seconds()),
	}
	s.sessions[session.Token] = session
	return session
}
