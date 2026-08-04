package repository

import (
	"context"

	"github.com/slan/service-biz/internal/model"
)

func (s *GormStore) ListCustomers(_ context.Context) ([]model.Customer, error) {
	return listModels(s.db.Order("customer_id asc"), func(row gormCustomerRecord) model.Customer {
		return row.model()
	})
}

func (s *GormStore) GetCustomer(_ context.Context, customerID string) (model.Customer, bool, error) {
	return firstModel(s.db.Where("customer_id = ?", customerID), func(row gormCustomerRecord) model.Customer {
		return row.model()
	})
}

func (s *GormStore) GetCustomerByEmail(_ context.Context, email string) (model.Customer, bool, error) {
	return firstModel(s.db.Where("email = ?", normalizeEmail(email)), func(row gormCustomerRecord) model.Customer {
		return row.model()
	})
}

func (s *GormStore) SaveCustomer(_ context.Context, customer model.Customer) error {
	row := customerRecordFromModel(customer)
	return upsertByColumns(s.db, &row, []string{"customer_id"}, []string{"email", "name", "country", "province", "city", "ip_region", "status", "created_at", "updated_at"})
}
