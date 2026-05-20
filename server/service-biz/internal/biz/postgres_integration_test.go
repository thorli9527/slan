package biz

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"
)

func TestPostgresMigrationIntegration(t *testing.T) {
	dsn := os.Getenv("SLAN_BIZ_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("SLAN_BIZ_POSTGRES_TEST_DSN is not set")
	}
	t.Setenv("SLAN_BIZ_POSTGRES_DSN", dsn)
	t.Setenv("SLAN_BIZ_MIGRATIONS_DIR", "../../migrations")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := InitPostgresFromEnv(ctx)
	if err != nil {
		t.Fatalf("init postgres: %v", err)
	}
	defer db.Close()

	assertTableExists(t, ctx, db, "users")
	assertTableExists(t, ctx, db, "login_failures")
	assertTableExists(t, ctx, db, "network_devices")
	assertTableExists(t, ctx, db, "mqtt_control_deliveries")
}

func TestPostgresStoreCorePersistenceIntegration(t *testing.T) {
	dsn := os.Getenv("SLAN_BIZ_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("SLAN_BIZ_POSTGRES_TEST_DSN is not set")
	}
	t.Setenv("SLAN_BIZ_POSTGRES_DSN", dsn)
	t.Setenv("SLAN_BIZ_MIGRATIONS_DIR", "../../migrations")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := InitPostgresFromEnv(ctx)
	if err != nil {
		t.Fatalf("init postgres: %v", err)
	}
	defer db.Close()
	clearPostgresCoreTables(t, ctx, db)

	first := NewStoreWithPostgres(db)
	auth, _, err := first.RegisterUser("persist@example.com", "secret", "Persist")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	device, _, err := first.RegisterDevice(auth.User.UserID, "persist-device-1", "Device", "linux", "Linux", "1.0", "", "pub")
	if err != nil {
		t.Fatalf("register device: %v", err)
	}
	if _, err := first.LoginUser("persist@example.com", "secret"); err != nil {
		t.Fatalf("login first store: %v", err)
	}

	restarted := NewStoreWithPostgres(db)
	if _, err := restarted.LoginUser("persist@example.com", "secret"); err != nil {
		t.Fatalf("login after postgres reload: %v", err)
	}
	restoredDevice, err := restarted.GetDevice(device.DeviceID)
	if err != nil {
		t.Fatalf("get restored device: %v", err)
	}
	if restoredDevice.GlobalIP != device.GlobalIP {
		t.Fatalf("expected restored global ip %s, got %s", device.GlobalIP, restoredDevice.GlobalIP)
	}
	configs, err := restarted.NetworkConfigsForDevice(device.DeviceID)
	if err != nil {
		t.Fatalf("network configs after postgres reload: %v", err)
	}
	if len(configs) == 0 || configs[0].GlobalIP != device.GlobalIP {
		t.Fatalf("unexpected restored configs: %+v", configs)
	}
}

func TestPostgresStorePreloginDevicePersistenceIntegration(t *testing.T) {
	dsn := os.Getenv("SLAN_BIZ_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("SLAN_BIZ_POSTGRES_TEST_DSN is not set")
	}
	t.Setenv("SLAN_BIZ_POSTGRES_DSN", dsn)
	t.Setenv("SLAN_BIZ_MIGRATIONS_DIR", "../../migrations")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := InitPostgresFromEnv(ctx)
	if err != nil {
		t.Fatalf("init postgres: %v", err)
	}
	defer db.Close()
	clearPostgresCoreTables(t, ctx, db)

	first := NewStoreWithPostgres(db)
	auth, _, err := first.RegisterUser("prelogin@example.com", "secret", "Prelogin")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	prelogin, err := first.PrepareDeviceLoginDevice("prelogin-device-1", "Device", "linux", "Linux", "1.0", "", "pk_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err != nil {
		t.Fatalf("prepare prelogin device: %v", err)
	}
	if prelogin.OwnerID != "" || prelogin.GlobalIP != "" {
		t.Fatalf("expected unbound prelogin device without ip, got %+v", prelogin)
	}

	restarted := NewStoreWithPostgres(db)
	restored, err := restarted.GetDevice("prelogin-device-1")
	if err != nil {
		t.Fatalf("get restored prelogin device: %v", err)
	}
	if restored.OwnerID != "" || restored.GlobalIP != "" {
		t.Fatalf("expected restored prelogin device without owner/ip, got %+v", restored)
	}
	if _, err := restarted.CompleteDeviceLoginForDevice("prelogin-device-1", auth.Session.Token, "login"); err != nil {
		t.Fatalf("complete prelogin device: %v", err)
	}
	bound, err := restarted.GetDevice("prelogin-device-1")
	if err != nil {
		t.Fatalf("get bound prelogin device: %v", err)
	}
	if bound.OwnerID != auth.User.UserID || bound.GlobalIP == "" {
		t.Fatalf("expected bound prelogin device with owner/ip, got %+v", bound)
	}
}

func TestPostgresStoreDeviceLoginPrepareRateLimitPersistenceIntegration(t *testing.T) {
	dsn := os.Getenv("SLAN_BIZ_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("SLAN_BIZ_POSTGRES_TEST_DSN is not set")
	}
	t.Setenv("SLAN_BIZ_POSTGRES_DSN", dsn)
	t.Setenv("SLAN_BIZ_MIGRATIONS_DIR", "../../migrations")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := InitPostgresFromEnv(ctx)
	if err != nil {
		t.Fatalf("init postgres: %v", err)
	}
	defer db.Close()
	clearPostgresCoreTables(t, ctx, db)

	first := NewStoreWithPostgres(db)
	for i := 0; i < prepareRateLimitMax; i++ {
		if err := first.CheckDeviceLoginPrepareRateLimit("persist-prepare-device", "203.0.113.88"); err != nil {
			t.Fatalf("expected prepare attempt %d allowed, got %v", i, err)
		}
	}

	restarted := NewStoreWithPostgres(db)
	if err := restarted.CheckDeviceLoginPrepareRateLimit("persist-prepare-device", "203.0.113.88"); err != errRateLimited {
		t.Fatalf("expected persisted prepare rate limit after restart, got %v", err)
	}
}

func TestPostgresStoreUserLoginRateLimitPersistenceIntegration(t *testing.T) {
	dsn := os.Getenv("SLAN_BIZ_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("SLAN_BIZ_POSTGRES_TEST_DSN is not set")
	}
	t.Setenv("SLAN_BIZ_POSTGRES_DSN", dsn)
	t.Setenv("SLAN_BIZ_MIGRATIONS_DIR", "../../migrations")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := InitPostgresFromEnv(ctx)
	if err != nil {
		t.Fatalf("init postgres: %v", err)
	}
	defer db.Close()
	clearPostgresCoreTables(t, ctx, db)

	first := NewStoreWithPostgres(db)
	if _, _, err := first.RegisterUser("rate-persist@example.com", "secret", "Rate Persist"); err != nil {
		t.Fatalf("register user: %v", err)
	}
	for i := 0; i < loginRateLimitMaxFail; i++ {
		if _, err := first.LoginUserWithRateLimit("rate-persist@example.com", "bad", "203.0.113.89"); err != errBadRequest {
			t.Fatalf("expected bad password failure %d, got %v", i, err)
		}
	}

	restarted := NewStoreWithPostgres(db)
	if _, err := restarted.LoginUserWithRateLimit("rate-persist@example.com", "secret", "203.0.113.89"); err != errRateLimited {
		t.Fatalf("expected persisted user login rate limit after restart, got %v", err)
	}
}

func TestPostgresStoreMQTTControlDeliveryPersistenceIntegration(t *testing.T) {
	dsn := os.Getenv("SLAN_BIZ_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("SLAN_BIZ_POSTGRES_TEST_DSN is not set")
	}
	t.Setenv("SLAN_BIZ_POSTGRES_DSN", dsn)
	t.Setenv("SLAN_BIZ_MIGRATIONS_DIR", "../../migrations")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := InitPostgresFromEnv(ctx)
	if err != nil {
		t.Fatalf("init postgres: %v", err)
	}
	defer db.Close()
	clearPostgresCoreTables(t, ctx, db)

	first := NewStoreWithPostgres(db)
	auth, _, err := first.RegisterUser("mqtt-persist@example.com", "secret", "MQTT Persist")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	device, _, err := first.RegisterDevice(auth.User.UserID, "mqtt-persist-device", "Device", "linux", "Linux", "1.0", "", "pub")
	if err != nil {
		t.Fatalf("register device: %v", err)
	}
	if _, err := first.PrepareMQTTControlDelivery(device.DeviceID, "delivery-pg", "device_user_login_succeeded", "login", map[string]string{"userId": auth.User.UserID}, 100, 500); err != nil {
		t.Fatalf("prepare delivery: %v", err)
	}
	if _, err := first.RecordMQTTControlPublishResult(device.DeviceID, "delivery-pg", true, "", 110); err != nil {
		t.Fatalf("record publish: %v", err)
	}

	restarted := NewStoreWithPostgres(db)
	due := restarted.ListRetryableMQTTControlDeliveries(111, 10)
	if len(due) != 1 || due[0].DeliveryID != "delivery-pg" || due[0].Status != "published" {
		t.Fatalf("expected published unacked delivery to reload as retryable, got %+v", due)
	}
	if _, err := restarted.RecordMQTTControlAck(device.DeviceID, "delivery-pg", "task-login", "deviceUserLoginSucceeded", "succeeded", "", 123000, 120); err != nil {
		t.Fatalf("record ack: %v", err)
	}

	ackedStore := NewStoreWithPostgres(db)
	if due := ackedStore.ListRetryableMQTTControlDeliveries(200, 10); len(due) != 0 {
		t.Fatalf("expected acked delivery not retryable after reload, got %+v", due)
	}
	delivery, err := ackedStore.GetMQTTControlDeliveryForDevice(device.DeviceID, "delivery-pg")
	if err != nil {
		t.Fatalf("get acked delivery: %v", err)
	}
	if delivery.Status != "succeeded" || delivery.AckedAt != 120 || delivery.ProcessedAtMs != 123000 || delivery.Action != "login" {
		t.Fatalf("unexpected acked delivery after reload: %+v", delivery)
	}
}

func assertTableExists(t *testing.T, ctx context.Context, db *sql.DB, table string) {
	t.Helper()
	var exists bool
	if err := db.QueryRowContext(ctx, `select exists(
		select 1 from information_schema.tables
		where table_schema = 'public' and table_name = $1
	)`, table).Scan(&exists); err != nil {
		t.Fatalf("query table %s: %v", table, err)
	}
	if !exists {
		t.Fatalf("expected table %s to exist", table)
	}
}

func clearPostgresCoreTables(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	for _, statement := range []string{
		`delete from mqtt_control_deliveries`,
		`delete from device_sessions`,
		`delete from login_failures`,
		`delete from network_devices`,
		`delete from device_runtime_status`,
		`delete from network_dns_records`,
		`delete from public_domain_mappings`,
		`delete from security_group_rules`,
		`delete from global_ip_addresses`,
		`delete from devices`,
		`delete from user_sessions`,
		`delete from network_dns_zones`,
		`delete from security_groups`,
		`delete from networks`,
		`delete from users`,
		`delete from ipam_subnets`,
	} {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			t.Fatalf("clear postgres core table: %v", err)
		}
	}
}
