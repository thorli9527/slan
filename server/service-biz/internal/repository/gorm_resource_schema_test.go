package repository

import (
	"reflect"
	"strings"
	"testing"
)

func TestManagedResourceSchemaHasNoUserOwnershipColumns(t *testing.T) {
	checks := []struct {
		record any
		field  string
	}{
		{record: gormDeviceRecord{}, field: "OwnerID"},
		{record: gormNetworkRecord{}, field: "OwnerID"},
		{record: gormDeviceGroupRecord{}, field: "UserID"},
		{record: gormDeviceGroupAssignmentRecord{}, field: "UserID"},
	}
	for _, check := range checks {
		if _, ok := reflect.TypeOf(check.record).FieldByName(check.field); ok {
			t.Fatalf("managed resource record must not contain %s", check.field)
		}
	}
}

func TestIdentityFieldsHaveDatabaseUniqueConstraints(t *testing.T) {
	checks := []struct {
		record any
		field  string
	}{
		{record: gormCustomerRecord{}, field: "Email"},
		{record: gormOperatorRecord{}, field: "Email"},
		{record: gormDeviceSessionRecord{}, field: "DeviceID"},
		{record: gormDeviceCredentialRecord{}, field: "KeyID"},
		{record: gormOperatorSessionRecord{}, field: "AccessToken"},
	}
	for _, check := range checks {
		field, ok := reflect.TypeOf(check.record).FieldByName(check.field)
		if !ok {
			t.Fatalf("%T must contain %s", check.record, check.field)
		}
		if !strings.Contains(field.Tag.Get("gorm"), "uniqueIndex") {
			t.Fatalf("%T.%s must have a database unique index", check.record, check.field)
		}
	}
}

func TestDeviceVirtualIPHasPartialUniqueConstraint(t *testing.T) {
	field, ok := reflect.TypeOf(gormDeviceRecord{}).FieldByName("VirtualIP")
	if !ok {
		t.Fatal("gormDeviceRecord must contain VirtualIP")
	}
	tag := field.Tag.Get("gorm")
	if !strings.Contains(tag, "uniqueIndex:uidx_devices_virtual_ip") || !strings.Contains(tag, "where:virtual_ip <> ''") {
		t.Fatalf("VirtualIP must be unique when non-empty, tag=%q", tag)
	}
}
