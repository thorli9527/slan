package repository

import (
	"context"

	"github.com/slan/service-biz/internal/model"
)

func (s *GormStore) GetCustomerPlan(_ context.Context, customerID string) (string, bool, error) {
	return firstModel(s.db.Where("customer_id = ?", customerID), func(row gormCustomerPlanRecord) string {
		return row.PlanCode
	})
}

func (s *GormStore) SaveCustomerPlan(_ context.Context, customerID, planCode string) error {
	row := gormCustomerPlanRecord{CustomerID: customerID, PlanCode: planCode}
	return upsertByColumns(s.db, &row, []string{"customer_id"}, []string{"plan_code"})
}

func (s *GormStore) ListClientDownloads(_ context.Context) ([]model.ClientDownload, error) {
	return listModels(s.db.Order("download_id asc"), func(row gormClientDownloadRecord) model.ClientDownload {
		return row.model()
	})
}

func (s *GormStore) GetClientDownload(_ context.Context, downloadID string) (model.ClientDownload, bool, error) {
	return firstModel(s.db.Where("download_id = ?", downloadID), func(row gormClientDownloadRecord) model.ClientDownload {
		return row.model()
	})
}

func (s *GormStore) SaveClientDownload(_ context.Context, item model.ClientDownload) error {
	row := clientDownloadRecordFromModel(item)
	return upsertByColumns(s.db, &row, []string{"download_id"}, []string{"name", "platform", "version", "arch", "channel", "url", "sha256", "release_notes", "status", "created_at", "updated_at"})
}

func (s *GormStore) DeleteClientDownload(_ context.Context, downloadID string) error {
	return s.db.Delete(&gormClientDownloadRecord{}, "download_id = ?", downloadID).Error
}

func (s *GormStore) ListPlans(_ context.Context) ([]model.Plan, error) {
	return listModels(s.db.Order("plan_code asc"), func(row gormPlanRecord) model.Plan {
		return row.model()
	})
}

func (s *GormStore) GetPlan(_ context.Context, planCode string) (model.Plan, bool, error) {
	return firstModel(s.db.Where("plan_code = ?", planCode), func(row gormPlanRecord) model.Plan {
		return row.model()
	})
}

func (s *GormStore) SavePlan(_ context.Context, item model.Plan) error {
	row := planRecordFromModel(item)
	return upsertByColumns(s.db, &row, []string{"plan_code"}, []string{"name", "device_limit", "invited_device_limit", "total_device_limit", "relay_monthly_gb", "relay_bandwidth_mbps", "relay_throttle_mbps", "p2_p_unlimited", "custom_domain", "acl", "dedicated_relay", "audit_log", "api_access", "monthly_price", "yearly_price", "status", "updated_at"})
}

func (s *GormStore) ListProducts(_ context.Context) ([]model.Product, error) {
	return listModels(s.db.Order("product_id asc"), func(row gormProductRecord) model.Product {
		return row.model()
	})
}

func (s *GormStore) GetProduct(_ context.Context, productID string) (model.Product, bool, error) {
	return firstModel(s.db.Where("product_id = ?", productID), func(row gormProductRecord) model.Product {
		return row.model()
	})
}

func (s *GormStore) SaveProduct(_ context.Context, item model.Product) error {
	row := productRecordFromModel(item)
	return upsertByColumns(s.db, &row, []string{"product_id"}, []string{"name", "type", "plan_code", "period", "valid_days", "relay_traffic_gb", "relay_bandwidth_mbps", "price", "sale_price", "currency", "auto_renew", "status", "description", "created_at", "updated_at"})
}

func (s *GormStore) ListOrders(_ context.Context) ([]model.Order, error) {
	return listModels(s.db.Order("order_id asc"), func(row gormOrderRecord) model.Order {
		return row.model()
	})
}

func (s *GormStore) GetOrder(_ context.Context, orderID string) (model.Order, bool, error) {
	return firstModel(s.db.Where("order_id = ?", orderID), func(row gormOrderRecord) model.Order {
		return row.model()
	})
}

func (s *GormStore) SaveOrder(_ context.Context, item model.Order) error {
	row := orderRecordFromModel(item)
	return upsertByColumns(s.db, &row, []string{"order_id"}, []string{"customer_id", "customer_email", "product_id", "product_name", "product_type", "status", "amount", "currency", "pay_status", "provision_status", "channel", "paid_at", "valid_until", "created_at", "updated_at"})
}

func (s *GormStore) ListRenewals(_ context.Context) ([]model.Renewal, error) {
	return listModels(s.db.Order("renewal_id asc"), func(row gormRenewalRecord) model.Renewal {
		return row.model()
	})
}

func (s *GormStore) GetRenewal(_ context.Context, renewalID string) (model.Renewal, bool, error) {
	return firstModel(s.db.Where("renewal_id = ?", renewalID), func(row gormRenewalRecord) model.Renewal {
		return row.model()
	})
}

func (s *GormStore) SaveRenewal(_ context.Context, item model.Renewal) error {
	row := renewalRecordFromModel(item)
	return upsertByColumns(s.db, &row, []string{"renewal_id"}, []string{"order_id", "customer_id", "customer_email", "plan_code", "period", "amount", "status", "renew_at", "paid_at", "source", "operator", "updated_at"})
}

func (s *GormStore) DeleteRenewal(_ context.Context, renewalID string) error {
	return s.db.Delete(&gormRenewalRecord{}, "renewal_id = ?", renewalID).Error
}
