package app

import (
	"reflect"
	"testing"

	servicepkg "github.com/slan/service-biz/internal/service"
)

func TestProxyEndpointsOnlyIncludeSuccessfullyDeployedNodes(t *testing.T) {
	nodes := []servicepkg.OpsServerNodeView{
		{NodeID: "node-a", ProxyEnabled: true, DeployStatus: "succeeded", APIProxyURL: "http://203.0.113.1:28080", MQTTProxyURL: "mqtt://203.0.113.1:1883"},
		{NodeID: "node-b", ProxyEnabled: true, DeployStatus: "failed", APIProxyURL: "http://203.0.113.2:28080", MQTTProxyURL: "mqtt://203.0.113.2:1883"},
		{NodeID: "node-c", ProxyEnabled: true, DeployStatus: "deploying", APIProxyURL: "http://203.0.113.3:28080", MQTTProxyURL: "mqtt://203.0.113.3:1883"},
		{NodeID: "node-d", ProxyEnabled: false, DeployStatus: "succeeded", APIProxyURL: "http://203.0.113.4:28080", MQTTProxyURL: "mqtt://203.0.113.4:1883"},
	}
	if got, want := activeAPIProxyURLs(nodes), []string{"http://203.0.113.1:28080"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("API proxy URLs = %#v, want %#v", got, want)
	}
	if got, want := activeMQTTProxyURLs(nodes), []string{"mqtt://203.0.113.1:1883"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("MQTT proxy URLs = %#v, want %#v", got, want)
	}
}
