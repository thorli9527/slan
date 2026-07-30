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
  testWidgets('home omits shell header and runtime notice banner',
      (tester) async {
    final bridge = _UiTestBridge(
      initialState: const ClientViewState(
        signedIn: true,
        userLabel: 'tester@example.com',
        networkEnabled: false,
        syncing: false,
        switchEnabled: true,
        notice: 'signedIn',
      ),
      activationDelay: Duration.zero,
    );

    await tester.pumpWidget(SlanClientV2App(bridge: bridge));
    await tester.pumpAndSettle();

    expect(find.text('SLAN Client'), findsNothing);
    expect(find.text('signedIn'), findsNothing);
    expect(find.byKey(const Key('network-switch')), findsOneWidget);
    expect(find.text('用户'), findsOneWidget);
    expect(find.text('当前用户邮箱'), findsNothing);
    expect(find.text('tester@example.com'), findsOneWidget);
  });

  testWidgets('browser login blocks repeated clicks while opening',
      (tester) async {
    debugDefaultTargetPlatformOverride = TargetPlatform.macOS;
    try {
      final bridge = _UiTestBridge(
        initialState: ClientViewState.initial(),
        activationDelay: Duration.zero,
        browserCommandDelay: const Duration(milliseconds: 100),
      );

      await tester.pumpWidget(SlanClientV2App(bridge: bridge));
      await tester.pumpAndSettle();

      final button = find.byKey(const Key('desktop-browser-login'));
      await tester.tap(button);
      await tester.tap(button);
      await tester.pump();

      expect(bridge.browserCommandCount, 1);
      expect(find.text('正在打开'), findsOneWidget);
      expect(tester.widget<FilledButton>(button).onPressed, isNull);

      await tester.pump(bridge.browserCommandDelay);
      await tester.pumpAndSettle();
      expect(find.text('打开浏览器登录'), findsOneWidget);
      expect(tester.widget<FilledButton>(button).onPressed, isNotNull);
    } finally {
      debugDefaultTargetPlatformOverride = null;
    }
  });

  testWidgets('desktop network control omits enabled status label',
      (tester) async {
    debugDefaultTargetPlatformOverride = TargetPlatform.macOS;
    try {
      final bridge = _UiTestBridge(
        initialState: const ClientViewState(
          signedIn: true,
          userLabel: 'tester@example.com',
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

  testWidgets('desktop shows network docking below user and before ip',
      (tester) async {
    debugDefaultTargetPlatformOverride = TargetPlatform.macOS;
    try {
      final bridge = _UiTestBridge(
        initialState: const ClientViewState(
          signedIn: true,
          userLabel: 'tester@example.com',
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
      expect(find.byKey(const Key('network-docking-actions')), findsOneWidget);
      expect(find.byKey(const Key('network-docking-avatar')), findsOneWidget);
      final userY = tester.getTopLeft(find.text('tester@example.com')).dy;
      final dockingY = tester
          .getTopLeft(find.byKey(const Key('network-docking-actions')))
          .dy;
      final ipY =
          tester.getTopLeft(find.byKey(const Key('network-ip-value'))).dy;
      expect(dockingY, greaterThan(userY));
      expect(dockingY, lessThan(ipY));
      final userX = tester.getTopLeft(find.text('用户')).dx;
      final dockingX = tester.getTopLeft(find.text('网络对接')).dx;
      expect(dockingX, closeTo(userX, 2));
      expect(find.byKey(const Key('generate-network-invite')), findsNothing);
      expect(find.text('生成接入码'), findsNothing);
      expect(find.byKey(const Key('accept-network-invite')), findsOneWidget);
    } finally {
      debugDefaultTargetPlatformOverride = null;
    }
  });

  testWidgets('desktop signed-in layout fits the native content area',
      (tester) async {
    debugDefaultTargetPlatformOverride = TargetPlatform.macOS;
    try {
      await tester.binding.setSurfaceSize(const Size(480, 190));
      final bridge = _UiTestBridge(
        initialState: const ClientViewState(
          signedIn: true,
          userLabel: 'long.desktop.account@staticlss.com',
          virtualIp: '10.0.1.137',
          networkEnabled: true,
          syncing: false,
          switchEnabled: true,
        ),
        activationDelay: Duration.zero,
      );

      await tester.pumpWidget(SlanClientV2App(bridge: bridge));
      await tester.pumpAndSettle();

      final console = find.byKey(const Key('open-web-console'));
      final logout = find.text('退出登录');
      expect(console, findsOneWidget);
      expect(logout, findsOneWidget);
      expect(tester.getBottomRight(console).dy, lessThanOrEqualTo(190));
      expect(tester.getBottomRight(logout).dy, lessThanOrEqualTo(190));
      expect(190 - tester.getBottomRight(logout).dy, lessThanOrEqualTo(36));
      expect(tester.takeException(), isNull);
    } finally {
      debugDefaultTargetPlatformOverride = null;
      await tester.binding.setSurfaceSize(null);
    }
  });

  testWidgets('desktop signed-out layout stays compact', (tester) async {
    debugDefaultTargetPlatformOverride = TargetPlatform.macOS;
    try {
      await tester.binding.setSurfaceSize(const Size(480, 190));
      final bridge = _UiTestBridge(
        initialState: ClientViewState.initial(),
        activationDelay: Duration.zero,
      );

      await tester.pumpWidget(SlanClientV2App(bridge: bridge));
      await tester.pumpAndSettle();

      final login = find.byKey(const Key('desktop-browser-login'));
      expect(login, findsOneWidget);
      expect(tester.getBottomRight(login).dy, lessThanOrEqualTo(190));
      expect(190 - tester.getBottomRight(login).dy, lessThanOrEqualTo(16));
      expect(tester.takeException(), isNull);
    } finally {
      debugDefaultTargetPlatformOverride = null;
      await tester.binding.setSurfaceSize(null);
    }
  });

  testWidgets('network invite dialog validates and submits trimmed code',
      (tester) async {
    debugDefaultTargetPlatformOverride = TargetPlatform.macOS;
    try {
      final bridge = _UiTestBridge(
        initialState: const ClientViewState(
          signedIn: true,
          userLabel: 'tester@example.com',
          networkEnabled: true,
          syncing: false,
          switchEnabled: true,
        ),
        activationDelay: Duration.zero,
      );

      await tester.pumpWidget(SlanClientV2App(bridge: bridge));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const Key('accept-network-invite')));
      await tester.pumpAndSettle();

      expect(find.byKey(const Key('network-invite-dialog')), findsOneWidget);
      await tester.tap(find.byKey(const Key('network-invite-submit')));
      await tester.pump();
      expect(find.text('请输入接入码'), findsWidgets);
      expect(bridge.acceptedInviteCodes, isEmpty);

      await tester.enterText(
          find.byKey(const Key('network-invite-code-input')), '  JOIN-123  ');
      await tester.tap(find.byKey(const Key('network-invite-submit')));
      await tester.pumpAndSettle();

      expect(bridge.acceptedInviteCodes, ['JOIN-123']);
      expect(find.byKey(const Key('network-invite-dialog')), findsNothing);
      expect(find.text('网络接入已确认'), findsOneWidget);
    } finally {
      debugDefaultTargetPlatformOverride = null;
    }
  });

  testWidgets('network invite dialog keeps errors inline', (tester) async {
    debugDefaultTargetPlatformOverride = TargetPlatform.macOS;
    try {
      final bridge = _UiTestBridge(
        initialState: const ClientViewState(
          signedIn: true,
          userLabel: 'tester@example.com',
          networkEnabled: true,
          syncing: false,
          switchEnabled: true,
        ),
        activationDelay: Duration.zero,
        inviteError: '接入码已失效',
      );

      await tester.pumpWidget(SlanClientV2App(bridge: bridge));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const Key('accept-network-invite')));
      await tester.pumpAndSettle();
      await tester.enterText(
          find.byKey(const Key('network-invite-code-input')), 'EXPIRED');
      await tester.tap(find.byKey(const Key('network-invite-submit')));
      await tester.pumpAndSettle();

      expect(find.byKey(const Key('network-invite-dialog')), findsOneWidget);
      expect(find.byKey(const Key('network-invite-error')), findsOneWidget);
      expect(find.text('接入失败：接入码已失效'), findsOneWidget);
    } finally {
      debugDefaultTargetPlatformOverride = null;
    }
  });

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

  testWidgets('switch shows ip only after network activation completes',
      (tester) async {
    final bridge = _UiTestBridge(
      initialState: const ClientViewState(
        signedIn: true,
        userLabel: 'tester@example.com',
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
    expect(_networkSwitch(tester).value, isTrue);
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
        signedIn: true,
        userLabel: 'tester@example.com',
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
    expect(_networkSwitch(tester).value, isTrue);
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

  testWidgets('signed in panel omits current device id', (tester) async {
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

    expect(find.byKey(const Key('client-device-id-value')), findsNothing);
    expect(find.text('device-current'), findsNothing);
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
    this.inviteError,
    this.dispatchStartDelay = Duration.zero,
    this.browserCommandDelay = Duration.zero,
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
  final String? inviteError;
  final Duration dispatchStartDelay;
  final Duration browserCommandDelay;
  ClientCommandType? lastCommand;
  Map<String, Object?>? lastPayload;
  int androidPrepareCount = 0;
  int networkCommandCount = 0;
  int browserCommandCount = 0;
  final List<String> acceptedInviteCodes = [];
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
  Future<void> prepareAndroidNetworkAuthorization() async {
    androidPrepareCount += 1;
  }

  @override
  Future<void> acceptNetworkInvite(String inviteCode) async {
    acceptedInviteCodes.add(inviteCode);
    if (inviteError != null) throw Exception(inviteError);
  }

  @override
  Future<void> dispatch(ClientCommand command) async {
    if (command.type == ClientCommandType.openClientLogin ||
        command.type == ClientCommandType.openWebConsole) {
      browserCommandCount += 1;
      await Future<void>.delayed(browserCommandDelay);
    }
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
      networkEnabled: true,
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
      networkEnabled: false,
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
