package service

import (
	"context"
	"strings"
	"time"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

type OpsResourceService struct {
	Devices        repository.DeviceRepository
	Networks       repository.NetworkRepository
	NetworkGroups  repository.NetworkDeviceGroupRepository
	GroupRuntime   DeviceGroupService
	EventPublisher NetworkEventPublisher
	Ops            repository.OpsNodeRepository
	NewNetworkID   func() string
	Now            func() time.Time
}

func (s OpsResourceService) deviceGroupRuntime() DeviceGroupService {
	runtime := s.GroupRuntime
	if runtime.Devices == nil {
		runtime.Devices = s.Devices
	}
	if runtime.Networks == nil {
		runtime.Networks = s.Networks
	}
	if runtime.NetworkGroups == nil {
		runtime.NetworkGroups = s.NetworkGroups
	}
	if runtime.Now == nil {
		runtime.Now = s.Now
	}
	return runtime
}

type opsNetworkLister interface {
	ListNetworks(ctx context.Context) ([]model.Network, error)
}

func (s OpsResourceService) globalNetworks(ctx context.Context) ([]model.Network, error) {
	lister, ok := s.Networks.(opsNetworkLister)
	if !ok {
		return nil, ErrNotImplemented
	}
	return lister.ListNetworks(ctx)
}

func (s OpsResourceService) ListOpsNetworks(ctx context.Context) ([]OpsNetworkView, error) {
	items, err := s.globalNetworks(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]OpsNetworkView, 0, len(items))
	for _, item := range items {
		view, err := s.opsNetworkView(ctx, item)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

func (s OpsResourceService) CreateOpsNetwork(ctx context.Context, input OpsNetworkInput) (OpsNetworkView, error) {
	input = normalizeOpsNetworkInput(input)
	if input.Name == "" {
		return OpsNetworkView{}, ErrInvalidArgument
	}
	now := currentTime(s.Now).Unix()
	item := model.Network{NetworkID: newManagedNetworkID(s.NewNetworkID), Name: input.Name,
		IntraGroupPolicy: input.IntraGroupPolicy, Status: input.Status, CreatedAt: now, UpdatedAt: now}
	if item.IntraGroupPolicy == "" {
		item.IntraGroupPolicy = "allow"
	}
	if item.Status == "" {
		item.Status = "active"
	}
	if err := s.Networks.SaveNetwork(ctx, item); err != nil {
		return OpsNetworkView{}, err
	}
	version, err := bumpNetworkConfigVersion(ctx, s.Networks, s.EventPublisher, s.Now, item.NetworkID, "network_created")
	if err != nil {
		return OpsNetworkView{}, err
	}
	if err := publishNetworkSnapshot(ctx, s.Devices, s.Networks, s.Ops, s.EventPublisher, s.Now, item.NetworkID, version.Version, version.Reason); err != nil {
		return OpsNetworkView{}, err
	}
	return s.opsNetworkView(ctx, item)
}

func (s OpsResourceService) UpdateOpsNetwork(ctx context.Context, networkID string, input OpsNetworkInput) (OpsNetworkView, error) {
	item, err := requireManagedNetwork(ctx, s.Networks, strings.TrimSpace(networkID))
	if err != nil {
		return OpsNetworkView{}, err
	}
	input = normalizeOpsNetworkInput(input)
	if input.Name != "" {
		item.Name = input.Name
	}
	if input.IntraGroupPolicy != "" {
		item.IntraGroupPolicy = input.IntraGroupPolicy
	}
	if input.Status != "" {
		item.Status = input.Status
	}
	item.UpdatedAt = currentTime(s.Now).Unix()
	if err := s.Networks.SaveNetwork(ctx, item); err != nil {
		return OpsNetworkView{}, err
	}
	version, err := bumpNetworkConfigVersion(ctx, s.Networks, s.EventPublisher, s.Now, item.NetworkID, "network_updated")
	if err != nil {
		return OpsNetworkView{}, err
	}
	if err := publishNetworkSnapshot(ctx, s.Devices, s.Networks, s.Ops, s.EventPublisher, s.Now, item.NetworkID, version.Version, version.Reason); err != nil {
		return OpsNetworkView{}, err
	}
	return s.opsNetworkView(ctx, item)
}

func (s OpsResourceService) DeleteOpsNetwork(ctx context.Context, networkID string) error {
	if _, err := requireManagedNetwork(ctx, s.Networks, strings.TrimSpace(networkID)); err != nil {
		return err
	}
	return s.Networks.DeleteNetwork(ctx, strings.TrimSpace(networkID))
}

func (s OpsResourceService) ListOpsDeviceGroups(ctx context.Context) (OpsDeviceGroupCollectionView, error) {
	groups, err := s.Devices.ListDeviceGroups(ctx)
	if err != nil {
		return OpsDeviceGroupCollectionView{}, err
	}
	assignments, err := s.Devices.ListDeviceGroupAssignments(ctx)
	if err != nil {
		return OpsDeviceGroupCollectionView{}, err
	}
	view := OpsDeviceGroupCollectionView{Items: make([]DeviceGroupView, 0, len(groups)), Members: []DeviceGroupMemberView{}}
	for _, group := range groups {
		view.Items = append(view.Items, deviceGroupView(group))
	}
	for _, assignment := range assignments {
		for _, groupID := range assignment.GroupIDs {
			view.Members = append(view.Members, DeviceGroupMemberView{GroupID: groupID, DeviceID: assignment.DeviceID, AddedAt: assignment.UpdatedAt})
		}
	}
	return view, nil
}

func (s OpsResourceService) CreateOpsDeviceGroup(ctx context.Context, input OpsDeviceGroupInput) (DeviceGroupView, error) {
	return s.deviceGroupRuntime().CreateDeviceGroup(ctx, CreateDeviceGroupInput{
		Name: strings.TrimSpace(input.Name), Description: strings.TrimSpace(input.Description),
	})
}

func (s OpsResourceService) UpdateOpsDeviceGroup(ctx context.Context, groupID string, input OpsDeviceGroupInput) (DeviceGroupView, error) {
	return s.deviceGroupRuntime().UpdateDeviceGroup(ctx, UpdateDeviceGroupInput{
		GroupID: strings.TrimSpace(groupID), Name: strings.TrimSpace(input.Name), Description: strings.TrimSpace(input.Description),
	})
}

func (s OpsResourceService) DeleteOpsDeviceGroup(ctx context.Context, groupID string) error {
	return s.deviceGroupRuntime().DeleteDeviceGroup(ctx, DeleteDeviceGroupInput{GroupID: strings.TrimSpace(groupID)})
}

func (s OpsResourceService) AddOpsDeviceGroupMember(ctx context.Context, groupID, deviceID string) error {
	return s.updateOpsDeviceGroups(ctx, groupID, deviceID, true)
}
func (s OpsResourceService) RemoveOpsDeviceGroupMember(ctx context.Context, groupID, deviceID string) error {
	return s.updateOpsDeviceGroups(ctx, groupID, deviceID, false)
}

func (s OpsResourceService) updateOpsDeviceGroups(ctx context.Context, groupID, deviceID string, add bool) error {
	if _, ok, err := s.Devices.GetDeviceGroup(ctx, strings.TrimSpace(groupID)); err != nil {
		return err
	} else if !ok {
		return ErrNotFound
	}
	if _, err := requireManagedDevice(ctx, s.Devices, strings.TrimSpace(deviceID)); err != nil {
		return err
	}
	assignments, err := s.Devices.ListDeviceGroupAssignments(ctx)
	if err != nil {
		return err
	}
	ids := []string{}
	for _, assignment := range assignments {
		if assignment.DeviceID == deviceID {
			ids = append(ids, assignment.GroupIDs...)
			break
		}
	}
	next := make([]string, 0, len(ids)+1)
	found := false
	for _, id := range ids {
		if id == groupID {
			found = true
			if !add {
				continue
			}
		}
		next = append(next, id)
	}
	if add && !found {
		next = append(next, groupID)
	}
	if err := s.Devices.SetDeviceGroups(ctx, model.DeviceGroupAssignment{DeviceID: deviceID, GroupIDs: next, UpdatedAt: currentTime(s.Now).Unix()}); err != nil {
		return err
	}
	return s.deviceGroupRuntime().syncAllNetworkDeviceGroups(ctx, "device_group_assignment_updated")
}

func (s OpsResourceService) AddOpsNetworkDeviceGroup(ctx context.Context, networkID, groupID string) error {
	if _, err := requireManagedNetwork(ctx, s.Networks, strings.TrimSpace(networkID)); err != nil {
		return err
	}
	if _, ok, err := s.Devices.GetDeviceGroup(ctx, strings.TrimSpace(groupID)); err != nil {
		return err
	} else if !ok {
		return ErrNotFound
	}
	_, err := s.deviceGroupRuntime().AddNetworkDeviceGroup(ctx, AddNetworkDeviceGroupInput{NetworkID: networkID, GroupID: groupID})
	return err
}
func (s OpsResourceService) RemoveOpsNetworkDeviceGroup(ctx context.Context, networkID, groupID string) error {
	_, err := s.deviceGroupRuntime().RemoveNetworkDeviceGroup(ctx, RemoveNetworkDeviceGroupInput{NetworkID: networkID, GroupID: groupID})
	return err
}

func (s OpsResourceService) opsNetworkView(ctx context.Context, item model.Network) (OpsNetworkView, error) {
	members, err := s.Networks.ListNetworkDevices(ctx, item.NetworkID)
	if err != nil {
		return OpsNetworkView{}, err
	}
	refs, err := s.NetworkGroups.ListNetworkDeviceGroupReferences(ctx, item.NetworkID)
	if err != nil {
		return OpsNetworkView{}, err
	}
	view := OpsNetworkView{NetworkID: item.NetworkID, Name: item.Name, IntraGroupPolicy: item.IntraGroupPolicy,
		Status: item.Status, DeviceIDs: []string{}, DeviceGroupIDs: []string{}, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
	for _, member := range members {
		view.DeviceIDs = append(view.DeviceIDs, member.DeviceID)
	}
	for _, ref := range refs {
		view.DeviceGroupIDs = append(view.DeviceGroupIDs, ref.GroupID)
	}
	return view, nil
}

func normalizeOpsNetworkInput(input OpsNetworkInput) OpsNetworkInput {
	input.Name = strings.TrimSpace(input.Name)
	input.IntraGroupPolicy, input.Status = strings.TrimSpace(input.IntraGroupPolicy), strings.TrimSpace(input.Status)
	return input
}
