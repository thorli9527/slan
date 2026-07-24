package clientlogkit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveUsesDeviceDirectoryAndPrivateFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SLAN_CLIENT_LOG_DIR", dir)
	name, size, err := Save("device/../one", map[string]any{"ok": true})
	if err != nil {
		t.Fatal(err)
	}
	if size == 0 {
		t.Fatal("saved bundle is empty")
	}
	path := filepath.Join(dir, "deviceone", name)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("unexpected permissions: %o", info.Mode().Perm())
	}
}
