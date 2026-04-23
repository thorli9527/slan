import 'package:flutter_test/flutter_test.dart';
import 'package:slan_app/application/control_plan_insights.dart';
import 'package:slan_app/infra/app_core/models/connection_models.dart';
import 'package:slan_app/infra/app_core/models/control_models.dart';
import 'package:slan_app/infra/app_core/models/diagnostic_models.dart';

void main() {
  group('control_plan_insights', () {
    test('formats preferred control path and relay labels', () {
      const status = ControlStatusModel(
        status: 'connected',
        sessionTokenPresent: true,
        networkMapPresent: true,
        peerCount: 1,
        connectPlanCount: 1,
        connectPlans: [
          ControlConnectPlanModel(
            peerNodeId: 'peer-1',
            preferDirect: false,
            pathCount: 1,
            preferredPath: ControlPathOptionModel(
              pathType: 'relay',
              endpoint: 'udp://127.0.0.1:19000',
              priority: 10,
            ),
            derpClusterId: 'cn-local-a',
            preferredDerpNodeIds: ['relay-cn-local-udp'],
          ),
        ],
      );

      expect(
        preferredControlPathLabel(status),
        'relay udp://127.0.0.1:19000',
      );
      expect(preferredControlRelayLabel(status), 'relay-cn-local-udp');
    });

    test('finds connect plan by peer id', () {
      const status = ControlStatusModel(
        status: 'connected',
        sessionTokenPresent: true,
        networkMapPresent: true,
        peerCount: 2,
        connectPlanCount: 2,
        connectPlans: [
          ControlConnectPlanModel(
            peerNodeId: 'peer-1',
            preferDirect: true,
            pathCount: 1,
          ),
          ControlConnectPlanModel(
            peerNodeId: 'peer-2',
            preferDirect: false,
            pathCount: 1,
            derpClusterId: 'cn-local-a',
          ),
        ],
      );

      expect(connectPlanForPeer(status, 'peer-2')?.peerNodeId, 'peer-2');
      expect(connectPlanForPeer(status, '  '), isNull);
      expect(connectPlanForPeer(status, 'missing'), isNull);
    });

    test('marks relay recommendation as matched when connection or probe looks relay', () {
      const plan = ControlConnectPlanModel(
        peerNodeId: 'peer-1',
        preferDirect: false,
        pathCount: 1,
        derpClusterId: 'cn-local-a',
        preferredDerpNodeIds: ['relay-cn-local-udp'],
      );
      const probe = DataPlaneProbeModel(
        probeId: 'probe-1',
        sampledAtMs: 1,
        activePath: {
          'pathType': 'relay',
        },
        bytesSent: 5,
        replyObserved: true,
      );

      expect(
        connectRecommendationMatchLabel(
          controlPlan: plan,
          connectionState:
              const ConnectionStateModel.connected(ConnectionPathModel.relay),
          lastProbe: null,
        ),
        'matched',
      );
      expect(
        connectRecommendationMatchLabel(
          controlPlan: plan,
          connectionState:
              const ConnectionStateModel.connected(ConnectionPathModel.p2p),
          lastProbe: probe,
        ),
        'matched',
      );
    });

    test('marks direct recommendation as diverged when relay wins', () {
      const plan = ControlConnectPlanModel(
        peerNodeId: 'peer-1',
        preferDirect: true,
        pathCount: 1,
        preferredPath: ControlPathOptionModel(
          pathType: 'direct_udp',
          endpoint: '198.51.100.10:51820',
          priority: 10,
        ),
      );

      expect(
        connectRecommendationMatchLabel(
          controlPlan: plan,
          connectionState:
              const ConnectionStateModel.connected(ConnectionPathModel.relay),
          lastProbe: null,
        ),
        'diverged',
      );
    });
  });
}
