package web

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
)

func firstNonEmpty(values ...string) string {
	return serviceapi.FirstNonEmpty(values...)
}

func requestUserID(r *http.Request) string {
	return firstNonEmpty(serviceapi.AuthenticatedUserID(r.Context()), serviceapi.PathOrQuery(r, "userId", "userId"))
}

func requestOwnerID(r *http.Request) string {
	return serviceapi.FirstNonEmpty(
		serviceapi.AuthenticatedUserID(r.Context()),
		serviceapi.PathOrQuery(r, "userId", "ownerUserId"),
		serviceapi.PathOrQuery(r, "ownerId", "ownerId"),
	)
}

func requestActorUserID(r *http.Request) string {
	return firstNonEmpty(serviceapi.AuthenticatedUserID(r.Context()), serviceapi.PathOrQuery(r, "actorUserId", "actorUserId"))
}

func requestNetworkID(r *http.Request) string {
	return serviceapi.PathOrQuery(r, "networkId", "networkId")
}

func requestDeviceID(r *http.Request) string {
	return serviceapi.PathOrQuery(r, "deviceId", "deviceId")
}

func requestZoneID(r *http.Request) string {
	return serviceapi.PathOrQuery(r, "zoneId", "zoneId")
}

func requestRecordID(r *http.Request) string {
	return serviceapi.PathOrQuery(r, "recordId", "recordId")
}

func requestSecurityGroupID(r *http.Request) string {
	return serviceapi.PathOrQuery(r, "securityGroupId", "securityGroupId")
}

func requestRuleID(r *http.Request) string {
	return serviceapi.PathOrQuery(r, "ruleId", "ruleId")
}

func requestGroupID(r *http.Request) string {
	return serviceapi.PathOrQuery(r, "groupId", "groupId")
}

func requestBootstrapKeyID(r *http.Request) string {
	return serviceapi.PathOrQuery(r, "keyId", "keyId")
}

func requestInviteID(r *http.Request) string {
	return serviceapi.PathOrQuery(r, "inviteId", "inviteId")
}

func setOwnerAndActor(r *http.Request, ownerID *string, actorUserID *string) {
	serviceapi.SetIfEmpty(ownerID, requestOwnerID(r))
	setActorUserID(r, actorUserID)
}

func setUserAndActor(r *http.Request, userID *string, actorUserID *string) {
	serviceapi.SetIfEmpty(userID, requestUserID(r))
	setActorUserID(r, actorUserID)
}

func setDeviceAndActor(r *http.Request, deviceID *string, actorUserID *string) {
	serviceapi.SetIfEmpty(deviceID, requestDeviceID(r))
	setActorUserID(r, actorUserID)
}

func setNetworkAndActor(r *http.Request, networkID *string, actorUserID *string) {
	serviceapi.SetIfEmpty(networkID, requestNetworkID(r))
	setActorUserID(r, actorUserID)
}

func setZoneAndActor(r *http.Request, zoneID *string, actorUserID *string) {
	serviceapi.SetIfEmpty(zoneID, requestZoneID(r))
	setActorUserID(r, actorUserID)
}

func setRecordAndActor(r *http.Request, recordID *string, actorUserID *string) {
	serviceapi.SetIfEmpty(recordID, requestRecordID(r))
	setActorUserID(r, actorUserID)
}

func setSecurityGroupAndActor(r *http.Request, securityGroupID *string, actorUserID *string) {
	serviceapi.SetIfEmpty(securityGroupID, requestSecurityGroupID(r))
	setActorUserID(r, actorUserID)
}

func setRuleAndActor(r *http.Request, ruleID *string, actorUserID *string) {
	serviceapi.SetIfEmpty(ruleID, requestRuleID(r))
	setActorUserID(r, actorUserID)
}

func setGroupAndActor(r *http.Request, groupID *string, actorUserID *string) {
	serviceapi.SetIfEmpty(groupID, requestGroupID(r))
	setActorUserID(r, actorUserID)
}

func setActorUserID(r *http.Request, actorUserID *string) {
	if authenticated := serviceapi.AuthenticatedUserID(r.Context()); authenticated != "" {
		*actorUserID = authenticated
		return
	}
	serviceapi.SetIfEmpty(actorUserID, requestActorUserID(r))
}
