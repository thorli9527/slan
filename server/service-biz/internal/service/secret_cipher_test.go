package service

import (
	"strings"
	"testing"

	"github.com/slan/service-biz/internal/model"
)

func TestAESGCMSecretCipherRoundTrip(t *testing.T) {
	cipher := NewAESGCMSecretCipher(strings.Repeat("11", 32))
	encrypted, err := cipher.Encrypt("ssh-password")
	if err != nil {
		t.Fatal(err)
	}
	if encrypted == "ssh-password" || strings.Contains(encrypted, "ssh-password") {
		t.Fatal("ciphertext exposed plaintext")
	}
	decrypted, err := cipher.Decrypt(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if decrypted != "ssh-password" {
		t.Fatalf("got %q", decrypted)
	}
}

func TestAESGCMSecretCipherRejectsMissingKey(t *testing.T) {
	if _, err := NewAESGCMSecretCipher("").Encrypt("password"); err == nil {
		t.Fatal("expected missing key error")
	}
}

func TestValidServerNodePortsRejectsDuplicates(t *testing.T) {
	valid := UpsertServerNodeInput{SSHPort: 22, RelayUDPPort: 29110, RelayAdminPort: 29111, RelayTCPPort: 29120, PunchUDPPort: 29130, PunchHTTPPort: 29131, APIProxyPort: 28080, MQTTProxyPort: 1883}
	if !validServerNodePorts(valid) {
		t.Fatal("expected valid ports")
	}
	valid.PunchUDPPort = valid.RelayUDPPort
	if validServerNodePorts(valid) {
		t.Fatal("expected duplicate ports to be rejected")
	}
}

func TestServerNodeDefaultsSetsTCPRelayIdentity(t *testing.T) {
	item := serverNodeDefaults(model.ServerNode{NodeID: "node1"})
	if item.RelayTCPPort != 29120 || item.RelayTCPNodeID != "derpnode1" {
		t.Fatalf("unexpected TCP relay defaults: %+v", item)
	}
}

func TestServiceActivationScriptStopsDisabledService(t *testing.T) {
	if got := serviceActivationScript("example.service", false); !strings.Contains(got, "disable --now example.service") {
		t.Fatalf("disabled activation script = %q", got)
	}
	if got := serviceActivationScript("example.service", true); !strings.Contains(got, "is-active --quiet example.service") {
		t.Fatalf("enabled activation script = %q", got)
	}
}

func TestServerNodeInstallScriptConfiguresTCPRelay(t *testing.T) {
	deployer := SSHServerNodeDeployer{
		BizURL:            "http://service-biz:8080",
		APIUpstreamURL:    "http://service-biz:8080",
		MQTTUpstreamAddr:  "mqtt:1883",
		InternalWireToken: "wire-token",
		TicketSecret:      "ticket-secret",
	}
	node := model.ServerNode{
		Host: "203.0.113.10", RelayUDPPort: 29110, RelayAdminPort: 29111, RelayTCPPort: 29120,
		PunchUDPPort: 29130, PunchHTTPPort: 29131, APIProxyPort: 28080, MQTTProxyPort: 1883,
		RelayEnabled: true, PunchEnabled: true, ProxyEnabled: true,
		RelayNodeID: "relay1", RelayTCPNodeID: "derp1", PunchNodeID: "punch1",
	}
	script := deployer.installScript(node)
	for _, expected := range []string{
		"SLAN_WIRE_DERP_LISTEN_ADDR=:29120",
		"SLAN_WIRE_DERP_ADMIN_LISTEN_ADDR=127.0.0.1:29121",
		"ExecStart=/opt/slan/node/server-wire-derp",
		"systemctl enable --now slan-wire-derp.service",
		"ufw allow 29120/tcp",
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("install script does not contain %q", expected)
		}
	}
}

func TestServerNodeDeploymentConfigDetectsCapabilityChanges(t *testing.T) {
	left := serverNodeDefaults(model.ServerNode{Host: "203.0.113.10", RelayEnabled: true, PunchEnabled: true, ProxyEnabled: true})
	right := left
	if !serverNodeDeploymentConfigEqual(left, right) {
		t.Fatal("identical deployment configuration must compare equal")
	}
	right.ProxyEnabled = false
	if serverNodeDeploymentConfigEqual(left, right) {
		t.Fatal("proxy capability change must require redeployment")
	}
	right = left
	right.RelayTCPPort++
	if serverNodeDeploymentConfigEqual(left, right) {
		t.Fatal("TCP relay port change must require redeployment")
	}
}
