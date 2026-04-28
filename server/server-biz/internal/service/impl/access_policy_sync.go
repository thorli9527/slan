package impl

import (
	"context"
	"time"
)

const accessPolicySyncInterval = 30 * time.Second

func (s *dbState) startAccessPolicySyncLoop() {
	go func() {
		ticker := time.NewTicker(accessPolicySyncInterval)
		defer ticker.Stop()
		for {
			s.enforceFixedAccessPolicy(context.Background())
			<-ticker.C
		}
	}()
}

func (s *dbState) enforceFixedAccessPolicy(ctx context.Context) {
	users, err := s.pg.ListUsers(ctx)
	if err != nil {
		return
	}
	for _, user := range users {
		s.enforceUserDeviceLimit(ctx, user.UserID, fixedDeviceLimit())
	}
}

func (s *dbState) enforceUserDeviceLimit(ctx context.Context, userID string, limit int) {
	if limit <= 0 {
		return
	}
	attachments, err := s.pg.ListActiveAttachmentsByUser(ctx, userID)
	if err != nil || len(attachments) <= limit {
		return
	}
	for _, attachment := range attachments[limit:] {
		if err := s.pg.SuspendAttachment(ctx, attachment.AttachmentID); err != nil {
			continue
		}
		s.publishDeviceIPReassigned(attachment.NetworkID, attachment.DeviceID, attachment.AttachmentID, "")
	}
}
