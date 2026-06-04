package biz

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

func (s *Store) ListRelayNodes() []OpsRelayNode {
	s.mu.Lock()
	defer s.mu.Unlock()
	return sortedValues(s.relayNodes, lessOpsRelayNode)
}

func lessOpsRelayNode(a, b OpsRelayNode) bool {
	aPriority := defaultInt(a.Priority, 100)
	bPriority := defaultInt(b.Priority, 100)
	if aPriority != bPriority {
		return aPriority < bPriority
	}
	if a.Region != b.Region {
		return a.Region < b.Region
	}
	return a.NodeID < b.NodeID
}

func (s *Store) UpsertRelayNode(node OpsRelayNode) (OpsRelayNode, error) {
	node.Name = strings.TrimSpace(node.Name)
	node.Region = defaultString(node.Region, "default")
	node.Transport = defaultString(node.Transport, "relay_udp")
	node.PublicAddr = relayCandidatePublicAddress(node.PublicAddr)
	node.Status = defaultString(node.Status, "active")
	node.Health = defaultString(node.Health, "healthy")
	if node.Name == "" || node.PublicAddr == "" {
		return OpsRelayNode{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.relayNodes {
		if existing.PublicAddr == node.PublicAddr && existing.NodeID != node.NodeID {
			return OpsRelayNode{}, errConflict
		}
	}
	now := time.Now().Unix()
	if strings.TrimSpace(node.NodeID) == "" {
		node.Health = "healthy"
		return s.addRelayNodeLocked(node), nil
	}
	existing, ok := s.relayNodes[node.NodeID]
	if !ok {
		return OpsRelayNode{}, errNotFound
	}
	node.CreatedAt = existing.CreatedAt
	node.UpdatedAt = now
	node.UsedTrafficGB = existing.UsedTrafficGB
	node.ActiveSessions = existing.ActiveSessions
	node.Health = existing.Health
	node.TicketKeyRotation = existing.TicketKeyRotation
	s.relayNodes[node.NodeID] = node
	return node, nil
}

func (s *Store) addRelayNodeLocked(node OpsRelayNode) OpsRelayNode {
	now := time.Now().Unix()
	node.NodeID = fmt.Sprintf("relay-%06d", s.nextRelayNodeSeq)
	s.nextRelayNodeSeq++
	node.CreatedAt = now
	node.UpdatedAt = now
	s.relayNodes[node.NodeID] = node
	return node
}

func (s *Store) UpdateRelayNodeStatus(nodeID string, req OpsNodeStatusRequest) (OpsRelayNode, error) {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" || (req.Enabled == nil && strings.TrimSpace(req.Status) == "" && strings.TrimSpace(req.Health) == "") {
		return OpsRelayNode{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	node, ok := s.relayNodes[nodeID]
	if !ok {
		return OpsRelayNode{}, errNotFound
	}
	if req.Enabled != nil {
		if *req.Enabled {
			node.Status = "active"
			if node.Health == "down" {
				node.Health = "healthy"
			}
		} else {
			node.Status = "disabled"
			node.Health = "down"
		}
	}
	if status := strings.TrimSpace(req.Status); status != "" {
		node.Status = status
	}
	if health := strings.TrimSpace(req.Health); health != "" {
		node.Health = health
	}
	node.UpdatedAt = time.Now().Unix()
	s.relayNodes[nodeID] = node
	return node, nil
}

func (s *Store) DeleteRelayNode(nodeID string) error {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.relayNodes[nodeID]; !ok {
		return errNotFound
	}
	delete(s.relayNodes, nodeID)
	return nil
}

func (s *Store) ListPunchNodes() []OpsPunchNode {
	s.mu.Lock()
	defer s.mu.Unlock()
	return sortedValues(s.punchNodes, func(a, b OpsPunchNode) bool {
		if a.Priority != b.Priority {
			return a.Priority < b.Priority
		}
		return a.NodeID < b.NodeID
	})
}

func (s *Store) ActivePunchNodes() []OpsPunchNode {
	s.mu.Lock()
	defer s.mu.Unlock()
	nodes := make([]OpsPunchNode, 0, len(s.punchNodes))
	for _, node := range s.punchNodes {
		if strings.TrimSpace(node.PublicUDPIP) == "" || node.PublicUDPPort <= 0 {
			continue
		}
		if !strings.EqualFold(defaultString(node.Status, "active"), "active") {
			continue
		}
		if !strings.EqualFold(defaultString(node.Health, "healthy"), "healthy") {
			continue
		}
		nodes = append(nodes, node)
	}
	sort.SliceStable(nodes, func(i, j int) bool {
		if nodes[i].Priority != nodes[j].Priority {
			return nodes[i].Priority < nodes[j].Priority
		}
		return nodes[i].NodeID < nodes[j].NodeID
	})
	return nodes
}

func (s *Store) UpsertPunchNode(node OpsPunchNode) (OpsPunchNode, error) {
	node.Name = strings.TrimSpace(node.Name)
	node.Region = defaultString(node.Region, "default")
	node.PublicUDPIP = strings.TrimSpace(node.PublicUDPIP)
	node.Status = defaultString(node.Status, "active")
	node.Health = defaultString(node.Health, "healthy")
	if node.Name == "" || node.PublicUDPIP == "" || node.PublicUDPPort <= 0 || node.PublicUDPPort > 65534 {
		return OpsPunchNode{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.punchNodes {
		if existing.PublicUDPIP == node.PublicUDPIP && existing.PublicUDPPort == node.PublicUDPPort && existing.NodeID != node.NodeID {
			return OpsPunchNode{}, errConflict
		}
	}
	now := time.Now().Unix()
	if strings.TrimSpace(node.NodeID) == "" {
		node.Health = defaultString(node.Health, "healthy")
		return s.addPunchNodeLocked(node), nil
	}
	existing, ok := s.punchNodes[node.NodeID]
	if !ok {
		return OpsPunchNode{}, errNotFound
	}
	node.CreatedAt = existing.CreatedAt
	node.UpdatedAt = now
	node.ActiveSessions = existing.ActiveSessions
	s.punchNodes[node.NodeID] = node
	return node, nil
}

func (s *Store) addPunchNodeLocked(node OpsPunchNode) OpsPunchNode {
	now := time.Now().Unix()
	node.NodeID = fmt.Sprintf("punch-%06d", s.nextPunchNodeSeq)
	s.nextPunchNodeSeq++
	node.CreatedAt = now
	node.UpdatedAt = now
	s.punchNodes[node.NodeID] = node
	return node
}

func (s *Store) UpdatePunchNodeStatus(nodeID string, req OpsNodeStatusRequest) (OpsPunchNode, error) {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" || (req.Enabled == nil && strings.TrimSpace(req.Status) == "" && strings.TrimSpace(req.Health) == "") {
		return OpsPunchNode{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	node, ok := s.punchNodes[nodeID]
	if !ok {
		return OpsPunchNode{}, errNotFound
	}
	if req.Enabled != nil {
		if *req.Enabled {
			node.Status = "active"
			if node.Health == "down" {
				node.Health = "healthy"
			}
		} else {
			node.Status = "disabled"
			node.Health = "down"
		}
	}
	if status := strings.TrimSpace(req.Status); status != "" {
		node.Status = status
	}
	if health := strings.TrimSpace(req.Health); health != "" {
		node.Health = health
	}
	node.UpdatedAt = time.Now().Unix()
	s.punchNodes[nodeID] = node
	return node, nil
}

func (s *Store) DeletePunchNode(nodeID string) error {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.punchNodes[nodeID]; !ok {
		return errNotFound
	}
	delete(s.punchNodes, nodeID)
	return nil
}
