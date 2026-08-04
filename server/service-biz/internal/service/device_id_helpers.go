package service

type deviceGroupIDProvider interface{ NewDeviceGroupID() string }
type deviceVirtualIPIDProvider interface{ NewDeviceVirtualIPID() string }
