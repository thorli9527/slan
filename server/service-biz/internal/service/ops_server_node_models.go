package service

type OpsServerNodeView struct {
	NodeID                string `json:"nodeId"`
	Name                  string `json:"name"`
	Host                  string `json:"host"`
	SSHPort               int    `json:"sshPort"`
	SSHUsername           string `json:"sshUsername"`
	SSHPasswordConfigured bool   `json:"sshPasswordConfigured"`
	SSHHostKeyFingerprint string `json:"sshHostKeyFingerprint"`
	RelayUDPPort          int    `json:"relayUdpPort"`
	RelayAdminPort        int    `json:"relayAdminPort"`
	RelayTCPPort          int    `json:"relayTcpPort"`
	PunchUDPPort          int    `json:"punchUdpPort"`
	PunchHTTPPort         int    `json:"punchHttpPort"`
	APIProxyPort          int    `json:"apiProxyPort"`
	MQTTProxyPort         int    `json:"mqttProxyPort"`
	APIProxyURL           string `json:"apiProxyUrl"`
	MQTTProxyURL          string `json:"mqttProxyUrl"`
	RelayEnabled          bool   `json:"relayEnabled"`
	PunchEnabled          bool   `json:"punchEnabled"`
	ProxyEnabled          bool   `json:"proxyEnabled"`
	RelayNodeID           string `json:"relayNodeId"`
	RelayTCPNodeID        string `json:"relayTcpNodeId"`
	PunchNodeID           string `json:"punchNodeId"`
	DeployStatus          string `json:"deployStatus"`
	LastDeployError       string `json:"lastDeployError"`
	LastDeployedAt        int64  `json:"lastDeployedAt"`
	CreatedAt             int64  `json:"createdAt"`
	UpdatedAt             int64  `json:"updatedAt"`
}

type UpsertServerNodeInput struct {
	NodeID         string
	Name           string
	Host           string
	SSHPort        int
	SSHUsername    string
	SSHPassword    string
	RelayUDPPort   int
	RelayAdminPort int
	RelayTCPPort   int
	PunchUDPPort   int
	PunchHTTPPort  int
	APIProxyPort   int
	MQTTProxyPort  int
	RelayEnabled   bool
	PunchEnabled   bool
	ProxyEnabled   bool
}

type ServerNodeDeployResult struct {
	HostKeyFingerprint string
}

type ServerNodeHostKeyView struct {
	Fingerprint string `json:"fingerprint"`
}
