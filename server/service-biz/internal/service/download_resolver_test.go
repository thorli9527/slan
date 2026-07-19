package service

import (
	"os/exec"
	"strings"
	"testing"
)

func TestClientBootstrapInstallScriptStopsExistingRuntimeBeforeReplacingFiles(t *testing.T) {
	script := clientBootstrapInstallScript(nil)
	fragments := []string{
		"stop_runtime()",
		"/proc/[0-9]*/exe",
		`"$install_root/bin/client-core-service"`,
		"stop_runtime\n    rm -rf \"$install_root\"",
		"systemctl daemon-reload",
	}
	for _, fragment := range fragments {
		if !strings.Contains(script, fragment) {
			t.Fatalf("generated install script does not contain %q", fragment)
		}
	}
	if output, err := exec.Command("sh", "-n", "-c", script).CombinedOutput(); err != nil {
		t.Fatalf("generated install script is invalid: %v\n%s", err, output)
	}
}
