package service

import (
	"testing"

	"github.com/slan/service-biz/internal/model"
)

func TestDeviceRuntimeOnlineSeparatesApplicationFromNetworkState(t *testing.T) {
	tests := []struct {
		name  string
		state model.DeviceRuntimeState
		want  bool
	}{
		{
			name:  "running with network enabled",
			state: model.DeviceRuntimeState{ApplicationState: "running", NetworkEnabled: true},
			want:  true,
		},
		{
			name:  "running with network disabled",
			state: model.DeviceRuntimeState{ApplicationState: "running", NetworkEnabled: false},
			want:  true,
		},
		{
			name:  "stopped",
			state: model.DeviceRuntimeState{ApplicationState: "stopped", NetworkEnabled: true},
			want:  false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := deviceRuntimeOnline(test.state); got != test.want {
				t.Fatalf("deviceRuntimeOnline() = %v, want %v", got, test.want)
			}
		})
	}
}
