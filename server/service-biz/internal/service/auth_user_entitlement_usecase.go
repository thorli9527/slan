package service

import "context"

func (s AuthUserEntitlementService) UserEntitlement(ctx context.Context, userID string) (UserEntitlementView, error) {
	userID = normalizeUserID(userID)
	if userID == "" {
		return UserEntitlementView{}, ErrInvalidArgument
	}
	if _, err := requireAuthUser(ctx, s.Users, userID); err != nil {
		return UserEntitlementView{}, err
	}
	devices, err := s.Devices.ListDevicesByOwner(ctx, userID)
	if err != nil {
		return UserEntitlementView{}, err
	}
	return userEntitlementView(userID, len(devices)), nil
}
