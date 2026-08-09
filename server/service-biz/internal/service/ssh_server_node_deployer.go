package service

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/slan/service-biz/internal/model"
	"golang.org/x/crypto/ssh"
)

type SSHServerNodeDeployer struct {
	AssetsDir         string
	BizURL            string
	APIUpstreamURL    string
	MQTTUpstreamAddr  string
	InternalWireToken string
	TicketSecret      string
}

func (d SSHServerNodeDeployer) ProbeHostKey(ctx context.Context, node model.ServerNode, password string) (string, error) {
	client, fingerprint, err := openSSHClient(ctx, node, password, false)
	if err != nil {
		return "", err
	}
	client.Close()
	return fingerprint, nil
}

func (d SSHServerNodeDeployer) Deploy(ctx context.Context, node model.ServerNode, password string) (ServerNodeDeployResult, error) {
	if (node.RelayEnabled && (strings.TrimSpace(d.BizURL) == "" || strings.TrimSpace(d.InternalWireToken) == "" || strings.TrimSpace(d.TicketSecret) == "")) ||
		(node.PunchEnabled && strings.TrimSpace(d.InternalWireToken) == "") ||
		(node.ProxyEnabled && (strings.TrimSpace(d.APIUpstreamURL) == "" || strings.TrimSpace(d.MQTTUpstreamAddr) == "")) {
		return ServerNodeDeployResult{}, errors.New("node deployment server environment is incomplete")
	}
	if node.ProxyEnabled {
		if err := d.validateProxyUpstreams(node); err != nil {
			return ServerNodeDeployResult{}, err
		}
	}
	client, observedFingerprint, err := openSSHClient(ctx, node, password, true)
	if err != nil {
		return ServerNodeDeployResult{}, err
	}
	defer client.Close()
	stopCancel := context.AfterFunc(ctx, func() { _ = client.Close() })
	defer stopCancel()
	if output, err := sshRun(ctx, client, "test \"$(id -u)\" = 0"); err != nil {
		return ServerNodeDeployResult{}, fmt.Errorf("SSH account must be root: %s: %w", output, err)
	}
	architecture, err := remoteNodeArchitecture(ctx, client)
	if err != nil {
		return ServerNodeDeployResult{}, err
	}
	relayBinary, err := os.ReadFile(filepath.Join(d.AssetsDir, "server-wire-relay-"+architecture))
	if err != nil {
		return ServerNodeDeployResult{}, fmt.Errorf("read relay deployment asset: %w", err)
	}
	derpBinary, err := os.ReadFile(filepath.Join(d.AssetsDir, "server-wire-derp-"+architecture))
	if err != nil {
		return ServerNodeDeployResult{}, fmt.Errorf("read TCP relay deployment asset: %w", err)
	}
	punchBinary, err := os.ReadFile(filepath.Join(d.AssetsDir, "server-wire-punch-"+architecture))
	if err != nil {
		return ServerNodeDeployResult{}, fmt.Errorf("read punch deployment asset: %w", err)
	}
	proxyBinary, err := os.ReadFile(filepath.Join(d.AssetsDir, "server-edge-proxy-"+architecture))
	if err != nil {
		return ServerNodeDeployResult{}, fmt.Errorf("read edge proxy deployment asset: %w", err)
	}
	if err := sshUpload(ctx, client, "/tmp/slan-server-wire-relay", relayBinary); err != nil {
		return ServerNodeDeployResult{}, err
	}
	if err := sshUpload(ctx, client, "/tmp/slan-server-wire-derp", derpBinary); err != nil {
		return ServerNodeDeployResult{}, err
	}
	if err := sshUpload(ctx, client, "/tmp/slan-server-wire-punch", punchBinary); err != nil {
		return ServerNodeDeployResult{}, err
	}
	if err := sshUpload(ctx, client, "/tmp/slan-server-edge-proxy", proxyBinary); err != nil {
		return ServerNodeDeployResult{}, err
	}
	script := d.installScript(node)
	if output, err := sshRunInput(ctx, client, "sh -s", []byte(script)); err != nil {
		return ServerNodeDeployResult{}, fmt.Errorf("install node services: %s: %w", truncateDeployError(output), err)
	}
	return ServerNodeDeployResult{HostKeyFingerprint: observedFingerprint}, nil
}

func openSSHClient(ctx context.Context, node model.ServerNode, password string, requireTrustedKey bool) (*ssh.Client, string, error) {
	var observedFingerprint string
	config := &ssh.ClientConfig{
		User: node.SSHUsername, Auth: []ssh.AuthMethod{ssh.Password(password)}, Timeout: 15 * time.Second,
		HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
			observedFingerprint = ssh.FingerprintSHA256(key)
			if requireTrustedKey && node.SSHHostKeyFingerprint == "" {
				return errors.New("SSH host key is not trusted")
			}
			if node.SSHHostKeyFingerprint != "" && node.SSHHostKeyFingerprint != observedFingerprint {
				return fmt.Errorf("SSH host key changed: expected %s, received %s", node.SSHHostKeyFingerprint, observedFingerprint)
			}
			return nil
		},
	}
	address := net.JoinHostPort(node.Host, strconv.Itoa(node.SSHPort))
	connection, err := (&net.Dialer{Timeout: 15 * time.Second}).DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, "", fmt.Errorf("connect SSH: %w", err)
	}
	clientConn, channels, requests, err := ssh.NewClientConn(connection, address, config)
	if err != nil {
		connection.Close()
		return nil, observedFingerprint, fmt.Errorf("authenticate SSH: %w", err)
	}
	return ssh.NewClient(clientConn, channels, requests), observedFingerprint, nil
}

func (d SSHServerNodeDeployer) validateProxyUpstreams(node model.ServerNode) error {
	apiUpstream, err := url.Parse(strings.TrimSpace(d.APIUpstreamURL))
	if err != nil || (apiUpstream.Scheme != "http" && apiUpstream.Scheme != "https") || apiUpstream.Hostname() == "" {
		return errors.New("invalid API proxy upstream URL")
	}
	apiPort := apiUpstream.Port()
	if apiPort == "" {
		if apiUpstream.Scheme == "https" {
			apiPort = "443"
		} else {
			apiPort = "80"
		}
	}
	if apiUpstream.Hostname() == node.Host && apiPort == strconv.Itoa(node.APIProxyPort) {
		return errors.New("API proxy upstream points to the proxy node itself")
	}
	mqttHost, mqttPort, err := net.SplitHostPort(strings.TrimSpace(d.MQTTUpstreamAddr))
	if err != nil {
		return fmt.Errorf("invalid MQTT proxy upstream address: %w", err)
	}
	if mqttHost == node.Host && mqttPort == strconv.Itoa(node.MQTTProxyPort) {
		return errors.New("MQTT proxy upstream points to the proxy node itself")
	}
	if strings.ContainsAny(d.APIUpstreamURL+d.MQTTUpstreamAddr, "\r\n") {
		return errors.New("proxy upstream contains invalid characters")
	}
	return nil
}

func remoteNodeArchitecture(ctx context.Context, client *ssh.Client) (string, error) {
	commands := []string{"uname -m", "arch", "dpkg --print-architecture", "apk --print-arch"}
	observed := make([]string, 0, len(commands))
	for _, command := range commands {
		for attempt := 0; attempt < 2; attempt++ {
			value, err := sshRun(ctx, client, command+" 2>/dev/null")
			if architecture := normalizeServerArchitecture(value); architecture != "" {
				return architecture, nil
			}
			if err != nil {
				observed = append(observed, command+": "+err.Error())
			} else {
				observed = append(observed, command+": "+strconv.Quote(strings.TrimSpace(value)))
			}
			if err := contextSleep(ctx, 200*time.Millisecond); err != nil {
				return "", fmt.Errorf("detect server architecture: %w", err)
			}
		}
	}
	return "", fmt.Errorf("detect server architecture failed (%s)", strings.Join(observed, "; "))
}

func normalizeServerArchitecture(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "x86_64", "amd64", "x64":
		return "amd64"
	case "aarch64", "arm64", "arm64/v8":
		return "arm64"
	default:
		return ""
	}
}

func contextSleep(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func sshUpload(ctx context.Context, client *ssh.Client, path string, data []byte) error {
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write(data); err != nil {
		return fmt.Errorf("compress %s: %w", filepath.Base(path), err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("compress %s: %w", filepath.Base(path), err)
	}
	compressedPath := path + ".gz"
	command := "umask 077 && cat > " + shellQuote(compressedPath) +
		" && gzip -dc " + shellQuote(compressedPath) + " > " + shellQuote(path) +
		" && rm " + shellQuote(compressedPath)
	output, err := sshRunInput(ctx, client, command, compressed.Bytes())
	if err != nil {
		return fmt.Errorf("upload %s: %s: %w", filepath.Base(path), truncateDeployError(output), err)
	}
	return nil
}

func sshRun(ctx context.Context, client *ssh.Client, command string) (string, error) {
	return sshRunInput(ctx, client, command, nil)
}

func sshRunInput(ctx context.Context, client *ssh.Client, command string, input []byte) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()
	if input != nil {
		session.Stdin = bytes.NewReader(input)
	}
	var combined bytes.Buffer
	session.Stdout, session.Stderr = &combined, &combined
	done := make(chan error, 1)
	go func() { done <- session.Run(command) }()
	select {
	case err = <-done:
		return strings.TrimSpace(combined.String()), err
	case <-ctx.Done():
		_ = session.Close()
		return strings.TrimSpace(combined.String()), ctx.Err()
	}
}

func (d SSHServerNodeDeployer) installScript(node model.ServerNode) string {
	relayEnv := strings.Join([]string{
		"SLAN_ENV=production", "SLAN_WIRE_RELAY_LISTEN_ADDR=:" + strconv.Itoa(node.RelayUDPPort), "SLAN_WIRE_RELAY_ADMIN_LISTEN_ADDR=:" + strconv.Itoa(node.RelayAdminPort),
		"SLAN_BIZ_URL=" + d.BizURL, "SLAN_INTERNAL_WIRE_TOKEN=" + d.InternalWireToken,
		"SLAN_WIRE_RELAY_REGION_ID=managed", "SLAN_WIRE_RELAY_NODE_ID=" + node.RelayNodeID,
		"SLAN_WIRE_RELAY_PUBLIC_HOST=" + node.Host, "SLAN_WIRE_RELAY_PUBLIC_UDP_PORT=" + strconv.Itoa(node.RelayUDPPort),
		"SLAN_WIRE_RELAY_PUBLIC_ADMIN_PORT=" + strconv.Itoa(node.RelayAdminPort),
		"SLAN_WIRE_TICKET_SECRET=" + d.TicketSecret, "SLAN_WIRE_TICKET_SECRETS=" + d.TicketSecret,
	}, "\n")
	derpEnv := strings.Join([]string{
		"SLAN_ENV=production", "SLAN_WIRE_DERP_LISTEN_ADDR=:" + strconv.Itoa(node.RelayTCPPort), "SLAN_WIRE_DERP_ADMIN_LISTEN_ADDR=127.0.0.1:" + strconv.Itoa(node.RelayTCPPort+1),
		"SLAN_BIZ_URL=" + d.BizURL, "SLAN_INTERNAL_WIRE_TOKEN=" + d.InternalWireToken,
		"SLAN_WIRE_DERP_REGION_ID=managed", "SLAN_WIRE_DERP_NODE_ID=" + node.RelayTCPNodeID,
		"SLAN_WIRE_DERP_PUBLIC_HOST=" + node.Host, "SLAN_WIRE_DERP_PUBLIC_PORT=" + strconv.Itoa(node.RelayTCPPort),
		"SLAN_WIRE_TICKET_SECRET=" + d.TicketSecret, "SLAN_WIRE_TICKET_SECRETS=" + d.TicketSecret,
	}, "\n")
	punchEnv := strings.Join([]string{
		"SLAN_ENV=production", "SLAN_WIRE_PUNCH_LISTEN_ADDR=:" + strconv.Itoa(node.PunchUDPPort), "SLAN_WIRE_PUNCH_HTTP_LISTEN_ADDR=:" + strconv.Itoa(node.PunchHTTPPort),
		"SLAN_BIZ_URL=" + d.BizURL, "SLAN_WIRE_PUNCH_NODE_ID=" + node.PunchNodeID,
		"SLAN_WIRE_PUNCH_PUBLIC_HOST=" + node.Host, "SLAN_WIRE_PUNCH_PUBLIC_UDP_PORT=" + strconv.Itoa(node.PunchUDPPort),
		"SLAN_INTERNAL_WIRE_TOKEN=" + d.InternalWireToken,
	}, "\n")
	proxyEnv := strings.Join([]string{
		"SLAN_EDGE_PROXY_API_LISTEN_ADDR=:" + strconv.Itoa(node.APIProxyPort),
		"SLAN_EDGE_PROXY_API_UPSTREAM=" + d.APIUpstreamURL,
		"SLAN_EDGE_PROXY_MQTT_LISTEN_ADDR=:" + strconv.Itoa(node.MQTTProxyPort),
		"SLAN_EDGE_PROXY_MQTT_UPSTREAM=" + d.MQTTUpstreamAddr,
	}, "\n")
	relayAction := serviceActivationScript("slan-wire-relay.service", node.RelayEnabled) + "\n" + serviceActivationScript("slan-wire-derp.service", node.RelayEnabled)
	punchAction := serviceActivationScript("slan-wire-punch.service", node.PunchEnabled)
	proxyAction := serviceActivationScript("slan-edge-proxy.service", node.ProxyEnabled)
	return fmt.Sprintf(`set -eu
install -d -m 0755 /opt/slan/node /etc/slan
install -m 0755 /tmp/slan-server-wire-relay /opt/slan/node/server-wire-relay
install -m 0755 /tmp/slan-server-wire-derp /opt/slan/node/server-wire-derp
install -m 0755 /tmp/slan-server-wire-punch /opt/slan/node/server-wire-punch
install -m 0755 /tmp/slan-server-edge-proxy /opt/slan/node/server-edge-proxy
cat > /etc/slan/relay.env <<'EOF_RELAY'
%s
EOF_RELAY
cat > /etc/slan/derp.env <<'EOF_DERP'
%s
EOF_DERP
cat > /etc/slan/punch.env <<'EOF_PUNCH'
%s
EOF_PUNCH
cat > /etc/slan/edge-proxy.env <<'EOF_PROXY'
%s
EOF_PROXY
chmod 0600 /etc/slan/relay.env /etc/slan/derp.env /etc/slan/punch.env /etc/slan/edge-proxy.env
cat > /etc/systemd/system/slan-wire-relay.service <<'EOF_SERVICE'
[Unit]
Description=SLAN UDP Relay
After=network-online.target
Wants=network-online.target
[Service]
EnvironmentFile=/etc/slan/relay.env
ExecStart=/opt/slan/node/server-wire-relay
Restart=always
RestartSec=2
LimitNOFILE=1048576
[Install]
WantedBy=multi-user.target
EOF_SERVICE
cat > /etc/systemd/system/slan-wire-derp.service <<'EOF_SERVICE'
[Unit]
Description=SLAN TCP Relay
After=network-online.target
Wants=network-online.target
[Service]
EnvironmentFile=/etc/slan/derp.env
ExecStart=/opt/slan/node/server-wire-derp
Restart=always
RestartSec=2
LimitNOFILE=1048576
[Install]
WantedBy=multi-user.target
EOF_SERVICE
cat > /etc/systemd/system/slan-wire-punch.service <<'EOF_SERVICE'
[Unit]
Description=SLAN UDP Punch
After=network-online.target
Wants=network-online.target
[Service]
EnvironmentFile=/etc/slan/punch.env
ExecStart=/opt/slan/node/server-wire-punch
Restart=always
RestartSec=2
LimitNOFILE=1048576
[Install]
WantedBy=multi-user.target
EOF_SERVICE
cat > /etc/systemd/system/slan-edge-proxy.service <<'EOF_SERVICE'
[Unit]
Description=SLAN App API and MQTT Edge Proxy
After=network-online.target
Wants=network-online.target
[Service]
EnvironmentFile=/etc/slan/edge-proxy.env
ExecStart=/opt/slan/node/server-edge-proxy
Restart=always
RestartSec=2
LimitNOFILE=1048576
[Install]
WantedBy=multi-user.target
EOF_SERVICE
systemctl daemon-reload
%s
%s
%s
if command -v ufw >/dev/null 2>&1; then
  ufw allow %d/udp >/dev/null || true
  ufw allow %d/tcp >/dev/null || true
  ufw allow %d/tcp >/dev/null || true
  ufw allow %d/udp >/dev/null || true
  ufw allow %d/tcp >/dev/null || true
  ufw allow %d/tcp >/dev/null || true
  ufw allow %d/tcp >/dev/null || true
fi
rm -f /tmp/slan-server-wire-relay /tmp/slan-server-wire-derp /tmp/slan-server-wire-punch /tmp/slan-server-edge-proxy
`, relayEnv, derpEnv, punchEnv, proxyEnv, relayAction, punchAction, proxyAction, node.RelayUDPPort, node.RelayAdminPort, node.RelayTCPPort, node.PunchUDPPort, node.PunchHTTPPort, node.APIProxyPort, node.MQTTProxyPort)
}

func serviceActivationScript(service string, enabled bool) string {
	if !enabled {
		return "systemctl disable --now " + service + " >/dev/null 2>&1 || true"
	}
	return "systemctl enable --now " + service + "\n" +
		"systemctl restart " + service + "\n" +
		"systemctl is-active --quiet " + service
}

func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }
