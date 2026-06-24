package repository

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func listModels[R any, M any](query *gorm.DB, mapper func(R) M) ([]M, error) {
	var rows []R
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]M, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapper(row))
	}
	return items, nil
}

func firstModel[R any, M any](query *gorm.DB, mapper func(R) M) (M, bool, error) {
	var row R
	if err := query.First(&row).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			var zero M
			return zero, false, nil
		}
		var zero M
		return zero, false, err
	}
	return mapper(row), true, nil
}

func upsertByColumns(db *gorm.DB, value any, keyColumns []string, updateColumns []string) error {
	columns := make([]clause.Column, 0, len(keyColumns))
	for _, name := range keyColumns {
		columns = append(columns, clause.Column{Name: name})
	}
	return db.Clauses(clause.OnConflict{
		Columns:   columns,
		DoUpdates: clause.AssignmentColumns(updateColumns),
	}).Create(value).Error
}
