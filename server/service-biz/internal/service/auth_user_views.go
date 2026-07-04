package service

import "github.com/slan/service-biz/internal/model"

func userView(item model.User) UserView {
	return UserView{
		UserID:    item.UserID,
		Email:     item.Email,
		Name:      item.Name,
		Status:    item.Status,
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}
}

func userSessionView(item model.UserSession) UserSessionView {
	return UserSessionView{
		SessionID:     item.SessionID,
		UserID:        item.UserID,
		AccessToken:   item.AccessToken,
		RefreshToken:  item.RefreshToken,
		Status:        item.Status,
		SessionMode:   item.SessionMode,
		ExpiresAt:     item.ExpiresAt,
		RefreshExpiry: item.RefreshExpiry,
		CreatedAt:     item.CreatedAt,
		UpdatedAt:     item.UpdatedAt,
		RevokedAt:     item.RevokedAt,
	}
}

func authSessionView(user model.User, session model.UserSession) AuthSessionView {
	return AuthSessionView{
		User:    userView(user),
		Session: userSessionView(session),
	}
}

func userSummaryView(item model.User) UserSummaryView {
	return UserSummaryView{User: userView(item)}
}

func changedUserPasswordView(item model.User) ChangedUserPasswordView {
	return ChangedUserPasswordView{
		User:      userView(item),
		UpdatedAt: item.UpdatedAt,
	}
}
