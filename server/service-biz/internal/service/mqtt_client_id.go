package service

import (
	"fmt"
	"sync/atomic"
	"time"
)

var mqttClientIDSequence atomic.Uint64

func serverMQTTClientID(baseClientID, role string, attempt int) string {
	sequence := mqttClientIDSequence.Add(1)
	return fmt.Sprintf(
		"%s-%s-r%d-%x-%x",
		baseClientID,
		role,
		attempt,
		time.Now().UnixNano(),
		sequence,
	)
}
