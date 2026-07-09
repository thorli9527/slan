import 'dart:async';

import 'package:client_core_plugin/client_core_plugin.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:slan_client_v2/app/slan_client_v2_app.dart';
import 'package:slan_client_v2/bridge/android_network_authorization.dart';
import 'package:slan_client_v2/bridge/client_commands.dart';
import 'package:slan_client_v2/bridge/client_core_bridge.dart';
import 'package:slan_client_v2/bridge/client_view_state.dart';
import 'package:slan_client_v2/bridge/control_transport_status.dart';

void main() {
  testWidgets('mobile login server settings updates bridge url',
      (tester) async {
    debugDefaultTargetPlatformOverride = TargetPlatform.android;

    try {
      final bridge = _UiTestBridge(
        initialState: const ClientViewState(
          signedIn: false,
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

  testWidgets('switch stays enabled while activating and shows assigned ip',
      (tester) async {
    final bridge = _UiTestBridge(
      initialState: const ClientViewState(
        signedIn: true,
        userLabel: 'tester@example.com',
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

    await tester.tap(find.byKey(const Key('network-switch')));
    await tester.pump();

    expect(bridge.lastCommand, ClientCommandType.enableNetwork);
    expect(_networkSwitch(tester).value, isTrue);
    expect(_networkSwitch(tester).onChanged, isNull);
    expect(find.text('未启用'), findsOneWidget);

    await tester.pump(bridge.activationDelay);
    await tester.pumpAndSettle();

    expect(_networkSwitch(tester).value, isTrue);
    expect(_networkSwitch(tester).onChanged, isNotNull);
    expect(find.byKey(const Key('network-ip-value')), findsOneWidget);
    expect(find.text('10.0.0.10'), findsOneWidget);
  });

  testWidgets('switch reverts and ip stays disabled when activation fails',
      (tester) async {
    final bridge = _UiTestBridge(
      initialState: const ClientViewState(
        signedIn: true,
        userLabel: 'tester@example.com',
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

    expect(_networkSwitch(tester).value, isTrue);

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
        signedIn: true,
        userLabel: 'tester@example.com',
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
        signedIn: true,
        userLabel: 'tester@example.com',
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
    expect(_networkSwitch(tester).value, isFalse);
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
        signedIn: true,
        userLabel: 'tester@example.com',
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
        signedIn: true,
        userLabel: 'tester@example.com',
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
        signedIn: true,
        userLabel: 'tester@example.com',
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
          dnsServers: ['10.0.0.1'],
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
        signedIn: true,
        userLabel: 'tester@example.com',
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

  testWidgets('signed in panel shows current device id', (tester) async {
    final bridge = _UiTestBridge(
      initialState: const ClientViewState(
        signedIn: true,
        userLabel: 'tester@example.com',
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

    expect(find.byKey(const Key('client-device-id-value')), findsOneWidget);
    expect(find.text('device-current'), findsOneWidget);
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
  ClientCommandType? lastCommand;
  Map<String, Object?>? lastPayload;
  int androidPrepareCount = 0;
  String serverUrl = 'http://127.0.0.1:28080';

  @override
  ValueListenable<ClientViewState> get state => _state;

  @override
  ValueListenable<AndroidNetworkAuthorizationState>
      get androidNetworkAuthorization => _androidNetworkAuthorization;

  @override
  Future<void> start() async {}

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
  Future<void> prepareAndroidNetworkAuthorization() async {
    androidPrepareCount += 1;
  }

  @override
  Future<void> dispatch(ClientCommand command) async {
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
    _state.value = _state.value.copyWith(
      networkEnabled: true,
      syncing: true,
      syncReason: command.type.name,
      switchEnabled: false,
      error: null,
      notice: null,
      clearVirtualIp: true,
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
          clearVirtualIp: true,
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
      networkEnabled: false,
      syncing: true,
      syncReason: command.type.name,
      switchEnabled: false,
      error: null,
      notice: null,
      clearVirtualIp: true,
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
        clearVirtualIp: true,
      );
    }));
  }

  @override
  Future<ControlTransportStatus?> localControlStatus() async => null;

  @override
  Future<void> close() async {}
}
