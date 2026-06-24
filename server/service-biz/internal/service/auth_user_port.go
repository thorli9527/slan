package service

import "context"

type AuthUserRegistrationUseCase interface {
	RegisterUser(ctx context.Context, input RegisterUserInput) (AuthSessionView, error)
}

type AuthUserSessionUseCase interface {
	LoginUser(ctx context.Context, input LoginUserInput) (AuthSessionView, error)
	RenewUserSession(ctx context.Context, accessToken string) (AuthSessionView, error)
	LogoutUser(ctx context.Context, accessToken string, input LogoutUserInput) error
}

type AuthUserAccountUseCase interface {
	GetUser(ctx context.Context, userID string) (UserView, error)
	ListUsers(ctx context.Context) ([]UserSummaryView, error)
	ChangeUserPassword(ctx context.Context, input ChangeUserPasswordInput) (ChangedUserPasswordView, error)
}

type AuthUserEntitlementUseCase interface {
	UserEntitlement(ctx context.Context, userID string) (UserEntitlementView, error)
}

type AuthUserUseCase interface {
	AuthUserRegistrationUseCase
	AuthUserSessionUseCase
	AuthUserAccountUseCase
	AuthUserEntitlementUseCase
}
