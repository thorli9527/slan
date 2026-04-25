package configs

import (
	"context"
	"testing"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestMigrateSubnetAttachmentIndexes_AllowsReuseAcrossSubnets(t *testing.T) {
	ctx := context.Background()
	db, err := gorm.Open(sqlite.New(sqlite.Config{
		DriverName: "sqlite",
		DSN:        "file::memory:?cache=shared",
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	if err := db.WithContext(ctx).AutoMigrate(&repo.SubnetAttachment{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	if err := migrateSubnetAttachmentIndexes(ctx, db); err != nil {
		t.Fatalf("migrate indexes: %v", err)
	}

	pg := repo.NewPostgresRepository(db)
	first := dto.SubnetAttachment{
		AttachmentID: "att-1",
		NetworkID:    "net-1",
		SubnetID:     "subnet-1",
		DeviceID:     "device-1",
		VirtualIP:    "100.96.0.2",
		Status:       "active",
	}
	secondSubnet := dto.SubnetAttachment{
		AttachmentID: "att-2",
		NetworkID:    "net-2",
		SubnetID:     "subnet-2",
		DeviceID:     "device-2",
		VirtualIP:    "100.96.0.2",
		Status:       "active",
	}
	duplicateInSubnet := dto.SubnetAttachment{
		AttachmentID: "att-3",
		NetworkID:    "net-1",
		SubnetID:     "subnet-1",
		DeviceID:     "device-3",
		VirtualIP:    "100.96.0.2",
		Status:       "active",
	}

	if err := pg.CreateAttachment(ctx, first); err != nil {
		t.Fatalf("create first attachment: %v", err)
	}
	if err := pg.CreateAttachment(ctx, secondSubnet); err != nil {
		t.Fatalf("create attachment in second subnet: %v", err)
	}
	if err := pg.CreateAttachment(ctx, duplicateInSubnet); err == nil {
		t.Fatal("expected duplicate virtual ip in same subnet to fail")
	}

	releasedFirst := dto.SubnetAttachment{
		AttachmentID: "att-4",
		NetworkID:    "net-1",
		SubnetID:     "subnet-1",
		DeviceID:     "device-4",
		VirtualIP:    "",
		Status:       "active",
	}
	releasedSecond := dto.SubnetAttachment{
		AttachmentID: "att-5",
		NetworkID:    "net-1",
		SubnetID:     "subnet-1",
		DeviceID:     "device-5",
		VirtualIP:    "",
		Status:       "active",
	}
	if err := pg.CreateAttachment(ctx, releasedFirst); err != nil {
		t.Fatalf("create first empty attachment: %v", err)
	}
	if err := pg.CreateAttachment(ctx, releasedSecond); err != nil {
		t.Fatalf("create second empty attachment: %v", err)
	}
}
