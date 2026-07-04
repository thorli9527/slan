package service

type deviceBootstrapKeyIDProvider interface{ NewDeviceBootstrapKeyID() string }
type deviceGroupIDProvider interface{ NewDeviceGroupID() string }
type deviceVirtualIPIDProvider interface{ NewDeviceVirtualIPID() string }
