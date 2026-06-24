package service

func prepareRelayTicketCandidates(
	candidates []RelayCandidateView,
	preferredEndpointIDs []string,
	runtimePath NetworkRuntimePathView,
	hasRuntimePath bool,
) ([]RelayCandidateView, []string) {
	if !hasRuntimePath {
		return candidates, preferredEndpointIDs
	}
	candidates = orderRelayCandidatesByRuntime(runtimePath, candidates)
	preferred := append([]string(nil), preferredEndpointIDs...)
	if runtimePath.DerpNodeID != "" && runtimePath.ActivePath == "derp_tcp_tls_443" {
		preferred = append([]string{runtimePath.DerpNodeID}, preferred...)
	}
	if runtimePath.RelayEndpoint != "" {
		for _, item := range candidates {
			if item.Address == runtimePath.RelayEndpoint && (runtimePath.RelayTransport == "" || item.Transport == runtimePath.RelayTransport) {
				preferred = append([]string{item.EndpointID}, preferred...)
				break
			}
		}
	}
	return candidates, preferred
}
