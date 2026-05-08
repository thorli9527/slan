package impl

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/netpath"
	"github.com/slan/server/server-biz/internal/repo"
	"github.com/slan/server/server-biz/internal/util"
)

func (s dbNetworkService) ListRelayPolicyTemplates(userID, networkID string) ([]dto.RelayPolicyTemplate, error) {
	if err := s.requireNetworkOwner(context.Background(), userID, networkID); err != nil {
		return nil, err
	}
	records, err := s.state.pg.ListRelayPolicyTemplates(context.Background(), networkID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.RelayPolicyTemplate, 0, len(records))
	for _, record := range records {
		out = append(out, relayPolicyTemplateDTO(record))
	}
	return out, nil
}

func (s dbNetworkService) CreateRelayPolicyTemplate(userID, networkID string, req dto.RelayPolicyTemplateRequest) (dto.RelayPolicyTemplate, error) {
	if err := s.requireNetworkOwner(context.Background(), userID, networkID); err != nil {
		return dto.RelayPolicyTemplate{}, err
	}
	record, err := relayPolicyTemplateRecord(util.NewID("relay-template"), networkID, req, false, time.Now().Unix(), 0)
	if err != nil {
		return dto.RelayPolicyTemplate{}, err
	}
	if err := s.state.pg.CreateRelayPolicyTemplate(context.Background(), record); err != nil {
		if repo.IsUniqueViolation(err) {
			return dto.RelayPolicyTemplate{}, ErrConflict
		}
		return dto.RelayPolicyTemplate{}, err
	}
	return relayPolicyTemplateDTO(record), nil
}

func (s dbNetworkService) UpdateRelayPolicyTemplate(userID, networkID, templateID string, req dto.RelayPolicyTemplateRequest) (dto.RelayPolicyTemplate, error) {
	if err := s.requireNetworkOwner(context.Background(), userID, networkID); err != nil {
		return dto.RelayPolicyTemplate{}, err
	}
	existing, err := s.state.pg.GetRelayPolicyTemplate(context.Background(), strings.TrimSpace(templateID))
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.RelayPolicyTemplate{}, ErrNotFound
		}
		return dto.RelayPolicyTemplate{}, err
	}
	if existing.NetworkID != networkID || existing.IsBuiltin {
		return dto.RelayPolicyTemplate{}, ErrForbidden
	}
	record, err := relayPolicyTemplateRecord(existing.TemplateID, networkID, req, false, existing.CreatedAt, time.Now().Unix())
	if err != nil {
		return dto.RelayPolicyTemplate{}, err
	}
	if err := s.state.pg.UpdateRelayPolicyTemplate(context.Background(), record); err != nil {
		if repo.IsNotFound(err) {
			return dto.RelayPolicyTemplate{}, ErrNotFound
		}
		if repo.IsUniqueViolation(err) {
			return dto.RelayPolicyTemplate{}, ErrConflict
		}
		return dto.RelayPolicyTemplate{}, err
	}
	return relayPolicyTemplateDTO(record), nil
}

func (s dbNetworkService) DeleteRelayPolicyTemplate(userID, networkID, templateID string) error {
	if err := s.requireNetworkOwner(context.Background(), userID, networkID); err != nil {
		return err
	}
	existing, err := s.state.pg.GetRelayPolicyTemplate(context.Background(), strings.TrimSpace(templateID))
	if err != nil {
		if repo.IsNotFound(err) {
			return ErrNotFound
		}
		return err
	}
	if existing.NetworkID != networkID || existing.IsBuiltin {
		return ErrForbidden
	}
	if err := s.state.pg.DeleteRelayPolicyTemplate(context.Background(), existing.TemplateID); err != nil {
		if repo.IsNotFound(err) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

func (s dbNetworkService) RecordRelayPolicyDispatch(userID, networkID string, req dto.OpsRelayDataPlanePolicyRequest, resp dto.OpsRelayDataPlanePolicyResponse) error {
	if err := s.requireNetworkOwner(context.Background(), userID, networkID); err != nil {
		return err
	}
	templateName := strings.TrimSpace(req.TemplateName)
	templateID := strings.TrimSpace(req.TemplateID)
	if templateID != "" && templateName == "" {
		if template, err := s.state.pg.GetRelayPolicyTemplate(context.Background(), templateID); err == nil {
			templateName = template.Name
		}
	}
	if err := s.state.pg.DeleteRelayPolicyExecutions(context.Background(), networkID, resp.TargetDeviceIDs); err != nil {
		return err
	}
	now := time.Now()
	for _, deviceID := range resp.TargetDeviceIDs {
		deviceID = strings.TrimSpace(deviceID)
		if deviceID == "" {
			continue
		}
		record := repo.RelayPolicyExecution{
			ExecutionID:       util.NewID("relay-exec"),
			NetworkID:         networkID,
			DeviceID:          deviceID,
			PolicyID:          resp.PolicyID,
			TemplateID:        templateID,
			TemplateName:      templateName,
			Scope:             resp.Scope,
			RelayMtu:          resp.RelayMtu,
			MaxFramePayload:   resp.MaxFramePayload,
			ExecutionLevel:    req.ExecutionLevel,
			Applied:           false,
			Reason:            strings.TrimSpace(resp.Reason),
			PolicyUpdatedAtMs: uint64(now.UnixMilli()),
			ReportedAtMs:      0,
			UpdatedAt:         now.Unix(),
		}
		if err := s.state.pg.UpsertRelayPolicyExecution(context.Background(), record); err != nil {
			return err
		}
	}
	return nil
}

func (s dbNetworkService) ListEnabledNetworkDeviceIDs(networkID string) ([]string, error) {
	states, err := s.state.tokens.ListEnabledNetworkMembers(context.Background(), strings.TrimSpace(networkID))
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(states))
	out := make([]string, 0, len(states))
	for _, state := range states {
		if strings.TrimSpace(state.NetworkID) != strings.TrimSpace(networkID) {
			continue
		}
		deviceID := strings.TrimSpace(state.DeviceID)
		if deviceID == "" || seen[deviceID] {
			continue
		}
		seen[deviceID] = true
		out = append(out, deviceID)
	}
	sort.Strings(out)
	return out, nil
}

func (s dbNetworkService) requireNetworkOwner(ctx context.Context, userID, networkID string) error {
	record, err := s.state.pg.GetNetworkByID(ctx, strings.TrimSpace(networkID))
	if err != nil {
		if repo.IsNotFound(err) {
			return ErrNotFound
		}
		return err
	}
	if record.OwnerUserID != userID {
		return ErrForbidden
	}
	return nil
}

func relayPolicyTemplateRecord(templateID, networkID string, req dto.RelayPolicyTemplateRequest, builtin bool, createdAt, updatedAt int64) (repo.RelayPolicyTemplate, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" || req.RelayMtu < 576 || req.RelayMtu > 1500 || req.MaxFramePayload < 512 || req.MaxFramePayload > 1400 || req.MaxFramePayload >= req.RelayMtu || req.RecommendationLevel > 11 || req.ExecutionLevel > 7 || req.TTLMinutes == 0 {
		return repo.RelayPolicyTemplate{}, ErrInvalidArgument
	}
	pathType := netpath.NormalizePolicyPathType(req.PathType)
	if strings.TrimSpace(req.PathType) != "" && strings.TrimSpace(req.PathType) != "any" && pathType == "" {
		return repo.RelayPolicyTemplate{}, ErrInvalidArgument
	}
	pathStrategy := normalizeRelayPolicyTemplateStrategy(req.PathStrategy)
	if pathStrategy == "" {
		return repo.RelayPolicyTemplate{}, ErrInvalidArgument
	}
	now := time.Now().Unix()
	if createdAt == 0 {
		createdAt = now
	}
	if updatedAt == 0 {
		updatedAt = now
	}
	return repo.RelayPolicyTemplate{
		TemplateID:          strings.TrimSpace(templateID),
		NetworkID:           strings.TrimSpace(networkID),
		Name:                name,
		Description:         strings.TrimSpace(req.Description),
		PathType:            pathType,
		PathStrategy:        pathStrategy,
		RelayMtu:            req.RelayMtu,
		MaxFramePayload:     req.MaxFramePayload,
		RecommendationLevel: req.RecommendationLevel,
		ExecutionLevel:      req.ExecutionLevel,
		TTLMinutes:          req.TTLMinutes,
		Reason:              strings.TrimSpace(req.Reason),
		IsBuiltin:           builtin,
		CreatedAt:           createdAt,
		UpdatedAt:           updatedAt,
	}, nil
}

func normalizeRelayPolicyTemplateStrategy(value string) string {
	switch strings.TrimSpace(value) {
	case "bandwidth_saving", "":
		return "bandwidth_saving"
	case "performance":
		return "performance"
	case "relay_saving":
		return "relay_saving"
	default:
		return ""
	}
}

func relayPolicyTemplateDTO(record repo.RelayPolicyTemplate) dto.RelayPolicyTemplate {
	scope := "network"
	if record.NetworkID == "" {
		scope = "global"
	}
	pathType := record.PathType
	if pathType == "" {
		pathType = "any"
	}
	return dto.RelayPolicyTemplate{
		TemplateID:          record.TemplateID,
		NetworkID:           record.NetworkID,
		Scope:               scope,
		Name:                record.Name,
		Description:         record.Description,
		PathType:            pathType,
		PathStrategy:        record.PathStrategy,
		RelayMtu:            record.RelayMtu,
		MaxFramePayload:     record.MaxFramePayload,
		RecommendationLevel: record.RecommendationLevel,
		ExecutionLevel:      record.ExecutionLevel,
		TTLMinutes:          record.TTLMinutes,
		Reason:              record.Reason,
		IsBuiltin:           record.IsBuiltin,
		CreatedAt:           record.CreatedAt,
		UpdatedAt:           record.UpdatedAt,
	}
}
