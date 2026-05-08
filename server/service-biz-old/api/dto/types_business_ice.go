package dto

type IceServer struct {
	ServerID  string   `json:"serverId"`
	Name      string   `json:"name,omitempty"`
	Provider  string   `json:"provider"`
	Region    string   `json:"region"`
	Country   string   `json:"country,omitempty"`
	PublicIP  string   `json:"publicIp,omitempty"`
	UDPAddr   string   `json:"udpAddr"`
	STUNPort  int      `json:"stunPort,omitempty"`
	Priority  int      `json:"priority"`
	Weight    int      `json:"weight"`
	Status    string   `json:"status,omitempty"`
	Features  []string `json:"features,omitempty"`
	Remark    string   `json:"remark,omitempty"`
	CreatedAt int64    `json:"createdAt,omitempty"`
	UpdatedAt int64    `json:"updatedAt,omitempty"`
}

type UpsertIceServerRequest struct {
	ServerID string   `json:"serverId"`
	Name     string   `json:"name"`
	Provider string   `json:"provider"`
	Region   string   `json:"region"`
	Country  string   `json:"country,omitempty"`
	PublicIP string   `json:"publicIp"`
	UDPAddr  string   `json:"udpAddr"`
	STUNPort int      `json:"stunPort,omitempty"`
	Priority int      `json:"priority,omitempty"`
	Weight   int      `json:"weight,omitempty"`
	Status   string   `json:"status,omitempty"`
	Features []string `json:"features,omitempty"`
	Remark   string   `json:"remark,omitempty"`
}

type UpdateIceServerStatusRequest struct {
	Status string `json:"status"`
}

type ClientIceServersResponse struct {
	IceServers       []IceServer `json:"iceServers"`
	RefreshAfterSec  int         `json:"refreshAfterSec"`
	ProbeIntervalSec int         `json:"probeIntervalSec"`
}

type Candidate struct {
	CandidateID    string `json:"candidateId"`
	CandidateType  string `json:"candidateType"`
	Addr           string `json:"addr"`
	SourceServerID string `json:"sourceServerId,omitempty"`
	Priority       int    `json:"priority,omitempty"`
}

type ReportCandidatesRequest struct {
	PeerID        string      `json:"peerId,omitempty"`
	NetworkID     string      `json:"networkId,omitempty"`
	UDPAvailable  bool        `json:"udpAvailable"`
	NATLevel      string      `json:"natLevel,omitempty"`
	MappingStable bool        `json:"mappingStable"`
	Candidates    []Candidate `json:"candidates"`
}

type ReportCandidatesResponse struct {
	PeerID    string `json:"peerId"`
	Stored    int    `json:"stored"`
	ExpiresIn int    `json:"expiresInSec"`
}

type CreatePunchPlanRequest struct {
	NetworkID string `json:"networkId,omitempty"`
	SrcPeerID string `json:"srcPeerId"`
	DstPeerID string `json:"dstPeerId"`
	DeviceID  string `json:"deviceId,omitempty"`
	PeerID    string `json:"peerId,omitempty"`
}

type PunchRetryPolicy struct {
	MaxRounds int   `json:"maxRounds"`
	BurstMS   []int `json:"burstMs"`
}

type PunchFallbackRelay struct {
	RelayID   string `json:"relayId,omitempty"`
	UDPAddr   string `json:"udpAddr,omitempty"`
	HTTP3Addr string `json:"http3Addr,omitempty"`
	TLSAddr   string `json:"tlsAddr,omitempty"`
}

type PunchPlan struct {
	SessionID     string             `json:"sessionId"`
	NetworkID     string             `json:"networkId,omitempty"`
	PeerA         string             `json:"peerA"`
	PeerB         string             `json:"peerB"`
	ACandidates   []Candidate        `json:"aCandidates"`
	BCandidates   []Candidate        `json:"bCandidates"`
	StartAfterMS  int                `json:"startAfterMs"`
	TimeoutMS     int                `json:"timeoutMs"`
	RetryPolicy   PunchRetryPolicy   `json:"retryPolicy"`
	FallbackRelay PunchFallbackRelay `json:"fallbackRelay,omitempty"`
}

type PunchSelectedPair struct {
	LocalCandidateID  string `json:"localCandidateId,omitempty"`
	RemoteCandidateID string `json:"remoteCandidateId,omitempty"`
	LocalSrflxAddr    string `json:"localSrflxAddr,omitempty"`
	RemoteAddrSeen    string `json:"remoteAddrSeen,omitempty"`
}

type PunchQuality struct {
	RTTMS    int     `json:"rttMs,omitempty"`
	LossRate float64 `json:"lossRate,omitempty"`
	JitterMS int     `json:"jitterMs,omitempty"`
}

type PunchFailure struct {
	Reason     string `json:"reason,omitempty"`
	TriedPairs int    `json:"triedPairs,omitempty"`
	TimeoutMS  int    `json:"timeoutMs,omitempty"`
}

type PunchFallbackReport struct {
	CurrentPath string `json:"currentPath,omitempty"`
	RelayID     string `json:"relayId,omitempty"`
}

type ReportPunchResultRequest struct {
	SessionID      string               `json:"sessionId,omitempty"`
	ReporterPeerID string               `json:"reporterPeerId"`
	RemotePeerID   string               `json:"remotePeerId"`
	Success        bool                 `json:"success"`
	PathKind       string               `json:"pathKind,omitempty"`
	SelectedPair   *PunchSelectedPair   `json:"selectedPair,omitempty"`
	Quality        *PunchQuality        `json:"quality,omitempty"`
	Failure        *PunchFailure        `json:"failure,omitempty"`
	Fallback       *PunchFallbackReport `json:"fallback,omitempty"`
}

type IceStats struct {
	Servers []IceServerStats `json:"servers,omitempty"`
}

type IceServerStats struct {
	ServerID        string  `json:"serverId"`
	Region          string  `json:"region,omitempty"`
	ProbeCount      int64   `json:"probeCount"`
	CandidateCount  int64   `json:"candidateCount"`
	P2PSuccessCount int64   `json:"p2pSuccessCount"`
	P2PFailureCount int64   `json:"p2pFailureCount"`
	AvgProbeRTTMS   int     `json:"avgProbeRttMs,omitempty"`
	SuccessRate     float64 `json:"successRate"`
	UpdatedAt       int64   `json:"updatedAt,omitempty"`
}
