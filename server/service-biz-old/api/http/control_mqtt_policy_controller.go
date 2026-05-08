package httpapi

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/netpath"
)

const relayPolicyControllerInterval = 10 * time.Minute

var relayPolicyControllerOnce sync.Once

// startRelayDataPlanePolicyController keeps network-level packet-size policy in
// sync with recent quality samples. Manual device overrides still go through
// the ops API and have higher priority on the client.
func startRelayDataPlanePolicyController(deps routerDeps) {
	if deps.Ops == nil || deps.ControlChannel == nil {
		return
	}
	relayPolicyControllerOnce.Do(func() {
		go func() {
			timer := time.NewTimer(30 * time.Second)
			defer timer.Stop()
			for {
				<-timer.C
				evaluateAndPublishRelayDataPlanePolicies(deps)
				timer.Reset(relayPolicyControllerInterval)
			}
		}()
	})
}

func evaluateAndPublishRelayDataPlanePolicies(deps routerDeps) {
	quality, err := deps.Ops.NetworkQuality()
	if err != nil {
		log.Printf("relay policy controller quality read failed: %v", err)
		return
	}
	for _, network := range quality.Summary.Networks {
		req, ok := relayPolicyRequestForNetworkSummary(network)
		if !ok {
			continue
		}
		if _, err := publishOpsRelayDataPlanePolicy(deps, req); err != nil {
			log.Printf("relay policy controller publish failed network=%s err=%v", network.NetworkID, err)
		}
	}
}

func relayPolicyRequestForNetworkSummary(summary dto.OpsNetworkQualityNetworkSummary) (dto.OpsRelayDataPlanePolicyRequest, bool) {
	samples := summary.CrossCountry.SampleCount + summary.NonCrossCountry.SampleCount
	if summary.NetworkID == "" || samples == 0 {
		return dto.OpsRelayDataPlanePolicyRequest{}, false
	}
	worstLoss := summary.CrossCountry.AvgPacketLossPpm
	if summary.NonCrossCountry.AvgPacketLossPpm > worstLoss {
		worstLoss = summary.NonCrossCountry.AvgPacketLossPpm
	}

	recLevel := uint8(2)
	execLevel := uint8(1)
	relayMtu := uint32(1280)
	maxPayload := uint32(1200)
	reason := "automated_network_quality_stable"
	switch {
	case worstLoss >= 50_000:
		recLevel = 8
		execLevel = 5
		relayMtu = 1100
		maxPayload = 1020
		reason = "automated_network_quality_high_loss"
	case worstLoss >= 10_000:
		recLevel = 5
		execLevel = 3
		relayMtu = 1200
		maxPayload = 1120
		reason = "automated_network_quality_degraded"
	}

	policyID := fmt.Sprintf("relay-policy-%s-l%d-m%d-p%d", summary.NetworkID, execLevel, relayMtu, maxPayload)
	return dto.OpsRelayDataPlanePolicyRequest{
		NetworkID:           summary.NetworkID,
		Scope:               "network",
		PathType:            "any",
		PreferredPathTypes:  netpath.BandwidthSavingPreferredPathTypes(),
		PolicyID:            policyID,
		Version:             1,
		RecommendationLevel: &recLevel,
		ExecutionLevel:      &execLevel,
		RelayMtu:            relayMtu,
		MaxFramePayload:     maxPayload,
		Reason:              reason,
		TTLMS:               uint64(relayPolicyControllerInterval * 2 / time.Millisecond),
	}, true
}
