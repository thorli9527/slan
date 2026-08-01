package service

import "context"

func (s OpsNodeService) ListRelayNodes(ctx context.Context) ([]OpsRelayNodeView, error) {
	items, err := s.Nodes.ListRelayNodes(ctx)
	if err != nil {
		return nil, err
	}
	return opsRelayNodeViews(items), nil
}

func (s OpsNodeService) UpsertRelayNode(ctx context.Context, input UpsertNodeInput) (OpsRelayNodeView, error) {
	input = normalizeUpsertNodeInput(input)
	if input.Name == "" && input.NodeID == "" {
		return OpsRelayNodeView{}, ErrInvalidArgument
	}
	now := opsNow(s.Now).Unix()
	nodeID := opsNodeID(s.Nodes, input.NodeID, "relay")
	item := newRelayNode(nodeID, now, input)
	if current, ok, err := s.Nodes.GetRelayNode(ctx, item.NodeID); err != nil {
		return OpsRelayNodeView{}, err
	} else if ok {
		item = mergeRelayNode(current, input, now)
	}
	if err := validateRelayNodeModel(ctx, s.Nodes, item); err != nil {
		return OpsRelayNodeView{}, err
	}
	if err := s.Nodes.SaveRelayNode(ctx, item); err != nil {
		return OpsRelayNodeView{}, err
	}
	return opsRelayNodeViewFromModel(item), nil
}

func (s OpsNodeService) UpdateRelayNodeStatus(ctx context.Context, nodeID, status string) (OpsRelayNodeView, error) {
	item, ok, err := s.Nodes.GetRelayNode(ctx, normalizeNodeID(nodeID))
	if err != nil {
		return OpsRelayNodeView{}, err
	}
	if !ok {
		return OpsRelayNodeView{}, ErrNotFound
	}
	item = applyRelayNodeStatus(item, status, opsNow(s.Now).Unix())
	if err := s.Nodes.SaveRelayNode(ctx, item); err != nil {
		return OpsRelayNodeView{}, err
	}
	return opsRelayNodeViewFromModel(item), nil
}

func (s OpsNodeService) DeleteRelayNode(ctx context.Context, nodeID string) error {
	return s.Nodes.DeleteRelayNode(ctx, normalizeNodeID(nodeID))
}

func (s OpsNodeService) ListPunchNodes(ctx context.Context) ([]OpsPunchNodeView, error) {
	items, err := s.Nodes.ListPunchNodes(ctx)
	if err != nil {
		return nil, err
	}
	return opsPunchNodeViews(items), nil
}

func (s OpsNodeService) UpsertPunchNode(ctx context.Context, input UpsertNodeInput) (OpsPunchNodeView, error) {
	input = normalizeUpsertNodeInput(input)
	if input.Name == "" && input.NodeID == "" {
		return OpsPunchNodeView{}, ErrInvalidArgument
	}
	now := opsNow(s.Now).Unix()
	item := newPunchNode(opsNodeID(s.Nodes, input.NodeID, "punch"), now, input)
	if current, ok, err := s.Nodes.GetPunchNode(ctx, item.NodeID); err != nil {
		return OpsPunchNodeView{}, err
	} else if ok {
		item = mergePunchNode(current, input, now)
	}
	if err := validatePunchNodeModel(ctx, s.Nodes, item); err != nil {
		return OpsPunchNodeView{}, err
	}
	if err := s.Nodes.SavePunchNode(ctx, item); err != nil {
		return OpsPunchNodeView{}, err
	}
	return opsPunchNodeViewFromModel(item), nil
}

func (s OpsNodeService) UpdatePunchNodeStatus(ctx context.Context, nodeID, status string) (OpsPunchNodeView, error) {
	item, ok, err := s.Nodes.GetPunchNode(ctx, normalizeNodeID(nodeID))
	if err != nil {
		return OpsPunchNodeView{}, err
	}
	if !ok {
		return OpsPunchNodeView{}, ErrNotFound
	}
	item = applyPunchNodeStatus(item, status, opsNow(s.Now).Unix())
	if err := s.Nodes.SavePunchNode(ctx, item); err != nil {
		return OpsPunchNodeView{}, err
	}
	return opsPunchNodeViewFromModel(item), nil
}

func (s OpsNodeService) DeletePunchNode(ctx context.Context, nodeID string) error {
	return s.Nodes.DeletePunchNode(ctx, normalizeNodeID(nodeID))
}
