package repo

import (
	"context"
	"testing"

	"github.com/slan/server/server-biz/api/dto"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestIsUniqueViolationRecognizesSQLiteConstraint(t *testing.T) {
	ctx := context.Background()
	db, err := gorm.Open(sqlite.New(sqlite.Config{
		DriverName: "sqlite",
		DSN:        "file::memory:?cache=shared",
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.WithContext(ctx).AutoMigrate(&SubnetAttachment{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	pg := NewPostgresRepository(db)
	first := dto.SubnetAttachment{
		AttachmentID: "att-1",
		NetworkID:    "net-1",
		SubnetID:     "subnet-1",
		DeviceID:     "device-1",
		VirtualIP:    "100.96.0.2",
		Status:       "active",
	}
	duplicate := dto.SubnetAttachment{
		AttachmentID: "att-2",
		NetworkID:    "net-1",
		SubnetID:     "subnet-1",
		DeviceID:     "device-2",
		VirtualIP:    "100.96.0.2",
		Status:       "active",
	}
	if err := pg.CreateAttachment(ctx, first); err != nil {
		t.Fatalf("create first attachment: %v", err)
	}
	if err := pg.CreateAttachment(ctx, duplicate); !IsUniqueViolation(err) {
		t.Fatalf("expected unique violation, got %v", err)
	}
}
