package web

import servicepkg "github.com/slan/service-biz/internal/service"

type upsertUserAliasRequest struct {
	UserID      string `json:"userId"`
	OwnerUserID string `json:"ownerUserId"`
	ActorUserID string `json:"actorUserId"`
	Email       string `json:"email"`
	Alias       string `json:"alias"`
}

func (r upsertUserAliasRequest) toInput() servicepkg.UpsertUserAliasInput {
	return servicepkg.UpsertUserAliasInput{
		UserID:      firstNonEmpty(r.UserID, r.OwnerUserID),
		ActorUserID: r.ActorUserID,
		Email:       r.Email,
		Alias:       r.Alias,
	}
}
