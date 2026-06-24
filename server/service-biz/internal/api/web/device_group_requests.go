package web

import servicepkg "github.com/slan/service-biz/internal/service"

type namedDeviceRequest struct {
	Name string `json:"name"`
}

func (r namedDeviceRequest) value(fallback ...string) string {
	return firstNonEmpty(append([]string{r.Name}, fallback...)...)
}

type deviceGroupNameFields struct {
	UserID      string `json:"userId"`
	GroupID     string `json:"groupId"`
	ActorUserID string `json:"actorUserId"`
	Description string `json:"description"`
	namedDeviceRequest
}

type createDeviceGroupRequest struct {
	deviceGroupNameFields
}

func (r createDeviceGroupRequest) toInput() servicepkg.CreateDeviceGroupInput {
	return servicepkg.CreateDeviceGroupInput{UserID: r.UserID, ActorUserID: r.ActorUserID, Name: r.Name, Description: r.Description}
}

type updateDeviceGroupRequest struct {
	deviceGroupNameFields
}

func (r updateDeviceGroupRequest) toInput() servicepkg.UpdateDeviceGroupInput {
	return servicepkg.UpdateDeviceGroupInput{GroupID: r.GroupID, ActorUserID: r.ActorUserID, Name: r.Name, Description: r.Description}
}

type setDeviceGroupsRequest struct {
	UserID      string   `json:"userId"`
	ActorUserID string   `json:"actorUserId"`
	DeviceID    string   `json:"deviceId"`
	GroupIDs    []string `json:"groupIds"`
}

func (r setDeviceGroupsRequest) toInput() servicepkg.SetDeviceGroupsInput {
	return servicepkg.SetDeviceGroupsInput{
		UserID:      r.UserID,
		ActorUserID: r.ActorUserID,
		DeviceID:    r.DeviceID,
		GroupIDs:    r.GroupIDs,
	}
}
