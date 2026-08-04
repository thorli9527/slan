package service

import (
	"context"
	"strings"

	"github.com/slan/service-biz/internal/model"
)

func (s OpsOperatorPasswordService) SetOperatorPassword(ctx context.Context, input SetOperatorPasswordInput) (OpsOperatorView, error) {
	targetID := strings.TrimSpace(input.OperatorID)
	view, err := s.setOperatorPassword(ctx, input)
	s.recordOperatorPasswordAudit(ctx, "reset_password", targetID, auditStatus(err), opsNow(s.Now).Unix())
	return view, err
}

func (s OpsOperatorPasswordService) setOperatorPassword(ctx context.Context, input SetOperatorPasswordInput) (OpsOperatorView, error) {
	input = normalizeSetOperatorPasswordInput(input)
	if input.OperatorID == "" {
		return OpsOperatorView{}, ErrInvalidArgument
	}
	if err := validateOperatorPassword(input.Password); err != nil {
		return OpsOperatorView{}, invalidArgumentError(err.Error())
	}
	item, err := requireOpsOperator(ctx, s.Operators, input.OperatorID)
	if err != nil {
		return OpsOperatorView{}, err
	}
	passwordHash, err := hashPassword(input.Password)
	if err != nil {
		return OpsOperatorView{}, err
	}
	item.PasswordHash = passwordHash
	item.UpdatedAt = opsNow(s.Now).Unix()
	if err := s.Operators.SaveOperator(ctx, item); err != nil {
		return OpsOperatorView{}, err
	}
	if err := s.OperatorSessions.DeleteOperatorSessionsByOperatorID(ctx, item.OperatorID); err != nil {
		return OpsOperatorView{}, err
	}
	return opsOperatorView(item), nil
}

func (s OpsOperatorPasswordService) ChangePassword(ctx context.Context, input OpsChangePasswordInput) (OpsOperatorView, error) {
	targetID := strings.TrimSpace(input.OperatorID)
	view, err := s.changePassword(ctx, input)
	s.recordOperatorPasswordAudit(ctx, "change_password", targetID, auditStatus(err), opsNow(s.Now).Unix())
	return view, err
}

func (s OpsOperatorPasswordService) changePassword(ctx context.Context, input OpsChangePasswordInput) (OpsOperatorView, error) {
	input.OperatorID = strings.TrimSpace(input.OperatorID)
	if input.OperatorID == "" || normalizedSecret(input.OldPassword) == "" {
		return OpsOperatorView{}, ErrInvalidArgument
	}
	if err := validateOperatorPassword(input.Password); err != nil {
		return OpsOperatorView{}, invalidArgumentError(err.Error())
	}
	item, err := requireOpsOperator(ctx, s.Operators, input.OperatorID)
	if err != nil {
		return OpsOperatorView{}, err
	}
	if !verifyPassword(item.PasswordHash, input.OldPassword) {
		return OpsOperatorView{}, ErrUnauthorized
	}
	return s.setOperatorPassword(ctx, SetOperatorPasswordInput{OperatorID: input.OperatorID, Password: input.Password})
}

func auditStatus(err error) string {
	if err != nil {
		return "failure"
	}
	return "success"
}

func (s OpsOperatorPasswordService) recordOperatorPasswordAudit(ctx context.Context, action, targetID, status string, now int64) {
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
