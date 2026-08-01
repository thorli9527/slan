package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

type mqttContextTestToken struct {
	done chan struct{}
	err  error
}

func (t *mqttContextTestToken) Wait() bool { <-t.done; return true }
func (t *mqttContextTestToken) WaitTimeout(timeout time.Duration) bool {
	select {
	case <-t.done:
		return true
	case <-time.After(timeout):
		return false
	}
}
func (t *mqttContextTestToken) Done() <-chan struct{} { return t.done }
func (t *mqttContextTestToken) Error() error          { return t.err }

func TestMQTTOperationTimeoutHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := mqttOperationTimeout(ctx, 4*time.Second); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled context, got %v", err)
	}
}

func TestMQTTOperationTimeoutUsesShorterDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	timeout, err := mqttOperationTimeout(ctx, 4*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if timeout <= 0 || timeout > 200*time.Millisecond {
		t.Fatalf("unexpected bounded timeout: %s", timeout)
	}
}

func TestMQTTRetryDelayStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	if err := mqttRetryDelay(ctx, time.Second); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled retry delay, got %v", err)
	}
	if elapsed := time.Since(started); elapsed >= 100*time.Millisecond {
		t.Fatalf("canceled retry delay returned too slowly: %s", elapsed)
	}
}

func TestMQTTWaitTokenStopsWhenContextIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	token := &mqttContextTestToken{done: make(chan struct{})}
	time.AfterFunc(10*time.Millisecond, cancel)
	started := time.Now()
	completed, err := mqttWaitToken(ctx, token, time.Second)
	if completed || !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled token wait, completed=%t err=%v", completed, err)
	}
	if elapsed := time.Since(started); elapsed >= 100*time.Millisecond {
		t.Fatalf("token wait cancellation returned too slowly: %s", elapsed)
	}
}
