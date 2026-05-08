package impl

import (
	"context"
	"testing"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestUpsertRelayPolicyExecutionFromNetworkState(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&repo.RelayPolicyExecution{}); err != nil {
		t.Fatalf("migrate relay policy execution: %v", err)
	}
	state := &dbState{pg: repo.NewPostgresRepository(db)}
	ctx := context.Background()

	if err := state.pg.UpsertRelayPolicyExecution(ctx, repo.RelayPolicyExecution{
		ExecutionID:  "relay-exec-existing",
		NetworkID:    "net-1",
		DeviceID:     "device-1",
		PolicyID:     "policy-1",
		TemplateID:   "template-1",
		TemplateName: "稳定优先",
		Applied:      false,
		UpdatedAt:    100,
	}); err != nil {
		t.Fatalf("seed relay policy execution: %v", err)
	}

	err = state.upsertRelayPolicyExecutionFromNetworkState(ctx, "device-1", "net-1", dto.DeviceNetworkStateRequest{
		ReportedAt:             1780000000,
		RelayPolicyID:          "policy-1",
		RelayPolicyScope:       "device",
		RelayPolicyApplied:     true,
		RelayPolicyUpdatedAtMs: 1780000000123,
		RelayPolicyAppliedAtMs: 1780000000456,
		RelayMtu:               1280,
		MaxFramePayload:        1200,
		RelayPolicyReason:      "template_stable_client_quality",
	})
	if err != nil {
		t.Fatalf("upsert relay policy execution: %v", err)
	}

	record, err := state.pg.GetRelayPolicyExecution(ctx, "net-1", "device-1", "policy-1")
	if err != nil {
		t.Fatalf("get relay policy execution: %v", err)
	}
	if !record.Applied {
		t.Fatalf("expected policy to be marked applied")
	}
	if record.TemplateID != "template-1" || record.TemplateName != "稳定优先" {
		t.Fatalf("expected template metadata to be preserved, got id=%q name=%q", record.TemplateID, record.TemplateName)
	}
	if record.Scope != "device" || record.RelayMtu != 1280 || record.MaxFramePayload != 1200 {
		t.Fatalf("unexpected policy fields: scope=%q mtu=%d payload=%d", record.Scope, record.RelayMtu, record.MaxFramePayload)
	}
	if record.PolicyUpdatedAtMs != 1780000000123 || record.ReportedAtMs != 1780000000456 {
		t.Fatalf("unexpected timestamps: updated=%d reported=%d", record.PolicyUpdatedAtMs, record.ReportedAtMs)
	}
}

func TestDeleteRelayPolicyExecutionsClearsOnlyTargetDevices(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&repo.RelayPolicyExecution{}); err != nil {
		t.Fatalf("migrate relay policy execution: %v", err)
	}
	pg := repo.NewPostgresRepository(db)
	ctx := context.Background()
	for _, record := range []repo.RelayPolicyExecution{
		{ExecutionID: "old-a", NetworkID: "net-1", DeviceID: "device-a", PolicyID: "policy-old-a", UpdatedAt: 100},
		{ExecutionID: "old-b", NetworkID: "net-1", DeviceID: "device-b", PolicyID: "policy-old-b", UpdatedAt: 100},
		{ExecutionID: "old-c", NetworkID: "net-2", DeviceID: "device-a", PolicyID: "policy-old-c", UpdatedAt: 100},
	} {
		if err := pg.UpsertRelayPolicyExecution(ctx, record); err != nil {
			t.Fatalf("seed relay policy execution %s: %v", record.ExecutionID, err)
		}
	}

	if err := pg.DeleteRelayPolicyExecutions(ctx, "net-1", []string{"device-a", "device-a", ""}); err != nil {
		t.Fatalf("delete relay policy executions: %v", err)
	}

	var remaining []repo.RelayPolicyExecution
	if err := db.Order("network_id, device_id").Find(&remaining).Error; err != nil {
		t.Fatalf("list relay policy executions: %v", err)
	}
	if len(remaining) != 2 {
		t.Fatalf("expected 2 remaining records, got %d: %+v", len(remaining), remaining)
	}
	if remaining[0].NetworkID != "net-1" || remaining[0].DeviceID != "device-b" {
		t.Fatalf("expected net-1/device-b to remain, got %+v", remaining[0])
	}
	if remaining[1].NetworkID != "net-2" || remaining[1].DeviceID != "device-a" {
		t.Fatalf("expected net-2/device-a to remain, got %+v", remaining[1])
	}
}

func TestPolicyOnlyQualityItemIncludesPolicyStatus(t *testing.T) {
	item := policyOnlyQualityItem(
		repo.RelayPolicyExecution{
			ExecutionID:       "relay-exec-1",
			NetworkID:         "net-1",
			DeviceID:          "device-1",
			NodeID:            "node-1",
			PolicyID:          "policy-1",
			TemplateID:        "template-1",
			TemplateName:      "稳定优先",
			Scope:             "device",
			RelayMtu:          1280,
			MaxFramePayload:   1200,
			Applied:           true,
			PolicyUpdatedAtMs: 1780000000123,
			ReportedAtMs:      1780000000456,
			UpdatedAt:         1780000000,
		},
		"主网络",
		map[string]repo.Device{
			"device-1": {DeviceID: "device-1", UserID: "user-1", Name: "MacBook"},
		},
		map[string]string{"user-1": "user@example.com"},
	)

	if item.DeviceID != "device-1" || item.DeviceName != "MacBook" || item.UserEmail != "user@example.com" {
		t.Fatalf("unexpected device fields: %+v", item)
	}
	if item.PolicyID != "policy-1" || item.PolicyTemplateName != "稳定优先" || !item.PolicyApplied {
		t.Fatalf("unexpected policy fields: %+v", item)
	}
	if item.RelayMtu != 1280 || item.MaxFramePayload != 1200 || item.PolicyReportedAtMs != 1780000000456 {
		t.Fatalf("unexpected relay fields: %+v", item)
	}
}
