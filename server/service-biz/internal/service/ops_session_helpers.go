package service

import (
	"time"

	"github.com/slan/service-biz/internal/model"
)

func newOpsSessionID(next func(string) string) string {
	return scopedID(next, "opsess")
}

func newOperatorSession(now time.Time, next func(string) string, operatorID string) (model.OperatorSession, error) {
	token, err := randomHex(24)
	if err != nil {
		return model.OperatorSession{}, err
	}
	return model.OperatorSession{
		SessionID:   newOpsSessionID(next),
		OperatorID:  operatorID,
		AccessToken: token,
		ExpiresAt:   now.Add(24 * time.Hour).Unix(),
		CreatedAt:   now.Unix(),
	}, nil
}
