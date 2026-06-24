package web

import (
	"net/http"
	"strconv"

	serviceapi "github.com/slan/service-biz/internal/api"
)

func firstNonEmpty(values ...string) string {
	return serviceapi.FirstNonEmpty(values...)
}

func atoi(value string) int {
	n, _ := strconv.Atoi(value)
	return n
}

func requestUserID(r *http.Request) string {
	return serviceapi.PathOrQuery(r, "userId", "userId")
}

func requestOwnerID(r *http.Request) string {
	return serviceapi.FirstNonEmpty(
		serviceapi.PathOrQuery(r, "userId", "ownerUserId"),
		serviceapi.PathOrQuery(r, "ownerId", "ownerId"),
	)
}

func requestActorUserID(r *http.Request) string {
	return serviceapi.PathOrQuery(r, "actorUserId", "actorUserId")
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

func requestMappingID(r *http.Request) string {
	return serviceapi.PathOrQuery(r, "mappingId", "mappingId")
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

func setOwnerAndActor(r *http.Request, ownerID *string, actorUserID *string) {
	serviceapi.SetIfEmpty(ownerID, requestOwnerID(r))
	serviceapi.SetIfEmpty(actorUserID, requestActorUserID(r))
}

func setUserAndActor(r *http.Request, userID *string, actorUserID *string) {
	serviceapi.SetIfEmpty(userID, requestUserID(r))
	serviceapi.SetIfEmpty(actorUserID, requestActorUserID(r))
}

func setDeviceAndActor(r *http.Request, deviceID *string, actorUserID *string) {
	serviceapi.SetIfEmpty(deviceID, requestDeviceID(r))
	serviceapi.SetIfEmpty(actorUserID, requestActorUserID(r))
}

func setNetworkAndActor(r *http.Request, networkID *string, actorUserID *string) {
	serviceapi.SetIfEmpty(networkID, requestNetworkID(r))
	serviceapi.SetIfEmpty(actorUserID, requestActorUserID(r))
}

func setZoneAndActor(r *http.Request, zoneID *string, actorUserID *string) {
	serviceapi.SetIfEmpty(zoneID, requestZoneID(r))
	serviceapi.SetIfEmpty(actorUserID, requestActorUserID(r))
}

func setRecordAndActor(r *http.Request, recordID *string, actorUserID *string) {
	serviceapi.SetIfEmpty(recordID, requestRecordID(r))
	serviceapi.SetIfEmpty(actorUserID, requestActorUserID(r))
}

func setMappingAndActor(r *http.Request, mappingID *string, actorUserID *string) {
	serviceapi.SetIfEmpty(mappingID, requestMappingID(r))
	serviceapi.SetIfEmpty(actorUserID, requestActorUserID(r))
}

func setSecurityGroupAndActor(r *http.Request, securityGroupID *string, actorUserID *string) {
	serviceapi.SetIfEmpty(securityGroupID, requestSecurityGroupID(r))
	serviceapi.SetIfEmpty(actorUserID, requestActorUserID(r))
}

func setRuleAndActor(r *http.Request, ruleID *string, actorUserID *string) {
	serviceapi.SetIfEmpty(ruleID, requestRuleID(r))
	serviceapi.SetIfEmpty(actorUserID, requestActorUserID(r))
}

func setGroupAndActor(r *http.Request, groupID *string, actorUserID *string) {
	serviceapi.SetIfEmpty(groupID, requestGroupID(r))
	serviceapi.SetIfEmpty(actorUserID, requestActorUserID(r))
}
