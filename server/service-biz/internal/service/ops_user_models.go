package service

type OpsUserRecord struct {
	UserID    string `json:"userId"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	Country   string `json:"country"`
	Province  string `json:"province"`
	City      string `json:"city"`
	IPRegion  string `json:"ipRegion"`
	Status    string `json:"status"`
	UpdatedAt int64  `json:"updatedAt"`
}

type OpsUserView struct {
	User           OpsUserRecord `json:"user"`
	OwnDevices     int           `json:"ownDevices"`
	InvitedDevices int           `json:"invitedDevices"`
	RelayUsedGB    int           `json:"relayUsedGb"`
}
