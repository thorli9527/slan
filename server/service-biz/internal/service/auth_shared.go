package service

import (
	"context"
	"time"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/pkg/mqttkit"
	"github.com/slan/service-biz/internal/repository"
)

type AuthUserService struct {
	Registration AuthUserRegistrationUseCase
	Sessions     AuthUserSessionUseCase
	Accounts     AuthUserAccountUseCase
	Entitlements AuthUserEntitlementUseCase
}

type authUserDependencies struct {
	Users              repository.UserRepository
	Sessions           repository.UserSessionRepository
	Devices            repository.DeviceRepository
	Networks           repository.NetworkRepository
	NewUserID          func() string
	NewNetID           func() string
	NewSessID          func(string) string
	NewSecurityGroupID func() string
	Now                func() time.Time
}

type AuthUserRegistrationService struct {
	authUserDependencies
}

type AuthUserSessionService struct {
	authUserDependencies
}

type AuthUserAccountService struct {
	authUserDependencies
}

type AuthUserEntitlementService struct {
	authUserDependencies
}

type AuthAliasService struct {
	Users   repository.UserRepository
	Aliases repository.UserAliasRepository
	Now     func() time.Time
}

type AuthConsoleService struct {
	Keys  AuthConsoleKeyUseCase
	Login AuthConsoleLoginUseCase
}

type authConsoleDependencies struct {
	Users     repository.UserRepository
	Sessions  repository.UserSessionRepository
	NewSessID func(string) string
	Now       func() time.Time
}

type AuthConsoleKeyService struct {
	authConsoleDependencies
}

type AuthConsoleLoginService struct {
	authConsoleDependencies
}

type AuthDeviceLoginService struct {
	Prepare  AuthDeviceLoginPrepareUseCase
	Complete AuthDeviceLoginCompleteUseCase
}

type authDeviceLoginDependencies struct {
	Users    repository.UserRepository
	Devices  repository.DeviceRepository
	Networks repository.NetworkRepository
	MQTT     mqttkit.Config
	Now      func() time.Time
}

type AuthDeviceLoginPrepareService struct {
	authDeviceLoginDependencies
}

type AuthDeviceLoginCompleteService struct {
	authDeviceLoginDependencies
}

func NewAuthUserService(
	users repository.UserRepository,
	sessions repository.UserSessionRepository,
	devices repository.DeviceRepository,
	networks repository.NetworkRepository,
	newUserID func() string,
	newNetworkID func() string,
	newSessionID func(string) string,
	newSecurityGroupID func() string,
	now func() time.Time,
) AuthUserService {
	deps := authUserDependencies{
		Users:              users,
		Sessions:           sessions,
		Devices:            devices,
		Networks:           networks,
		NewUserID:          newUserID,
		NewNetID:           newNetworkID,
		NewSessID:          newSessionID,
		NewSecurityGroupID: newSecurityGroupID,
		Now:                now,
	}
	return AuthUserService{
		Registration: AuthUserRegistrationService{authUserDependencies: deps},
		Sessions:     AuthUserSessionService{authUserDependencies: deps},
		Accounts:     AuthUserAccountService{authUserDependencies: deps},
		Entitlements: AuthUserEntitlementService{authUserDependencies: deps},
	}
}

func NewAuthConsoleService(
	users repository.UserRepository,
	sessions repository.UserSessionRepository,
	newSessionID func(string) string,
	now func() time.Time,
) AuthConsoleService {
	deps := authConsoleDependencies{
		Users:     users,
		Sessions:  sessions,
		NewSessID: newSessionID,
		Now:       now,
	}
	return AuthConsoleService{
		Keys:  AuthConsoleKeyService{authConsoleDependencies: deps},
		Login: AuthConsoleLoginService{authConsoleDependencies: deps},
	}
}

func NewAuthDeviceLoginService(
	users repository.UserRepository,
	devices repository.DeviceRepository,
	networks repository.NetworkRepository,
	mqtt mqttkit.Config,
	now func() time.Time,
) AuthDeviceLoginService {
	deps := authDeviceLoginDependencies{
		Users:    users,
		Devices:  devices,
		Networks: networks,
		MQTT:     mqtt,
		Now:      now,
	}
	return AuthDeviceLoginService{
		Prepare:  AuthDeviceLoginPrepareService{authDeviceLoginDependencies: deps},
		Complete: AuthDeviceLoginCompleteService{authDeviceLoginDependencies: deps},
	}
}

func (s AuthUserService) RegisterUser(ctx context.Context, input RegisterUserInput) (AuthSessionView, error) {
	return s.Registration.RegisterUser(ctx, input)
}

func (s AuthUserService) LoginUser(ctx context.Context, input LoginUserInput) (AuthSessionView, error) {
	return s.Sessions.LoginUser(ctx, input)
}

func (s AuthUserService) RenewUserSession(ctx context.Context, accessToken string) (AuthSessionView, error) {
	return s.Sessions.RenewUserSession(ctx, accessToken)
}

func (s AuthUserService) LogoutUser(ctx context.Context, accessToken string, input LogoutUserInput) error {
	return s.Sessions.LogoutUser(ctx, accessToken, input)
}

func (s AuthUserService) GetUser(ctx context.Context, userID string) (UserView, error) {
	return s.Accounts.GetUser(ctx, userID)
}

func (s AuthUserService) ListUsers(ctx context.Context) ([]UserSummaryView, error) {
	return s.Accounts.ListUsers(ctx)
}

func (s AuthUserService) ChangeUserPassword(ctx context.Context, input ChangeUserPasswordInput) (ChangedUserPasswordView, error) {
	return s.Accounts.ChangeUserPassword(ctx, input)
}

func (s AuthUserService) UserEntitlement(ctx context.Context, userID string) (UserEntitlementView, error) {
	return s.Entitlements.UserEntitlement(ctx, userID)
}

func (s AuthConsoleService) CreateConsoleLoginKey(ctx context.Context, input CreateConsoleLoginKeyInput) (ConsoleLoginKeyView, error) {
	return s.Keys.CreateConsoleLoginKey(ctx, input)
}

func (s AuthConsoleService) ConsoleLogin(ctx context.Context, input ConsoleLoginInput) (AuthSessionView, error) {
	return s.Login.ConsoleLogin(ctx, input)
}

func (s AuthDeviceLoginService) PrepareDeviceLoginDevice(ctx context.Context, input PrepareDeviceLoginDeviceInput) (PrepareDeviceLoginDeviceView, error) {
	return s.Prepare.PrepareDeviceLoginDevice(ctx, input)
}

func (s AuthDeviceLoginService) CompleteDeviceLoginDevice(ctx context.Context, input CompleteDeviceLoginDeviceInput) (CompleteDeviceLoginDeviceView, error) {
	return s.Complete.CompleteDeviceLoginDevice(ctx, input)
}

func authNow(now func() time.Time) time.Time {
	return currentTime(now)
}

func newAuthUserID(next func() string) string {
	return generatedID(next, "user")
}

func newAuthSessionID(next func(string) string) string {
	return scopedID(next, "sess")
}

func newAuthNetworkID(next func() string) string {
	return generatedID(next, "net")
}

func newAuthDeviceID(devices repository.DeviceRepository) string {
	return repositoryID[deviceIDProvider](devices, "device", func(provider deviceIDProvider) string {
		return provider.NewDeviceID()
	})
}

func newAuthUserSession(now time.Time, next func(string) string, userID string) (model.UserSession, error) {
	access, err := randomHex(24)
	if err != nil {
		return model.UserSession{}, err
	}
	refresh, err := randomHex(24)
	if err != nil {
		return model.UserSession{}, err
	}
	return model.UserSession{
		SessionID:     newAuthSessionID(next),
		UserID:        userID,
		AccessToken:   access,
		RefreshToken:  refresh,
		ExpiresAt:     now.Add(24 * time.Hour).Unix(),
		RefreshExpiry: now.Add(7 * 24 * time.Hour).Unix(),
		CreatedAt:     now.Unix(),
	}, nil
}

func requireAuthUser(ctx context.Context, users repository.UserRepository, userID string) (model.User, error) {
	user, ok, err := users.GetUser(ctx, userID)
	if err != nil {
		return model.User{}, err
	}
	if !ok {
		return model.User{}, ErrNotFound
	}
	return user, nil
}
