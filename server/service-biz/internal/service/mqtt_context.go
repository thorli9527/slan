package service

import (
	"context"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func mqttOperationTimeout(ctx context.Context, maximum time.Duration) (time.Duration, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return 0, context.DeadlineExceeded
		}
		if remaining < maximum {
			return remaining, nil
		}
	}
	return maximum, nil
}

func mqttWaitToken(ctx context.Context, token mqtt.Token, maximum time.Duration) (bool, error) {
	timeout, err := mqttOperationTimeout(ctx, maximum)
	if err != nil {
		return false, err
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case <-token.Done():
		return true, nil
	case <-timer.C:
		return false, nil
	}
}

func mqttRetryDelay(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
