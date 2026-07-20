package service

import (
	"context"

	"github.com/slan/service-biz/internal/repository"
)

func (s AuthUserRegistrationService) RegisterUser(ctx context.Context, input RegisterUserInput) (AuthSessionView, error) {
	input = normalizeRegisterUserInput(input)
	if input.Email == "" || normalizedSecret(input.Password) == "" {
		return AuthSessionView{}, ErrInvalidArgument
	}
	if _, ok, _ := s.Users.GetByEmail(ctx, input.Email); ok {
		return AuthSessionView{}, ErrConflict
	}
	if s.Devices == nil {
		return AuthSessionView{}, ErrNotImplemented
	}
	networkGroups, ok := s.Networks.(repository.NetworkDeviceGroupRepository)
	if !ok {
		return AuthSessionView{}, ErrNotImplemented
	}
	now := authNow(s.Now)
	user := newRegisteredUser(s.NewUserID, input, now.Unix())
	if err := s.Users.SaveUser(ctx, user); err != nil {
		return AuthSessionView{}, err
	}
	network := newDefaultUserNetwork(s.NewNetID, user.UserID, now.Unix())
	if err := s.Networks.SaveNetwork(ctx, network); err != nil {
		return AuthSessionView{}, err
	}
	deviceGroup := newDefaultUserDeviceGroup(s.Devices, user.UserID, now.Unix())
	if err := s.Devices.SaveDeviceGroup(ctx, deviceGroup); err != nil {
		return AuthSessionView{}, err
	}
	if err := networkGroups.SaveNetworkDeviceGroupReference(
		ctx,
		newDefaultUserNetworkDeviceGroupReference(network.NetworkID, deviceGroup.GroupID, now.Unix()),
	); err != nil {
		return AuthSessionView{}, err
	}
	securityGroup := newDefaultUserSecurityGroup(s.NewSecurityGroupID, s.Networks, network.NetworkID, now.Unix())
	if err := s.Networks.SaveSecurityGroup(ctx, securityGroup); err != nil {
		return AuthSessionView{}, err
	}
	for _, rule := range newDefaultUserSecurityRules(s.Networks, securityGroup.SecurityGroupID, deviceGroup.GroupID, now.Unix()) {
		if err := s.Networks.SaveSecurityRule(ctx, rule); err != nil {
			return AuthSessionView{}, err
		}
	}
	if _, err := bumpNetworkConfigVersion(ctx, s.Networks, nil, s.Now, network.NetworkID, "user_default_network_created"); err != nil {
		return AuthSessionView{}, err
	}
	session, err := newAuthUserSession(now, s.NewSessID, user.UserID, tokenModeShort)
	if err != nil {
		return AuthSessionView{}, err
	}
	if err := s.Sessions.SaveUserSession(ctx, session); err != nil {
		return AuthSessionView{}, err
	}
	return authSessionView(user, session), nil
}
