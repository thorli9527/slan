import 'package:flutter_test/flutter_test.dart';
import 'package:slan_app_core_plugin_platform_interface/slan_app_core_plugin_platform_interface.dart';

void main() {
  group('AppCoreMethodModels', () {
    test('decodes bootstrap payload', () {
      final payload = AppCoreBootstrapPayload.fromJson({
        'device': {
          'device': {
            'deviceId': 'dev-1',
            'name': 'thor-mac',
            'platform': 'macos',
            'status': 'online',
          },
          'attachments': [
            {
              'networkId': 'net-1',
              'deviceId': 'dev-1',
              'virtualIp': '100.64.0.10',
            },
          ],
        },
        'networks': [
          {
            'networkId': 'net-1',
            'name': 'home',
            'defaultSubnetCidr': '100.64.0.0/24',
            'subnets': [
              {
                'networkId': 'net-1',
                'cidr': '100.64.0.0/24',
                'isDefault': true,
              },
            ],
            'members': [
              {
                'deviceId': 'dev-1',
                'role': 'owner',
              },
            ],
          },
        ],
        'controlPlane': {
          'wsUrl': 'mqtt://127.0.0.1:1883',
          'heartbeatSeconds': 15,
        },
        'stunServers': ['stun:stun.l.google.com:19302'],
        'relay': {
          'defaultClusterId': 'cn-local-a',
          'countries': [
            {
              'countryCode': 'CN',
              'countryName': 'China',
              'cities': [
                {
                  'cityCode': 'local',
                  'cityName': 'Local',
                  'clusters': [
                    {
                      'clusterId': 'cn-local-a',
                      'clusterName': 'CN Local A',
                      'nodes': [
                        {
                          'nodeId': 'relay-cn-local-udp',
                          'transport': 'udp',
                          'address': '127.0.0.1:19000',
                          'priority': 10,
                        },
                      ],
                    },
                  ],
                },
              ],
            },
          ],
        },
        'networkMap': {
          'networkId': 'net-1',
        },
      });

      expect(payload.device.device.deviceId, 'dev-1');
      expect(payload.device.attachments.single.virtualIp, '100.64.0.10');
      expect(payload.networks.single.subnets.single.isDefault, isTrue);
      expect(payload.controlPlane.wsUrl, 'mqtt://127.0.0.1:1883');
      expect(payload.relay.defaultClusterId, 'cn-local-a');
      expect(payload.networkMap?.networkId, 'net-1');
    });

    test('decodes control status payload', () {
      final payload = AppCoreControlStatusPayload.fromJson({
        'status': 'connected',
        'wsUrl': 'mqtt://127.0.0.1:1883',
        'heartbeatSeconds': 15,
        'sessionTokenPresent': true,
        'networkMapPresent': true,
        'networkId': 'net-1',
        'nodeId': 'node-1',
        'deviceId': 'dev-1',
        'peerCount': 1,
        'connectPlanCount': 1,
        'connectPlans': [
          {
            'peerNodeId': 'node-2',
            'preferDirect': false,
            'pathCount': 1,
            'preferredPath': {
              'pathType': 'relay',
              'endpoint': 'udp://127.0.0.1:19000',
              'priority': 10,
            },
            'derpClusterId': 'cn-local-a',
            'preferredDerpNodeIds': ['relay-cn-local-udp'],
            'relayTicketId': 'ticket-1',
          },
        ],
      });

      expect(payload.status, 'connected');
      expect(payload.networkMapPresent, isTrue);
      expect(payload.connectPlans.single.peerNodeId, 'node-2');
      expect(payload.connectPlans.single.preferredPath?.pathType, 'relay');
      expect(payload.connectPlans.single.relayTicketId, 'ticket-1');
    });

    test('decodes data plane probe payload', () {
      final payload = AppCoreDataPlaneProbePayload.fromJson({
        'probeId': 'probe-1',
        'sampledAtMs': 1,
        'activePath': {
          'relay': {'peer_node_id': 'peer-1'},
        },
        'bytesSent': 5,
        'replyObserved': true,
        'replyBytesReceived': 5,
        'replySampledAtMs': 3,
        'replyRttMs': 2,
        'tunnelPeerVirtualIp': '100.64.0.2',
        'observedRttMs': 2,
        'packetLossPpm': 0,
        'pathScore': 100,
        'derpClusterId': 'cn-local-a',
        'derpNodeId': 'relay-cn-local-udp',
      });

      expect(payload.probeId, 'probe-1');
      expect(payload.activePath.kind, 'relay');
      expect(payload.activePath.isRelay, isTrue);
      expect(payload.activePath.details['peer_node_id'], 'peer-1');
      expect(payload.replyObserved, isTrue);
      expect(payload.replyRttMs, 2);
      expect(payload.derpClusterId, 'cn-local-a');
      expect(payload.derpNodeId, 'relay-cn-local-udp');
    });

    test('decodes tunnel backend diagnostics', () {
      final action = WireGuardTunnelActionResult.fromJson({
        'action': 'bringTunnelUp',
        'accepted': true,
        'phase': 'started',
        'source': 'rust-helper',
        'detail': 'started',
        'connectionStatus': 'connected',
        'hasConfiguration': true,
        'backendName': 'linux-kernel',
        'backendState': 'started',
        'backendExecutionMode': 'system',
        'backendExecutionBackend': 'shell',
        'backendInterfaceName': 'slan0',
        'backendIsUp': true,
        'backendPlannedPeerCount': 1,
        'backendRecentCommandCount': 4,
      });
      final runtime = WireGuardTunnelRuntimeView.fromJson({
        'state': 'configured',
        'transport': 'p2p',
        'backendName': 'linux-kernel',
        'backendState': 'started',
        'backendExecutionMode': 'system',
        'backendExecutionBackend': 'shell',
        'backendInterfaceName': 'slan0',
        'backendIsUp': true,
        'backendPlannedPeerCount': 1,
        'backendRecentCommandCount': 4,
        'peerVirtualIp': '100.64.0.2',
        'peerPublicKey': 'peer-pk',
        'localVirtualIp': '100.64.0.10',
        'remoteAddress': '198.51.100.10:51820',
      });

      expect(action.backendName, 'linux-kernel');
      expect(action.backendExecutionMode, 'system');
      expect(action.backendExecutionBackend, 'shell');
      expect(action.backendInterfaceName, 'slan0');
      expect(action.backendIsUp, isTrue);
      expect(action.backendPlannedPeerCount, 1);
      expect(action.backendRecentCommandCount, 4);
      expect(runtime.backendName, 'linux-kernel');
      expect(runtime.backendExecutionMode, 'system');
      expect(runtime.backendExecutionBackend, 'shell');
      expect(runtime.backendInterfaceName, 'slan0');
      expect(runtime.backendIsUp, isTrue);
      expect(runtime.backendPlannedPeerCount, 1);
      expect(runtime.backendRecentCommandCount, 4);
    });

    test('decodes platform doctor and install plan payloads', () {
      final doctor = AppCorePlatformDoctorPayload.fromJson({
        'platform': {
          'os': 'linux',
          'distroId': 'ubuntu',
          'versionId': '24.04',
          'idLike': ['debian'],
          'family': 'debian',
          'kernelRelease': '6.8.0',
          'packageManager': 'apt-get',
        },
        'tunnelBackend': {
          'name': 'linux-kernel',
          'executionMode': 'system',
          'executionBackend': 'shell',
          'interfaceName': 'slan0',
          'isUp': true,
          'plannedPeerCount': 1,
          'recentCommandCount': 4,
        },
        'checks': [
          {
            'name': 'ip_command',
            'status': 'ok',
            'detail': 'ip command available',
          },
        ],
      });
      final installPlan = AppCorePlatformInstallPlanPayload.fromJson({
        'platform': {'os': 'linux', 'family': 'debian'},
        'packages': ['wireguard-tools', 'iproute2'],
        'supportedDriverModes': ['in-memory', 'linux-kernel-shell'],
        'warnings': [],
      });

      expect(doctor.platform.os, 'linux');
      expect(doctor.platform.packageManager, 'apt-get');
      expect(doctor.tunnelBackend.name, 'linux-kernel');
      expect(doctor.tunnelBackend.isUp, isTrue);
      expect(doctor.checks.single.name, 'ip_command');
      expect(installPlan.packages, contains('wireguard-tools'));
      expect(installPlan.supportedDriverModes, contains('linux-kernel-shell'));
    });
  });

  group('SlanAppCorePluginPlatform', () {
    test('wraps typed methods and arguments', () async {
      final platform = _FakePlatform({
        'register': {
          'userId': 'user-1',
          'accessToken': 'token-1',
          'refreshToken': 'refresh-1',
          'expiresIn': 3600,
        },
        'listDevices': {
          'items': [
            {
              'deviceId': 'dev-1',
              'name': 'thor-mac',
              'platform': 'macos',
              'status': 'online',
            },
          ],
        },
        'joinNetwork': {
          'networkId': 'net-1',
          'deviceId': 'dev-1',
          'memberId': 'member-1',
          'attachmentId': 'att-1',
          'virtualIp': '10.0.0.2',
        },
        'joinNetworkByOwnerEmail': {
          'networkId': 'net-owner',
          'deviceId': 'dev-1',
          'memberId': 'member-owner',
          'attachmentId': 'att-owner',
          'virtualIp': '10.0.1.2',
        },
        'joinNetworkByKey': {
          'networkId': 'net-key',
          'deviceId': 'dev-1',
          'memberId': 'member-key',
          'attachmentId': 'att-key',
          'virtualIp': '10.0.2.2',
        },
        'updateAttachmentRemark': {
          'attachmentId': 'att-key',
          'networkId': 'net-key',
          'subnetId': 'subnet-key',
          'deviceId': 'dev-1',
          'deviceName': 'desk',
          'userId': 'user-1',
          'userEmail': 'user@example.com',
          'role': 'member',
          'remark': 'desk',
          'virtualIp': '10.0.2.2',
          'status': 'active',
        },
        'activateNetwork': {
          'networkId': 'net-key',
          'deviceId': 'dev-1',
          'memberId': 'member-key',
          'attachmentId': 'att-key',
          'virtualIp': '10.0.2.2',
        },
        'switchNetwork': {
          'networkId': 'net-key',
          'deviceId': 'dev-1',
          'memberId': 'member-key',
          'attachmentId': 'att-key',
          'virtualIp': '10.0.2.2',
        },
        'deactivateNetwork': null,
        'setDeviceNetworkState': null,
        'connect': {
          'status': 'connected',
          'path': 'derp',
        },
        'probe': {
          'probeId': 'probe-1',
          'sampledAtMs': 1,
          'activePath': {
            'relay': {'peer_node_id': 'peer-1'},
          },
          'bytesSent': 5,
          'replyObserved': true,
          'replyBytesReceived': 5,
          'replySampledAtMs': 3,
          'replyRttMs': 2,
          'tunnelPeerVirtualIp': '100.64.0.2',
          'observedRttMs': null,
          'packetLossPpm': null,
          'pathScore': null,
          'derpClusterId': null,
          'derpNodeId': null,
        },
        'send': {
          'status': 'sent',
          'bytesSent': 5,
          'path': 'relay',
        },
        'disconnect': null,
        'platformDoctor': {
          'platform': {'os': 'linux'},
          'tunnelBackend': {
            'name': 'linux-kernel',
            'isUp': false,
            'plannedPeerCount': 0,
            'recentCommandCount': 0,
          },
          'checks': [],
        },
        'platformInstallPlan': {
          'platform': {'os': 'linux'},
          'packages': ['wireguard-tools'],
          'supportedDriverModes': ['linux-kernel-shell'],
          'warnings': [],
        },
      });

      final session = await platform.register(
        email: 'user@example.com',
        password: 'password123',
      );
      final devices = await platform.listDevices();
      final joined = await platform.joinNetwork(
        networkId: 'net-1',
        deviceId: 'dev-1',
      );
      final ownerJoin = await platform.joinNetworkByOwnerEmail(
        ownerEmail: 'owner@example.com',
        deviceId: 'dev-1',
      );
      final keyJoin = await platform.joinNetworkByKey(
        joinKey: 'join-key-1',
        deviceId: 'dev-1',
      );
      final remark = await platform.updateAttachmentRemark(
        networkId: 'net-key',
        attachmentId: 'att-key',
        remark: 'desk',
      );
      final activated = await platform.activateNetwork(
        networkId: 'net-key',
        deviceId: 'dev-1',
      );
      final switched = await platform.switchNetwork(
        networkId: 'net-key',
        deviceId: 'dev-1',
      );
      await platform.deactivateNetwork(networkId: 'net-key', deviceId: 'dev-1');
      await platform.setDeviceNetworkState(
        deviceId: 'dev-1',
        networkId: 'net-key',
        controlReachable: true,
        networkOnline: true,
        tunnelUp: true,
        lastProbeOk: true,
        virtualIp: '10.0.2.2',
        reportedAt: 123,
      );
      final connect = await platform.connect(
        networkId: 'net-1',
        peerNodeId: 'node-2',
      );
      final probe = await platform.probe(
        payload: 'hello',
        probeTimeoutMs: 7,
      );
      final bytesSent = await platform.send(payload: 'hello');
      await platform.disconnect();
      final doctor = await platform.platformDoctor();
      final installPlan = await platform.platformInstallPlan();

      expect(session.accessToken, 'token-1');
      expect(devices.single.deviceId, 'dev-1');
      expect(joined.attachmentId, 'att-1');
      expect(ownerJoin.networkId, 'net-owner');
      expect(keyJoin.virtualIp, '10.0.2.2');
      expect(remark.remark, 'desk');
      expect(activated.attachmentId, 'att-key');
      expect(switched.attachmentId, 'att-key');
      expect(connect.path, 'derp');
      expect(probe.probeId, 'probe-1');
      expect(bytesSent, 5);
      expect(doctor.tunnelBackend.name, 'linux-kernel');
      expect(installPlan.packages, ['wireguard-tools']);
      expect(platform.calls.map((call) => call.method), [
        'register',
        'listDevices',
        'joinNetwork',
        'joinNetworkByOwnerEmail',
        'joinNetworkByKey',
        'updateAttachmentRemark',
        'activateNetwork',
        'switchNetwork',
        'deactivateNetwork',
        'setDeviceNetworkState',
        'connect',
        'probe',
        'send',
        'disconnect',
        'platformDoctor',
        'platformInstallPlan',
      ]);
      expect(platform.calls.first.args, {
        'email': 'user@example.com',
        'password': 'password123',
      });
      expect(platform.calls[2].args, {
        'networkId': 'net-1',
        'deviceId': 'dev-1',
      });
      expect(platform.calls[3].args, {
        'ownerEmail': 'owner@example.com',
        'deviceId': 'dev-1',
      });
      expect(platform.calls[4].args, {
        'joinKey': 'join-key-1',
        'deviceId': 'dev-1',
      });
      expect(platform.calls[5].args, {
        'networkId': 'net-key',
        'attachmentId': 'att-key',
        'remark': 'desk',
      });
      expect(platform.calls[6].args, {
        'networkId': 'net-key',
        'deviceId': 'dev-1',
      });
      expect(platform.calls[7].args, {
        'networkId': 'net-key',
        'deviceId': 'dev-1',
      });
      expect(platform.calls[8].args, {
        'networkId': 'net-key',
        'deviceId': 'dev-1',
      });
      expect(platform.calls[9].args, {
        'deviceId': 'dev-1',
        'networkId': 'net-key',
        'controlReachable': true,
        'networkOnline': true,
        'tunnelUp': true,
        'lastProbeOk': true,
        'virtualIp': '10.0.2.2',
        'reportedAt': 123,
      });
      expect(platform.calls[10].args, {
        'networkId': 'net-1',
        'peerNodeId': 'node-2',
      });
      expect(platform.calls[11].args, {
        'payload': 'hello',
        'probeTimeoutMs': 7,
      });
      expect(platform.calls[12].args, {
        'payload': 'hello',
      });
    });
  });
}

class _FakePlatform extends SlanAppCorePluginPlatform {
  _FakePlatform(this._responses);

  final Map<String, Object?> _responses;
  final List<_Call> calls = [];

  @override
  Future<Object?> invoke(
    String method, [
    Map<String, Object?> args = const {},
  ]) async {
    calls.add(_Call(method: method, args: Map<String, Object?>.from(args)));
    if (!_responses.containsKey(method)) {
      throw StateError('Missing fake response for $method');
    }
    return _responses[method];
  }
}

class _Call {
  const _Call({
    required this.method,
    required this.args,
  });

  final String method;
  final Map<String, Object?> args;
}
