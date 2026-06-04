package biz

import (
	"context"
)

func (s *Store) loadPostgresCoreLocked(ctx context.Context) error {
	if s.db == nil {
		return nil
	}
	if err := s.loadPostgresUsersLocked(ctx); err != nil {
		return err
	}
	if err := s.loadPostgresNetworksLocked(ctx); err != nil {
		return err
	}
	if err := s.loadPostgresDevicesLocked(ctx); err != nil {
		return err
	}
	if err := s.loadPostgresLoginFailuresLocked(ctx); err != nil {
		return err
	}
	if err := s.loadPostgresDeviceSharingLocked(ctx); err != nil {
		return err
	}
	if err := s.loadPostgresIPAMLocked(ctx); err != nil {
		return err
	}
	if err := s.loadPostgresNetworkDevicesLocked(ctx); err != nil {
		return err
	}
	if err := s.loadPostgresRuntimeStatusesLocked(ctx); err != nil {
		return err
	}
	if err := s.loadPostgresNetworkConfigResourcesLocked(ctx); err != nil {
		return err
	}
	if err := s.loadPostgresDeviceSessionsLocked(ctx); err != nil {
		return err
	}
	return s.loadPostgresControlDeliveriesLocked(ctx)
}
