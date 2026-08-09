package service

import "context"

func (s RuntimeNodeRegistryService) ListRelayNodes(ctx context.Context) ([]RuntimeRelayNodeView, error) {
	items, err := s.Nodes.ListRelayNodes(ctx)
	if err != nil {
		return nil, err
	}
	return runtimeRelayNodeViews(items), nil
}

func (s RuntimeNodeRegistryService) UpsertRelayNode(ctx context.Context, input UpsertRuntimeNodeInput) (RuntimeRelayNodeView, error) {
	input = normalizeUpsertRuntimeNodeInput(input)
	if input.Name == "" && input.NodeID == "" {
		return RuntimeRelayNodeView{}, ErrInvalidArgument
	}
	now := opsNow(s.Now).Unix()
	nodeID := runtimeNodeID(s.Nodes, input.NodeID, "relay")
	item := newRelayNode(nodeID, now, input)
	if current, ok, err := s.Nodes.GetRelayNode(ctx, item.NodeID); err != nil {
		return RuntimeRelayNodeView{}, err
	} else if ok {
		item = mergeRelayNode(current, input, now)
	}
	if err := validateRelayNodeModel(ctx, s.Nodes, item); err != nil {
		return RuntimeRelayNodeView{}, err
	}
	if err := s.Nodes.SaveRelayNode(ctx, item); err != nil {
		return RuntimeRelayNodeView{}, err
	}
	return runtimeRelayNodeViewFromModel(item), nil
}

func (s RuntimeNodeRegistryService) UpdateRelayNodeStatus(ctx context.Context, nodeID, status string) (RuntimeRelayNodeView, error) {
	item, ok, err := s.Nodes.GetRelayNode(ctx, normalizeNodeID(nodeID))
	if err != nil {
		return RuntimeRelayNodeView{}, err
	}
	if !ok {
		return RuntimeRelayNodeView{}, ErrNotFound
	}
	item = applyRelayNodeStatus(item, status, opsNow(s.Now).Unix())
	if err := s.Nodes.SaveRelayNode(ctx, item); err != nil {
		return RuntimeRelayNodeView{}, err
	}
	return runtimeRelayNodeViewFromModel(item), nil
}

func (s RuntimeNodeRegistryService) DeleteRelayNode(ctx context.Context, nodeID string) error {
	return s.Nodes.DeleteRelayNode(ctx, normalizeNodeID(nodeID))
}

func (s RuntimeNodeRegistryService) ListPunchNodes(ctx context.Context) ([]RuntimePunchNodeView, error) {
	items, err := s.Nodes.ListPunchNodes(ctx)
	if err != nil {
		return nil, err
	}
	return runtimePunchNodeViews(items), nil
}

func (s RuntimeNodeRegistryService) UpsertPunchNode(ctx context.Context, input UpsertRuntimeNodeInput) (RuntimePunchNodeView, error) {
	input = normalizeUpsertRuntimeNodeInput(input)
	if input.Name == "" && input.NodeID == "" {
		return RuntimePunchNodeView{}, ErrInvalidArgument
	}
	now := opsNow(s.Now).Unix()
	item := newPunchNode(runtimeNodeID(s.Nodes, input.NodeID, "punch"), now, input)
	if current, ok, err := s.Nodes.GetPunchNode(ctx, item.NodeID); err != nil {
		return RuntimePunchNodeView{}, err
	} else if ok {
		item = mergePunchNode(current, input, now)
	}
	if err := validatePunchNodeModel(ctx, s.Nodes, item); err != nil {
		return RuntimePunchNodeView{}, err
	}
	if err := s.Nodes.SavePunchNode(ctx, item); err != nil {
		return RuntimePunchNodeView{}, err
	}
	return runtimePunchNodeViewFromModel(item), nil
}

func (s RuntimeNodeRegistryService) UpdatePunchNodeStatus(ctx context.Context, nodeID, status string) (RuntimePunchNodeView, error) {
	item, ok, err := s.Nodes.GetPunchNode(ctx, normalizeNodeID(nodeID))
	if err != nil {
		return RuntimePunchNodeView{}, err
	}
	if !ok {
		return RuntimePunchNodeView{}, ErrNotFound
	}
	item = applyPunchNodeStatus(item, status, opsNow(s.Now).Unix())
	if err := s.Nodes.SavePunchNode(ctx, item); err != nil {
		return RuntimePunchNodeView{}, err
	}
	return runtimePunchNodeViewFromModel(item), nil
}

func (s RuntimeNodeRegistryService) DeletePunchNode(ctx context.Context, nodeID string) error {
	return s.Nodes.DeletePunchNode(ctx, normalizeNodeID(nodeID))
}
