package repository

import (
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *GormStore) nextID(name, prefix string) string {
	var result string
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var counter gormCounter
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&counter, "name = ?", name).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				counter = gormCounter{Name: name, Value: 1}
				if err := tx.Create(&counter).Error; err != nil {
					return err
				}
			} else {
				return err
			}
		} else {
			counter.Value++
			if err := tx.Save(&counter).Error; err != nil {
				return err
			}
		}
		result = fmt.Sprintf("%s-%06d", prefix, counter.Value)
		return nil
	})
	if err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return result
}
