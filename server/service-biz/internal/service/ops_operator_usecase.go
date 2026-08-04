package service

import (
	"context"
	"strings"

	"github.com/slan/service-biz/internal/model"
)

func (s OpsOperatorService) ListOperators(ctx context.Context) ([]OpsOperatorView, error) {
	items, err := s.Operators.ListOperators(ctx)
	if err != nil {
		return nil, err
	}
	return opsOperatorViews(items), nil
}

func (s OpsOperatorService) CreateOperator(ctx context.Context, input CreateOperatorInput) (OpsOperatorView, error) {
	targetID := normalizedEmail(input.Email)
	view, err := s.createOperator(ctx, input)
	if err == nil {
		targetID = view.Operator.OperatorID
	}
	s.recordOperatorManagementAudit(ctx, "create_operator", targetID, auditStatus(err), opsNow(s.Now).Unix())
	return view, err
}

func (s OpsOperatorService) createOperator(ctx context.Context, input CreateOperatorInput) (OpsOperatorView, error) {
	input = normalizeCreateOperatorInput(input)
	if input.Role == "" {
		input.Role = "operator"
	}
	if input.Email == "" {
		return OpsOperatorView{}, ErrInvalidArgument
	}
	if !validOperatorRole(input.Role) {
		return OpsOperatorView{}, ErrInvalidArgument
	}
	if err := validateOperatorPassword(input.Password); err != nil {
		return OpsOperatorView{}, invalidArgumentError(err.Error())
	}
	if _, ok, _ := s.Operators.GetOperatorByEmail(ctx, input.Email); ok {
		return OpsOperatorView{}, ErrConflict
	}
	now := opsNow(s.Now).Unix()
	item, err := newOpsOperator(newOpsOperatorID(s.Operators), input, now)
	if err != nil {
		return OpsOperatorView{}, err
	}
	if err := s.Operators.SaveOperator(ctx, item); err != nil {
		if existing, found, lookupErr := s.Operators.GetOperatorByEmail(ctx, input.Email); lookupErr == nil && found && existing.OperatorID != item.OperatorID {
			return OpsOperatorView{}, ErrConflict
		}
		return OpsOperatorView{}, err
	}
	return opsOperatorView(item), nil
}

func (s OpsOperatorService) UpdateOperator(ctx context.Context, input UpdateOperatorInput) (OpsOperatorView, error) {
	targetID := strings.TrimSpace(input.OperatorID)
	view, err := s.updateOperator(ctx, input)
	s.recordOperatorManagementAudit(ctx, "update_operator", targetID, auditStatus(err), opsNow(s.Now).Unix())
	return view, err
}

func (s OpsOperatorService) updateOperator(ctx context.Context, input UpdateOperatorInput) (OpsOperatorView, error) {
	input = normalizeUpdateOperatorInput(input)
	if input.OperatorID == "" {
		return OpsOperatorView{}, ErrInvalidArgument
	}
	if (input.Role != "" && !validOperatorRole(input.Role)) || (input.Status != "" && !validOperatorStatus(input.Status)) {
		return OpsOperatorView{}, ErrInvalidArgument
	}
	item, err := requireOpsOperator(ctx, s.Operators, input.OperatorID)
	if err != nil {
		return OpsOperatorView{}, err
	}
	wasActive := item.Status == "active"
	updated := applyUpdateOperatorInput(item, input, opsNow(s.Now).Unix())
	if item.Role == "admin" && item.Status == "active" && (updated.Role != "admin" || updated.Status != "active") {
		items, err := s.Operators.ListOperators(ctx)
		if err != nil {
			return OpsOperatorView{}, err
		}
		activeAdmins := 0
		for _, operator := range items {
			if operator.Role == "admin" && operator.Status == "active" {
				activeAdmins++
			}
		}
		if activeAdmins <= 1 {
			return OpsOperatorView{}, conflictError("at least one active admin is required")
		}
	}
	item = updated
	if err := s.Operators.SaveOperator(ctx, item); err != nil {
		return OpsOperatorView{}, err
	}
	if wasActive && item.Status != "active" {
		if err := s.OperatorSessions.DeleteOperatorSessionsByOperatorID(ctx, item.OperatorID); err != nil {
			return OpsOperatorView{}, err
		}
	}
	return opsOperatorView(item), nil
}

func validOperatorRole(role string) bool {
	return role == "admin" || role == "operator"
}

func validOperatorStatus(status string) bool {
	return status == "active" || status == "disabled"
}

func (s OpsOperatorService) recordOperatorManagementAudit(ctx context.Context, action, targetID, status string, now int64) {
	if s.Audit == nil {
		return
	}
	eventID, err := randomHex(16)
	if err != nil {
		return
	}
	_ = s.Audit.SaveAuditEvent(ctx, model.AuditEvent{
		EventID: "audit_" + eventID, ActorType: "operator", ActorID: AuthenticatedOperatorID(ctx),
		Action: action, ResourceType: "operator", ResourceID: targetID, Status: status, CreatedAt: now,
	})
}
