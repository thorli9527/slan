package repository

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

func (s *GormStore) nextID(name, prefix string) string {
	_ = s
	_ = name
	value, err := randomIDHex()
	if err != nil {
		return fmt.Sprintf("%s%x", prefix, time.Now().UnixNano())
	}
	return prefix + value
}

func randomIDHex() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

func (s *GormStore) nextCounterValue(name string) int64 {
	if s == nil || s.db == nil || name == "" {
		return 0
	}
	var value int64
	err := s.db.Raw(`
		INSERT INTO gorm_counters (name, value) VALUES (?, 1)
		ON CONFLICT (name) DO UPDATE SET value = gorm_counters.value + 1
		RETURNING value
	`, name).Scan(&value).Error
	if err != nil {
		return 0
	}
	return value
}
