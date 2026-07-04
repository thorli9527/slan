package service

import "fmt"

func serverMQTTClientID(baseClientID, role string, attempt int) string {
	clientID := fmt.Sprintf("%s-%s", baseClientID, role)
	if attempt > 1 {
		clientID = fmt.Sprintf("%s-r%d", clientID, attempt)
	}
	return clientID
}
