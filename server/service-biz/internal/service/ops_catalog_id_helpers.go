package service

type clientDownloadIDProvider interface{ NewClientDownloadID() string }
type productIDProvider interface{ NewProductID() string }
type orderIDProvider interface{ NewOrderID() string }
