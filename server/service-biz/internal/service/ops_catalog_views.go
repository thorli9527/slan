package service

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/pkg/downloadkit"
)

func clientDownloadView(item model.ClientDownload) ClientDownloadView {
	return ClientDownloadView{
		DownloadID:   item.DownloadID,
		Name:         item.Name,
		Platform:     item.Platform,
		Version:      item.Version,
		URL:          item.URL,
		Status:       item.Status,
		CreatedAt:    item.CreatedAt,
		UpdatedAt:    item.UpdatedAt,
		PlatformName: item.Platform,
		Channel:      firstNonEmpty(item.Channel, "stable"),
		FileSize:     clientDownloadFileSize(item.URL),
		Arch:         item.Arch,
		SHA256:       item.SHA256,
		ReleaseNotes: item.ReleaseNotes,
	}
}

func planView(item model.Plan) PlanView {
	return PlanView{
		PlanCode:           item.PlanCode,
		Name:               item.Name,
		DeviceLimit:        item.DeviceLimit,
		Status:             item.Status,
		UpdatedAt:          item.UpdatedAt,
		InvitedDeviceLimit: item.InvitedDeviceLimit,
		TotalDeviceLimit:   item.TotalDeviceLimit,
		RelayMonthlyGB:     item.RelayMonthlyGB,
		RelayBandwidthMbps: item.RelayBandwidthMbps,
		RelayThrottleMbps:  item.RelayThrottleMbps,
		P2PUnlimited:       item.P2PUnlimited,
		CustomDomain:       item.CustomDomain,
		ACL:                item.ACL,
		DedicatedRelay:     item.DedicatedRelay,
		AuditLog:           item.AuditLog,
		APIAccess:          item.APIAccess,
		MonthlyPrice:       item.MonthlyPrice,
		YearlyPrice:        item.YearlyPrice,
	}
}

func productView(item model.Product) ProductView {
	return ProductView{
		ProductID:          item.ProductID,
		Name:               item.Name,
		PlanCode:           item.PlanCode,
		Price:              item.Price,
		Status:             item.Status,
		CreatedAt:          item.CreatedAt,
		UpdatedAt:          item.UpdatedAt,
		Type:               item.Type,
		Period:             item.Period,
		ValidDays:          item.ValidDays,
		RelayTrafficGB:     item.RelayTrafficGB,
		RelayBandwidthMbps: item.RelayBandwidthMbps,
		SalePrice:          item.SalePrice,
		Currency:           item.Currency,
		AutoRenew:          item.AutoRenew,
		Description:        item.Description,
	}
}

func orderView(item model.Order) OrderView {
	return OrderView{
		OrderID:         item.OrderID,
		CustomerID:      item.CustomerID,
		ProductID:       item.ProductID,
		Status:          item.Status,
		Amount:          item.Amount,
		CreatedAt:       item.CreatedAt,
		UpdatedAt:       item.UpdatedAt,
		CustomerEmail:   item.CustomerEmail,
		ProductName:     item.ProductName,
		ProductType:     item.ProductType,
		Currency:        item.Currency,
		PayStatus:       item.PayStatus,
		ProvisionStatus: item.ProvisionStatus,
		Channel:         item.Channel,
		PaidAt:          item.PaidAt,
		ValidUntil:      item.ValidUntil,
	}
}

func renewalView(item model.Renewal) RenewalView {
	return RenewalView{
		RenewalID:     item.RenewalID,
		OrderID:       item.OrderID,
		Status:        item.Status,
		RenewAt:       item.RenewAt,
		UpdatedAt:     item.UpdatedAt,
		CustomerEmail: item.CustomerEmail,
		PlanCode:      item.PlanCode,
		Period:        item.Period,
		Amount:        item.Amount,
		PaidAt:        item.PaidAt,
		Source:        item.Source,
		Operator:      item.Operator,
	}
}

func clientDownloadFileSize(rawURL string) int64 {
	fileName := filepath.Base(strings.TrimSpace(rawURL))
	if fileName == "." || fileName == "/" || fileName == "" {
		return 0
	}
	info, err := os.Stat(downloadkit.ClientDownloadPath(fileName))
	if err != nil {
		return 0
	}
	return info.Size()
}
