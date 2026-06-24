package service

type deviceIDProvider interface{ NewDeviceID() string }
