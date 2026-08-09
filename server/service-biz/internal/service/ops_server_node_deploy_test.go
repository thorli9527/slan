package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/slan/service-biz/internal/model"
)

type serverNodeDeployTestCipher struct{}

func (serverNodeDeployTestCipher) Encrypt(value string) (string, error) { return value, nil }
func (serverNodeDeployTestCipher) Decrypt(value string) (string, error) { return value, nil }

type serverNodeDeployTestDeployer struct {
	fingerprint string
	deployCalls int
}

func (d *serverNodeDeployTestDeployer) ProbeHostKey(context.Context, model.ServerNode, string) (string, error) {
	return d.fingerprint, nil
}

func (d *serverNodeDeployTestDeployer) Deploy(_ context.Context, node model.ServerNode, _ string) (ServerNodeDeployResult, error) {
	d.deployCalls++
	return ServerNodeDeployResult{HostKeyFingerprint: node.SSHHostKeyFingerprint}, nil
}

func TestListServerNodesRecoversStaleDeployment(t *testing.T) {
	now := time.Unix(1_700_001_000, 0)
	repo := &deleteServerNodeRepo{item: model.ServerNode{
		NodeID: "server1", DeployStatus: "deploying", UpdatedAt: now.Add(-serverNodeDeploymentTimeout).Unix(),
	}}
	service := OpsServerNodeService{ServerNodes: repo, Now: func() time.Time { return now }}

	items, err := service.ListServerNodes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].DeployStatus != "failed" {
		t.Fatalf("unexpected server nodes: %#v", items)
	}
	if !strings.Contains(items[0].LastDeployError, "超时") {
		t.Fatalf("unexpected deploy error: %q", items[0].LastDeployError)
	}
}

func TestDeployServerNodeRequiresFirstHostKeyConfirmation(t *testing.T) {
	now := time.Unix(1_700_001_000, 0)
	repo := &deleteServerNodeRepo{item: model.ServerNode{
		NodeID: "server1", Name: "node", Host: "203.0.113.10", SSHPort: 22, SSHUsername: "root",
		SSHPasswordCiphertext: "password", DeployStatus: "not_deployed", CreatedAt: now.Unix(), UpdatedAt: now.Unix(),
	}}
	deployer := &serverNodeDeployTestDeployer{fingerprint: "SHA256:confirmed"}
	service := OpsServerNodeService{
		ServerNodes: repo, Nodes: &deleteRuntimeNodeRepo{}, Cipher: serverNodeDeployTestCipher{}, Deployer: deployer,
		Now: func() time.Time { return now },
	}

	if _, err := service.DeployServerNode(context.Background(), "server1", ""); err == nil || !strings.Contains(err.Error(), "confirmation") {
		t.Fatalf("expected host-key confirmation error, got %v", err)
	}
	if deployer.deployCalls != 0 {
		t.Fatalf("deploy calls = %d", deployer.deployCalls)
	}

	item, err := service.DeployServerNode(context.Background(), "server1", deployer.fingerprint)
	if err != nil {
		t.Fatal(err)
	}
	if item.DeployStatus != "succeeded" || repo.item.SSHHostKeyFingerprint != deployer.fingerprint {
		t.Fatalf("unexpected deployed node: %#v", item)
	}
}

func TestDeployServerNodeAdoptsRuntimeIDsForRecreatedHost(t *testing.T) {
	now := time.Unix(1_700_001_000, 0)
	repo := &deleteServerNodeRepo{item: model.ServerNode{
		NodeID: "server-new", Name: "node", Host: "203.0.113.10", SSHPort: 22, SSHUsername: "root",
		SSHPasswordCiphertext: "password", SSHHostKeyFingerprint: "SHA256:confirmed",
		RelayNodeID: "relay-new", RelayTCPNodeID: "derp-new", PunchNodeID: "punch-new",
		RelayUDPPort: 29110, RelayTCPPort: 29120, PunchUDPPort: 29130,
		DeployStatus: "not_deployed", CreatedAt: now.Unix(), UpdatedAt: now.Unix(),
	}}
	runtimeNodes := &runtimeNodeTestRepo{
		relayNodes: []model.RelayNode{
			{NodeID: "relay-existing", Endpoint: "udp://203.0.113.10:29110", Transport: relayTransportUDP},
			{NodeID: "derp-existing", Endpoint: "203.0.113.10:29120", Transport: relayTransportDerpTLS},
		},
		punchNodes: []model.PunchNode{{NodeID: "punch-existing", Endpoint: "203.0.113.10:29130"}},
	}
	deployer := &serverNodeDeployTestDeployer{fingerprint: "SHA256:confirmed"}
	service := OpsServerNodeService{
		ServerNodes: repo, Nodes: runtimeNodes, Cipher: serverNodeDeployTestCipher{}, Deployer: deployer,
		Now: func() time.Time { return now },
	}

	if _, err := service.DeployServerNode(context.Background(), "server-new", ""); err != nil {
		t.Fatal(err)
	}
	if repo.item.RelayNodeID != "relay-existing" || repo.item.RelayTCPNodeID != "derp-existing" || repo.item.PunchNodeID != "punch-existing" {
		t.Fatalf("runtime IDs were not adopted: %#v", repo.item)
	}
}

func TestNormalizeServerArchitecture(t *testing.T) {
	tests := map[string]string{
		"x86_64\n": "amd64",
		"amd64":    "amd64",
		"aarch64":  "arm64",
		"arm64":    "arm64",
		"":         "",
		"riscv64":  "",
	}
	for input, expected := range tests {
		if actual := normalizeServerArchitecture(input); actual != expected {
			t.Fatalf("normalizeServerArchitecture(%q) = %q, want %q", input, actual, expected)
		}
	}
}
