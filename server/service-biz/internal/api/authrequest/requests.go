package authrequest

import servicepkg "github.com/slan/service-biz/internal/service"

type RegisterUser struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
	DeviceID string `json:"deviceId"`
}

func (r RegisterUser) ToInput() servicepkg.RegisterUserInput {
	return servicepkg.RegisterUserInput{
		Email:    r.Email,
		Password: r.Password,
		Name:     r.Name,
		DeviceID: r.DeviceID,
	}
}

type LoginUser struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	SessionMode string `json:"sessionMode"`
	DeviceID    string `json:"deviceId"`
}

func (r LoginUser) ToInput() servicepkg.LoginUserInput {
	return servicepkg.LoginUserInput{
		Email:       r.Email,
		Password:    r.Password,
		SessionMode: r.SessionMode,
		DeviceID:    r.DeviceID,
	}
}

type LogoutUser struct {
	DeviceToken string `json:"deviceToken"`
}

func (r LogoutUser) ToInput() servicepkg.LogoutUserInput {
	return servicepkg.LogoutUserInput{
		DeviceToken: r.DeviceToken,
	}
}

type RenewUserSession struct {
	RefreshToken string `json:"refreshToken"`
}

func (r RenewUserSession) ToInput() servicepkg.RenewUserSessionInput {
	return servicepkg.RenewUserSessionInput{
		RefreshToken: r.RefreshToken,
	}
}

type CreateConsoleLoginKey struct {
	UserID string `json:"userId"`
}

func (r CreateConsoleLoginKey) ToInput() servicepkg.CreateConsoleLoginKeyInput {
	return servicepkg.CreateConsoleLoginKeyInput{
		UserID: r.UserID,
	}
}

type ConsoleLogin struct {
	Key      string `json:"key"`
	LoginKey string `json:"loginKey"`
}

func (r ConsoleLogin) ToInput() servicepkg.ConsoleLoginInput {
	return servicepkg.ConsoleLoginInput{
		Key:      r.Key,
		LoginKey: r.LoginKey,
	}
}

type PrepareDeviceLoginDevice struct {
	UserID        string `json:"userId"`
	DeviceID      string `json:"deviceId"`
	Name          string `json:"name"`
	Platform      string `json:"platform"`
	Alias         string `json:"alias"`
	OSName        string `json:"osName"`
	OSVersion     string `json:"osVersion"`
	PublicKey     string `json:"publicKey"`
	DeviceVersion string `json:"deviceVersion"`
	CountryCode   string `json:"countryCode"`
}

func (r PrepareDeviceLoginDevice) ToInput() servicepkg.PrepareDeviceLoginDeviceInput {
	return servicepkg.PrepareDeviceLoginDeviceInput{
		UserID:        r.UserID,
		DeviceID:      r.DeviceID,
		Name:          r.Name,
		Platform:      r.Platform,
		Alias:         r.Alias,
		OSName:        r.OSName,
		OSVersion:     r.OSVersion,
		PublicKey:     r.PublicKey,
		DeviceVersion: r.DeviceVersion,
		CountryCode:   r.CountryCode,
	}
}

type CompleteDeviceLoginDevice struct {
	AccessToken   string `json:"accessToken"`
	DeviceID      string `json:"deviceId"`
	UserID        string `json:"userId"`
	Name          string `json:"name"`
	Platform      string `json:"platform"`
	Alias         string `json:"alias"`
	OSName        string `json:"osName"`
	OSVersion     string `json:"osVersion"`
	PublicKey     string `json:"publicKey"`
	DeviceVersion string `json:"deviceVersion"`
	CountryCode   string `json:"countryCode"`
}

func (r CompleteDeviceLoginDevice) ToInput() servicepkg.CompleteDeviceLoginDeviceInput {
	return servicepkg.CompleteDeviceLoginDeviceInput{
		AccessToken:   r.AccessToken,
		DeviceID:      r.DeviceID,
		UserID:        r.UserID,
		Name:          r.Name,
		Platform:      r.Platform,
		Alias:         r.Alias,
		OSName:        r.OSName,
		OSVersion:     r.OSVersion,
		PublicKey:     r.PublicKey,
		DeviceVersion: r.DeviceVersion,
		CountryCode:   r.CountryCode,
	}
}

type ChangeUserPassword struct {
	UserID      string `json:"userId"`
	ActorUserID string `json:"actorUserId"`
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

func (r ChangeUserPassword) ToInput() servicepkg.ChangeUserPasswordInput {
	return servicepkg.ChangeUserPasswordInput{
		UserID:      r.UserID,
		ActorUserID: r.ActorUserID,
		OldPassword: r.OldPassword,
		NewPassword: r.NewPassword,
	}
}
