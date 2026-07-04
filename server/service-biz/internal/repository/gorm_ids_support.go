package repository

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"gorm.io/gorm"
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
	err := s.db.Transaction(func(db *gorm.DB) error {
		var counter gormCounter
		result := db.Where("name = ?", name).First(&counter)
		if result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				counter = gormCounter{Name: name, Value: 1}
				value = counter.Value
				return db.Create(&counter).Error
			}
			return result.Error
		}
		counter.Value += 1
		value = counter.Value
		return db.Save(&counter).Error
	})
	if err != nil {
		return 0
	}
	return value
}
