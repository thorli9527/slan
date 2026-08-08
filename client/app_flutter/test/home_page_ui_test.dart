import 'dart:async';

import 'package:client_core_plugin/client_core_plugin.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:slan_client_v2/app/slan_client_v2_app.dart';
import 'package:slan_client_v2/bridge/android_network_authorization.dart';
import 'package:slan_client_v2/bridge/client_commands.dart';
import 'package:slan_client_v2/bridge/client_core_bridge.dart';
import 'package:slan_client_v2/bridge/client_view_state.dart';
import 'package:slan_client_v2/bridge/control_transport_status.dart';

void main() {
  testWidgets('home omits shell header and runtime notice banner',
      (tester) async {
    final bridge = _UiTestBridge(
      initialState: const ClientViewState(
        activated: true,
        deviceId: 'device-test',
        networkEnabled: false,
        syncing: false,
        switchEnabled: true,
        notice: 'activated',
      ),
      activationDelay: Duration.zero,
    );

    await tester.pumpWidget(SlanClientV2App(bridge: bridge));
    await tester.pumpAndSettle();

    expect(find.text('SLAN Client'), findsNothing);
    expect(find.text('activated'), findsNothing);
    expect(find.byKey(const Key('network-switch')), findsOneWidget);
    expect(find.byKey(const Key('network-ip-value')), findsOneWidget);
    expect(find.text('当前用户邮箱'), findsNothing);
    expect(find.text('device-test'), findsNothing);
    expect(find.byKey(const Key('current-device-value')), findsNothing);
  });

  testWidgets('device activation submits a trimmed authorization key',
      (tester) async {
    debugDefaultTargetPlatformOverride = TargetPlatform.macOS;
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(SystemChannels.platform, (call) async {
      if (call.method == 'Clipboard.getData') {
        return <String, Object?>{'text': '  keyid_secret  '};
      }
      return null;
    });
    try {
      final bridge = _UiTestBridge(
        initialState: ClientViewState.initial(),
        activationDelay: Duration.zero,
      );

      await tester.pumpWidget(SlanClientV2App(bridge: bridge));
      await tester.pumpAndSettle();

      await tester.tap(
        find.byKey(const Key('device-authorization-key-paste')),
      );
      await tester.pump();
      final button = find.byKey(const Key('device-activate-submit'));
      await tester.tap(button);
      await tester.pumpAndSettle();

      expect(bridge.activatedKeys, ['keyid_secret']);
      expect(
        tester
            .widget<TextField>(
              find.byKey(const Key('device-authorization-key')),
            )
            .controller
            ?.text,
        isEmpty,
      );
    } finally {
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(SystemChannels.platform, null);
      debugDefaultTargetPlatformOverride = null;
    }
  });

  testWidgets('desktop network control omits enabled status label',
      (tester) async {
    debugDefaultTargetPlatformOverride = TargetPlatform.macOS;
    try {
      final bridge = _UiTestBridge(
        initialState: const ClientViewState(
          activated: true,
          deviceId: 'device-test',
          networkEnabled: true,
          syncing: false,
          switchEnabled: true,
        ),
        activationDelay: Duration.zero,
      );

      await tester.pumpWidget(SlanClientV2App(bridge: bridge));
      await tester.pumpAndSettle();

      expect(find.byKey(const Key('network-switch')), findsOneWidget);
      expect(find.text('网络已启用'), findsNothing);
      expect(find.text('网络未启用'), findsNothing);
    } finally {
      debugDefaultTargetPlatformOverride = null;
    }
  });

  testWidgets('desktop omits network invite controls', (tester) async {
    debugDefaultTargetPlatformOverride = TargetPlatform.macOS;
    try {
      final bridge = _UiTestBridge(
        initialState: const ClientViewState(
          activated: true,
          deviceId: 'device-test',
          virtualIp: '10.0.0.8',
          networkEnabled: true,
          syncing: false,
          switchEnabled: true,
          signalScore: 91,
          signalQuality: 'excellent',
          signalPath: 'direct_udp',
        ),
        activationDelay: Duration.zero,
      );

      await tester.pumpWidget(SlanClientV2App(bridge: bridge));
      await tester.pumpAndSettle();

      expect(find.text('优秀 · 91 分 · UDP 直联'), findsNothing);
      expect(
          find.byKey(const Key('client-signal-quality-value')), findsNothing);
      expect(find.byKey(const Key('network-docking-actions')), findsNothing);
      expect(find.byKey(const Key('generate-network-invite')), findsNothing);
      expect(find.text('生成接入码'), findsNothing);
      expect(find.byKey(const Key('accept-network-invite')), findsNothing);
    } finally {
      debugDefaultTargetPlatformOverride = null;
    }
  });

  testWidgets('desktop activated layout fits the native content area',
      (tester) async {
    debugDefaultTargetPlatformOverride = TargetPlatform.macOS;
    try {
      await tester.binding.setSurfaceSize(const Size(460, 176));
      final bridge = _UiTestBridge(
        initialState: const ClientViewState(
          activated: true,
          virtualIp: '10.0.1.137',
          networkEnabled: true,
          syncing: false,
          switchEnabled: true,
        ),
        activationDelay: Duration.zero,
      );

      await tester.pumpWidget(SlanClientV2App(bridge: bridge));
      await tester.pumpAndSettle();

      final networkSwitch = find.byKey(const Key('network-switch'));
      expect(networkSwitch, findsOneWidget);
      expect(find.text('解除激活'), findsNothing);
      expect(tester.getBottomRight(networkSwitch).dy, lessThanOrEqualTo(176));
      expect(tester.takeException(), isNull);
    } finally {
      debugDefaultTargetPlatformOverride = null;
      await tester.binding.setSurfaceSize(null);
    }
  });

  testWidgets('desktop inactive layout stays compact', (tester) async {
    debugDefaultTargetPlatformOverride = TargetPlatform.macOS;
    try {
      await tester.binding.setSurfaceSize(const Size(460, 176));
      final bridge = _UiTestBridge(
        initialState: ClientViewState.initial(),
        activationDelay: Duration.zero,
      );

      await tester.pumpWidget(SlanClientV2App(bridge: bridge));
      await tester.pumpAndSettle();

      final activation = find.byKey(const Key('device-activate-submit'));
      expect(activation, findsOneWidget);
      expect(tester.getBottomRight(activation).dy, lessThanOrEqualTo(176));

      await tester.tap(find.byKey(const Key('server-settings')));
      await tester.pumpAndSettle();
      final save = find.byKey(const Key('server-save'));
      expect(save, findsOneWidget);
      expect(tester.getBottomRight(save).dy, lessThanOrEqualTo(176));
      expect(tester.takeException(), isNull);
    } finally {
      debugDefaultTargetPlatformOverride = null;
      await tester.binding.setSurfaceSize(null);
    }
  });

  testWidgets('mobile activation server settings updates bridge url',
      (tester) async {
    debugDefaultTargetPlatformOverride = TargetPlatform.android;

    try {
      final bridge = _UiTestBridge(
        initialState: const ClientViewState(
          activated: false,
          networkEnabled: false,
          syncing: false,
          switchEnabled: true,
        ),
        activationDelay: Duration.zero,
      );

      await tester.pumpWidget(SlanClientV2App(bridge: bridge));
      await tester.pumpAndSettle();

      expect(find.byKey(const Key('server-base-url-value')), findsOneWidget);
      expect(find.text('http://127.0.0.1:28080'), findsOneWidget);

      await tester.tap(find.byKey(const Key('server-settings')));
      await tester.pumpAndSettle();
      await tester.enterText(
        find.byKey(const Key('server-base-url')),
        '10.0.2.2:28080/',
      );
      await tester.tap(find.byKey(const Key('server-save')));
      await tester.pumpAndSettle();

      expect(bridge.serverUrl, 'http://10.0.2.2:28080');
      expect(find.text('http://10.0.2.2:28080'), findsOneWidget);
    } finally {
      debugDefaultTargetPlatformOverride = null;
    }
  });

  testWidgets('desktop activated client hides server API settings',
      (tester) async {
    debugDefaultTargetPlatformOverride = TargetPlatform.macOS;
    try {
      await tester.binding.setSurfaceSize(const Size(460, 176));
      final bridge = _UiTestBridge(
        initialState: const ClientViewState(
          activated: true,
          deviceId: 'device-test',
          networkEnabled: false,
          syncing: false,
          switchEnabled: true,
        ),
        activationDelay: Duration.zero,
      );

      await tester.pumpWidget(SlanClientV2App(bridge: bridge));
      await tester.pumpAndSettle();

      expect(find.byKey(const Key('server-settings')), findsNothing);
      expect(find.text('服务器 API 设置'), findsNothing);
      expect(find.text('解除激活'), findsNothing);
      expect(tester.takeException(), isNull);
    } finally {
      debugDefaultTargetPlatformOverride = null;
      await tester.binding.setSurfaceSize(null);
    }
  });

  testWidgets('switch shows ip only after network activation completes',
      (tester) async {
    final bridge = _UiTestBridge(
      initialState: const ClientViewState(
        activated: true,
        virtualIp: '10.0.0.10',
        networkEnabled: false,
        syncing: false,
        switchEnabled: true,
      ),
      activationDelay: const Duration(milliseconds: 250),
      assignedIp: '10.0.0.10',
    );

    await tester.pumpWidget(SlanClientV2App(bridge: bridge));
    await tester.pumpAndSettle();

    expect(_networkSwitch(tester).value, isFalse);
    expect(find.text('未启用'), findsOneWidget);
    expect(find.text('10.0.0.10'), findsNothing);

    await tester.tap(find.byKey(const Key('network-switch')));
    await tester.pump();

    expect(bridge.lastCommand, ClientCommandType.enableNetwork);
    expect(_networkSwitch(tester).value, isFalse);
    expect(_networkSwitch(tester).onChanged, isNull);
    expect(find.text('10.0.0.10'), findsNothing);

    await tester.pump(bridge.activationDelay);
    await tester.pumpAndSettle();

    expect(_networkSwitch(tester).value, isTrue);
    expect(_networkSwitch(tester).onChanged, isNotNull);
    expect(find.byKey(const Key('network-ip-value')), findsOneWidget);
    expect(find.text('10.0.0.10'), findsOneWidget);
  });

  testWidgets('switch blocks repeated taps before bridge state update',
      (tester) async {
    final bridge = _UiTestBridge(
      initialState: const ClientViewState(
        activated: true,
        networkEnabled: false,
        syncing: false,
        switchEnabled: true,
      ),
      dispatchStartDelay: const Duration(milliseconds: 100),
      activationDelay: const Duration(milliseconds: 100),
      assignedIp: '10.0.0.10',
    );

    await tester.pumpWidget(SlanClientV2App(bridge: bridge));
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const Key('network-switch')));
    await tester.tap(find.byKey(const Key('network-switch')));
    expect(bridge.networkCommandCount, 1);

    await tester.pump();
    expect(_networkSwitch(tester).value, isFalse);
    expect(_networkSwitch(tester).onChanged, isNull);

    await tester.pump(const Duration(milliseconds: 200));
    await tester.pumpAndSettle();
    expect(bridge.networkCommandCount, 1);
    expect(_networkSwitch(tester).value, isTrue);
    expect(_networkSwitch(tester).onChanged, isNotNull);
  });

  testWidgets('switch reverts and ip stays disabled when activation fails',
      (tester) async {
    final bridge = _UiTestBridge(
      initialState: const ClientViewState(
        activated: true,
        networkEnabled: false,
        syncing: false,
        switchEnabled: true,
      ),
      activationDelay: const Duration(milliseconds: 100),
      activationError: 'device unavailable',
    );

    await tester.pumpWidget(SlanClientV2App(bridge: bridge));
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const Key('network-switch')));
    await tester.pump();

    expect(_networkSwitch(tester).value, isFalse);

    await tester.pump(bridge.activationDelay);
    await tester.pumpAndSettle();

    expect(find.text('操作失败'), findsOneWidget);
    expect(find.text('设备不可用，请联系管理员重新启用。'), findsOneWidget);
    await tester.tap(find.text('确定'));
    await tester.pumpAndSettle();

    expect(_networkSwitch(tester).value, isFalse);
    expect(_networkSwitch(tester).onChanged, isNotNull);
    expect(find.text('未启用'), findsOneWidget);
  });

  testWidgets('server disabled activation failure shows admin message',
      (tester) async {
    await tester.binding.setSurfaceSize(const Size(455, 247));
    addTearDown(() => tester.binding.setSurfaceSize(null));

    final bridge = _UiTestBridge(
      initialState: const ClientViewState(
        activated: true,
        networkEnabled: false,
        syncing: false,
        switchEnabled: true,
      ),
      activationDelay: const Duration(milliseconds: 100),
      activationError:
          'device unavailable: current device has been disabled by network admin',
    );

    await tester.pumpWidget(SlanClientV2App(bridge: bridge));
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const Key('network-switch')));
    await tester.pump(bridge.activationDelay);
    await tester.pumpAndSettle();

    expect(find.text('操作失败'), findsOneWidget);
    expect(find.text('服务端停用，请联系管理员。'), findsOneWidget);
    expect(find.text('设备不可用，请联系管理员重新启用。'), findsNothing);
  });

  testWidgets('disable failure shows error dialog', (tester) async {
    final bridge = _UiTestBridge(
      initialState: const ClientViewState(
        activated: true,
        networkEnabled: true,
        syncing: false,
        switchEnabled: true,
        virtualIp: '10.0.0.10',
      ),
      activationDelay: const Duration(milliseconds: 100),
      disableError: 'network command failed',
    );

    await tester.pumpWidget(SlanClientV2App(bridge: bridge));
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const Key('network-switch')));
    await tester.pump();

    expect(bridge.lastCommand, ClientCommandType.disableNetwork);
    expect(_networkSwitch(tester).value, isTrue);
    expect(_networkSwitch(tester).onChanged, isNull);

    await tester.pump(bridge.activationDelay);
    await tester.pumpAndSettle();

    expect(find.text('操作失败'), findsOneWidget);
    expect(find.text('network command failed'), findsOneWidget);
    await tester.tap(find.text('确定'));
    await tester.pumpAndSettle();

    expect(_networkSwitch(tester).value, isTrue);
    expect(_networkSwitch(tester).onChanged, isNotNull);
    expect(find.text('10.0.0.10'), findsOneWidget);
  });

  testWidgets('non-network errors do not show switch error dialog',
      (tester) async {
    final bridge = _UiTestBridge(
      initialState: const ClientViewState(
        activated: true,
        networkEnabled: false,
        syncing: false,
        switchEnabled: true,
        error: 'session expired',
      ),
      activationDelay: const Duration(milliseconds: 100),
    );

    await tester.pumpWidget(SlanClientV2App(bridge: bridge));
    await tester.pumpAndSettle();

    expect(find.text('操作失败'), findsNothing);
    expect(find.text('登录状态已失效，请重新登录。'), findsNothing);
  });

  testWidgets('same switch error can be shown again after retry',
      (tester) async {
    final bridge = _UiTestBridge(
      initialState: const ClientViewState(
        activated: true,
        networkEnabled: false,
        syncing: false,
        switchEnabled: true,
      ),
      activationDelay: const Duration(milliseconds: 100),
      activationError: 'device unavailable',
    );

    await tester.pumpWidget(SlanClientV2App(bridge: bridge));
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const Key('network-switch')));
    await tester.pump(bridge.activationDelay);
    await tester.pumpAndSettle();
    expect(find.text('操作失败'), findsOneWidget);
    await tester.tap(find.text('确定'));
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const Key('network-switch')));
    await tester.pump(bridge.activationDelay);
    await tester.pumpAndSettle();
    expect(find.text('操作失败'), findsOneWidget);
  });

  testWidgets('android authorization panel shows fetched network config',
      (tester) async {
    final bridge = _UiTestBridge(
      initialState: const ClientViewState(
        activated: true,
        networkEnabled: false,
        syncing: false,
        switchEnabled: true,
      ),
      androidAuthorizationState: const AndroidNetworkAuthorizationState(
        checking: false,
        permissionState: AndroidVpnPermissionState.granted,
        networkConfig: AndroidVpnSessionConfig(
          sessionName: 'SLAN',
          virtualIp: '10.0.0.10',
          prefixLen: 32,
          relayEndpointId: 'relay-cn',
        ),
      ),
      activationDelay: Duration.zero,
    );

    await tester.pumpWidget(SlanClientV2App(bridge: bridge));
    await tester.pumpAndSettle();

    expect(find.text('Android 网络配置已就绪'), findsOneWidget);
    expect(find.textContaining('10.0.0.10/32'), findsOneWidget);
    expect(find.textContaining('relay-cn'), findsNothing);
  });

  testWidgets('android authorization panel exposes consent action',
      (tester) async {
    final bridge = _UiTestBridge(
      initialState: const ClientViewState(
        activated: true,
        networkEnabled: false,
        syncing: false,
        switchEnabled: true,
      ),
      androidAuthorizationState: const AndroidNetworkAuthorizationState(
        checking: false,
        permissionState: AndroidVpnPermissionState.needsUserConsent,
        consentRequest: AndroidVpnConsentRequest(requestId: 'vpn-request-1'),
      ),
      activationDelay: Duration.zero,
    );

    await tester.pumpWidget(SlanClientV2App(bridge: bridge));
    await tester.pumpAndSettle();

    expect(find.text('Android 网络待授权'), findsOneWidget);
    expect(find.text('授权'), findsOneWidget);

    await tester.tap(find.text('授权'));
    await tester.pumpAndSettle();

    expect(bridge.androidPrepareCount, greaterThanOrEqualTo(2));
  });

  testWidgets('activated panel hides device identity', (tester) async {
    final bridge = _UiTestBridge(
      initialState: const ClientViewState(
        activated: true,
        deviceId: 'device-current',
        networkEnabled: true,
        syncing: false,
        switchEnabled: true,
        virtualIp: '10.0.0.10',
      ),
      activationDelay: Duration.zero,
    );

    await tester.pumpWidget(SlanClientV2App(bridge: bridge));
    await tester.pumpAndSettle();

    expect(find.byKey(const Key('current-device-value')), findsNothing);
    expect(find.text('device-current'), findsNothing);
    expect(find.text('tester@example.com'), findsNothing);
    expect(find.byKey(const Key('network-ip-value')), findsOneWidget);
    expect(find.text('10.0.0.10'), findsOneWidget);
  });
}

Switch _networkSwitch(WidgetTester tester) {
  return tester.widget<Switch>(find.byKey(const Key('network-switch')));
}

class _UiTestBridge implements ClientCoreBridge {
  _UiTestBridge({
    required ClientViewState initialState,
    required this.activationDelay,
    AndroidNetworkAuthorizationState androidAuthorizationState =
        AndroidNetworkAuthorizationState.initial,
    this.assignedIp,
    this.activationError,
    this.disableError,
    this.dispatchStartDelay = Duration.zero,
  })  : _state = ValueNotifier<ClientViewState>(initialState),
        _androidNetworkAuthorization =
            ValueNotifier<AndroidNetworkAuthorizationState>(
          androidAuthorizationState,
        );

  final ValueNotifier<ClientViewState> _state;
  final ValueNotifier<AndroidNetworkAuthorizationState>
      _androidNetworkAuthorization;
  final Duration activationDelay;
  final String? assignedIp;
  final String? activationError;
  final String? disableError;
  final Duration dispatchStartDelay;
  ClientCommandType? lastCommand;
  Map<String, Object?>? lastPayload;
  int androidPrepareCount = 0;
  int networkCommandCount = 0;
  final List<String> activatedKeys = [];
  String serverUrl = 'http://127.0.0.1:28080';

  @override
  ValueListenable<ClientViewState> get state => _state;

  @override
  ValueListenable<AndroidNetworkAuthorizationState>
      get androidNetworkAuthorization => _androidNetworkAuthorization;

  @override
  Future<void> start() async {}

  @override
  Future<void> notifyAppResumed() async {}

  @override
  Future<String> serverBaseUrl() async => serverUrl;

  @override
  Future<void> updateServerBaseUrl(String serverBaseUrl) async {
    var value = serverBaseUrl.trim();
    if (!value.contains('://')) {
      value = 'http://$value';
    }
    serverUrl = value.replaceFirst(RegExp(r'/+$'), '');
  }

  @override
  Future<void> activateDevice(String key) async {
    activatedKeys.add(key);
  }

  @override
  Future<void> prepareAndroidNetworkAuthorization() async {
    androidPrepareCount += 1;
  }

  @override
  Future<void> dispatch(ClientCommand command) async {
    if (command.type == ClientCommandType.enableNetwork ||
        command.type == ClientCommandType.disableNetwork) {
      networkCommandCount += 1;
      if (dispatchStartDelay > Duration.zero) {
        await Future<void>.delayed(dispatchStartDelay);
      }
    }
    lastCommand = command.type;
    lastPayload = command.payload;
    if (command.type == ClientCommandType.enableNetwork) {
      _enable(command);
      return;
    }
    if (command.type == ClientCommandType.disableNetwork) {
      _disable(command);
      return;
    }
  }

  void _enable(ClientCommand command) {
    final previousIp = _state.value.virtualIp;
    _state.value = _state.value.copyWith(
      syncing: true,
      syncReason: command.type.name,
      switchEnabled: false,
      error: null,
      notice: null,
      virtualIp: previousIp,
    );

    unawaited(Future<void>(() async {
      await Future<void>.delayed(activationDelay);
      if (activationError != null) {
        _state.value = _state.value.copyWith(
          networkEnabled: false,
          syncing: false,
          clearSyncReason: true,
          switchEnabled: true,
          error: activationError,
          errorSource: ClientErrorSource.networkSwitch,
          virtualIp: previousIp,
        );
        return;
      }
      _state.value = _state.value.copyWith(
        networkEnabled: true,
        syncing: false,
        clearSyncReason: true,
        switchEnabled: true,
        virtualIp: assignedIp,
      );
    }));
  }

  void _disable(ClientCommand command) {
    final previousIp = _state.value.virtualIp;
    _state.value = _state.value.copyWith(
      syncing: true,
      syncReason: command.type.name,
      switchEnabled: false,
      error: null,
      notice: null,
      virtualIp: previousIp,
    );

    unawaited(Future<void>(() async {
      await Future<void>.delayed(activationDelay);
      if (disableError != null) {
        _state.value = _state.value.copyWith(
          networkEnabled: true,
          syncing: false,
          clearSyncReason: true,
          switchEnabled: true,
          virtualIp: previousIp,
          error: disableError,
          errorSource: ClientErrorSource.networkSwitch,
        );
        return;
      }
      _state.value = _state.value.copyWith(
        networkEnabled: false,
        syncing: false,
        clearSyncReason: true,
        switchEnabled: true,
        virtualIp: previousIp,
      );
    }));
  }

  @override
  Future<ControlTransportStatus?> localControlStatus() async => null;

  @override
  Future<void> close() async {}
}
