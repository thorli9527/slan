package service

import (
	"context"
	"strings"
	"time"
)

const deviceGeoRefreshInterval = 24 * time.Hour

type DeviceLocation struct {
	CountryCode string
	CityCode    string
}

func (s NetworkRuntimeService) CurrentDeviceLocation(ctx context.Context, deviceID string) (DeviceLocation, bool, error) {
	if s.Devices == nil {
		return DeviceLocation{}, false, nil
	}
	device, ok, err := s.Devices.GetDevice(ctx, strings.TrimSpace(deviceID))
	if err != nil || !ok {
		return DeviceLocation{}, false, err
	}
	location := DeviceLocation{
		CountryCode: strings.ToUpper(strings.TrimSpace(device.CountryCode)),
		CityCode:    strings.TrimSpace(device.CityCode),
	}
	return location, location.CountryCode != "", nil
}

func (s NetworkRuntimeService) ObserveDeviceLocation(ctx context.Context, deviceID, publicIP string) error {
	deviceID = strings.TrimSpace(deviceID)
	publicIP = strings.TrimSpace(publicIP)
	if deviceID == "" || publicIP == "" || s.LocateIP == nil || s.Devices == nil {
		return nil
	}
	location, ok := s.LocateIP(publicIP)
	if !ok {
		return nil
	}
	now := networkNow(s.Now).UTC()
	device, found, err := s.Devices.GetDevice(ctx, deviceID)
	if err != nil || !found {
		return err
	}
	countryCode := strings.ToUpper(strings.TrimSpace(location.CountryCode))
	cityCode := strings.TrimSpace(location.CityCode)
	if device.PublicIP == publicIP &&
		device.CountryCode == countryCode &&
		device.CityCode == cityCode &&
		device.GeoUpdatedAt > now.Add(-deviceGeoRefreshInterval).Unix() {
		return nil
	}
	device.PublicIP = publicIP
	device.CountryCode = countryCode
	device.CityCode = cityCode
	device.GeoUpdatedAt = now.Unix()
	device.UpdatedAt = now.Unix()
	return s.Devices.SaveDevice(ctx, device)
}
