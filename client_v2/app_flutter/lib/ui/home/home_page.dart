import 'dart:async';

import 'package:client_core_plugin/client_core_plugin.dart';
import 'package:flutter/material.dart';

import '../../bridge/android_network_authorization.dart';
import '../../bridge/client_commands.dart';
import '../../bridge/client_core_bridge.dart';
import '../../bridge/client_ui_diagnostics.dart';
import '../../bridge/client_view_state.dart';

class HomePage extends StatefulWidget {
  const HomePage({required this.bridge, super.key});

  final ClientCoreBridge bridge;

  @override
  State<HomePage> createState() => _HomePageState();
}

class _HomePageState extends State<HomePage> {
  static const String _clientPingPrefix = 'SLAN_PING:';
  static const String _clientPongPrefix = 'SLAN_PONG:';

  final TextEditingController _emailController = TextEditingController();
  final TextEditingController _passwordController = TextEditingController();
  final TextEditingController _messageTargetController =
      TextEditingController();
  final TextEditingController _messageBodyController = TextEditingController();
  final TextEditingController _pingTargetController = TextEditingController();
  final TextEditingController _serverBaseUrlController =
      TextEditingController();
  String? _lastDiagnosticsSnapshot;
  String? _lastShownError;
  String? _lastPingResult;
  String? _lastMessageSendResult;
  String? _serverBaseUrl;
  bool _lastSignedIn = false;
  bool _sendingMessage = false;
  bool _pinging = false;

  @override
  void initState() {
    super.initState();
    widget.bridge.state.addListener(_logStateChange);
    _logStateChange();
    unawaited(_loadServerBaseUrl());
    unawaited(_startBridge());
  }

  @override
  void dispose() {
    widget.bridge.state.removeListener(_logStateChange);
    _emailController.dispose();
    _passwordController.dispose();
    _messageTargetController.dispose();
    _messageBodyController.dispose();
    _pingTargetController.dispose();
    _serverBaseUrlController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: SafeArea(
        child: ValueListenableBuilder<ClientViewState>(
          valueListenable: widget.bridge.state,
          builder: (context, state, _) {
            return Align(
              alignment: Alignment.topCenter,
              child: SingleChildScrollView(
                padding: const EdgeInsets.fromLTRB(20, 12, 20, 20),
                child: ConstrainedBox(
                  constraints: const BoxConstraints(maxWidth: 560),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      if (state.signedIn) ...[
                        _buildSignedInHeader(state: state),
                        _buildAndroidAuthorizationPanel(),
                        const SizedBox(height: 14),
                        _ClientMessageComposer(
                          targetController: _messageTargetController,
                          bodyController: _messageBodyController,
                          syncing: state.syncing || _sendingMessage,
                          resultText: _lastMessageSendResult,
                          onSend: _sendClientMessage,
                        ),
                        const SizedBox(height: 14),
                        _ClientPingTool(
                          targetController: _pingTargetController,
                          syncing: state.syncing || _pinging,
                          resultText: _lastPingResult,
                          onPing: _pingClient,
                        ),
                        const SizedBox(height: 14),
                        _SignedInActions(
                          showConsole: _showWebConsoleAction,
                          onOpenConsole: _showWebConsoleAction
                              ? () => widget.bridge.dispatch(
                                    const ClientCommand(
                                      ClientCommandType.openWebConsole,
                                    ),
                                  )
                              : null,
                          onLogout: () => widget.bridge.dispatch(
                            const ClientCommand(ClientCommandType.logout),
                          ),
                        ),
                      ] else ...[
                        const _SignedOutStatus(),
                        const SizedBox(height: 14),
                        if (_usesPasswordLogin)
                          _PasswordLoginForm(
                            emailController: _emailController,
                            passwordController: _passwordController,
                            serverBaseUrl: _serverBaseUrl,
                            syncing: state.syncing,
                            onSettings: _showServerSettings,
                            onSubmit: _loginWithPassword,
                          )
                        else
                          SizedBox(
                            height: 40,
                            child: FilledButton(
                              onPressed: state.syncing
                                  ? null
                                  : () => widget.bridge.dispatch(
                                        const ClientCommand(
                                          ClientCommandType.openClientLogin,
                                        ),
                                      ),
                              child: const Text('Login'),
                            ),
                          ),
                      ],
                    ],
                  ),
                ),
              ),
            );
          },
        ),
      ),
    );
  }

  bool get _usesPasswordLogin =>
      Theme.of(context).platform == TargetPlatform.iOS ||
      Theme.of(context).platform == TargetPlatform.android;

  bool get _showWebConsoleAction => !_usesPasswordLogin;

  Future<void> _loadServerBaseUrl() async {
    final value = await widget.bridge.serverBaseUrl();
    if (!mounted) {
      return;
    }
    setState(() {
      _serverBaseUrl = value;
      _serverBaseUrlController.text = value;
    });
  }

  Future<void> _showServerSettings() async {
    _serverBaseUrlController.text =
        _serverBaseUrl ?? await widget.bridge.serverBaseUrl();
    final saved = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('服务器设置'),
        content: TextField(
          key: const Key('server-base-url'),
          controller: _serverBaseUrlController,
          keyboardType: TextInputType.url,
          textInputAction: TextInputAction.done,
          decoration: const InputDecoration(
            labelText: '服务器地址',
            hintText: 'http://example.com:28080',
            border: OutlineInputBorder(),
            isDense: true,
          ),
          onSubmitted: (_) => Navigator.of(context).pop(true),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(false),
            child: const Text('取消'),
          ),
          FilledButton(
            key: const Key('server-save'),
            onPressed: () => Navigator.of(context).pop(true),
            child: const Text('保存'),
          ),
        ],
      ),
    );
    if (saved != true || !mounted) {
      return;
    }
    final value = _serverBaseUrlController.text.trim();
    try {
      await widget.bridge.updateServerBaseUrl(value);
      final normalized = await widget.bridge.serverBaseUrl();
      if (!mounted) {
        return;
      }
      setState(() {
        _serverBaseUrl = normalized;
        _serverBaseUrlController.text = normalized;
      });
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('服务器已设置为 $normalized')),
      );
    } catch (error) {
      if (!mounted) {
        return;
      }
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('保存服务器失败：$error')),
      );
    }
  }

  Future<void> _loginWithPassword() async {
    final email = _emailController.text.trim();
    final password = _passwordController.text;
    ClientUiDiagnostics.unawaitedLog(
      'home.login.submit',
      state: widget.bridge.state.value,
      fields: {
        'hasEmail': email.isNotEmpty,
        'hasPassword': password.isNotEmpty,
      },
    );
    if (email.isEmpty || password.isEmpty) {
      _lastShownError = null;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('请输入账号和密码')),
      );
      return;
    }
    try {
      await widget.bridge.dispatch(
        ClientCommand(
          ClientCommandType.loginWithPassword,
          {
            'email': email,
            'password': password,
          },
        ),
      );
    } catch (error) {
      if (!mounted) {
        return;
      }
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('登录失败：$error')),
      );
    }
  }

  void _toggleNetwork(bool enabled) {
    _lastShownError = null;
    ClientUiDiagnostics.unawaitedLog(
      'home.switch.tap',
      state: widget.bridge.state.value,
      fields: {'targetEnabled': enabled},
    );
    widget.bridge.dispatch(
      ClientCommand(
        enabled
            ? ClientCommandType.enableNetwork
            : ClientCommandType.disableNetwork,
      ),
    );
  }

  Future<void> _pingClient() async {
    if (_pinging) {
      return;
    }
    final targetDeviceId = _pingTargetController.text.trim();
    if (targetDeviceId.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('请输入目标设备 ID')),
      );
      return;
    }
    final sentAtMs = DateTime.now().millisecondsSinceEpoch;
    final pingId = 'ping-$sentAtMs';
    setState(() {
      _pinging = true;
      _lastPingResult = '等待响应...';
    });
    try {
      await widget.bridge.dispatch(
        ClientCommand(
          ClientCommandType.sendClientMessage,
          {
            'targetDeviceId': targetDeviceId,
            'body': '$_clientPingPrefix$pingId:$sentAtMs',
            'metadata': {
              'kind': 'client_ping',
              'pingId': pingId,
              'sentAtMs': sentAtMs,
            },
          },
        ),
      );
      final rttMs = await _waitForClientPong(
        targetDeviceId: targetDeviceId,
        pingId: pingId,
        sentAtMs: sentAtMs,
      );
      if (!mounted) {
        return;
      }
      setState(() {
        _lastPingResult = '来自 $targetDeviceId：${rttMs}ms';
      });
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Ping 成功：${rttMs}ms')),
      );
    } catch (error) {
      if (!mounted) {
        return;
      }
      setState(() {
        _lastPingResult = '失败：$error';
      });
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Ping 失败：$error')),
      );
    } finally {
      if (mounted) {
        setState(() => _pinging = false);
      }
    }
  }

  Future<void> _sendClientMessage() async {
    if (_sendingMessage) {
      return;
    }
    final targetDeviceId = _messageTargetController.text.trim();
    final body = _messageBodyController.text.trim();
    if (targetDeviceId.isEmpty || body.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('请输入目标设备 ID 和消息内容')),
      );
      return;
    }
    setState(() {
      _sendingMessage = true;
      _lastMessageSendResult = null;
    });
    try {
      await widget.bridge.dispatch(
        ClientCommand(
          ClientCommandType.sendClientMessage,
          {
            'targetDeviceId': targetDeviceId,
            'body': body,
          },
        ),
      );
      if (!mounted) {
        return;
      }
      _messageBodyController.clear();
      setState(() {
        _lastMessageSendResult = '消息已发送';
      });
    } catch (error) {
      if (!mounted) {
        return;
      }
      setState(() {
        _lastMessageSendResult = '消息发送失败：$error';
      });
    } finally {
      if (mounted) {
        setState(() => _sendingMessage = false);
      }
    }
  }

  Future<int> _waitForClientPong({
    required String targetDeviceId,
    required String pingId,
    required int sentAtMs,
  }) async {
    final deadline = DateTime.now().add(const Duration(seconds: 15));
    while (DateTime.now().isBefore(deadline)) {
      final state = widget.bridge.state.value;
      final body = state.lastClientMessageBody?.trim() ?? '';
      final from = state.lastClientMessageFromDeviceId?.trim() ?? '';
      final rtt = _pongRttMs(
        body: body,
        fromDeviceId: from,
        expectedDeviceId: targetDeviceId,
        expectedPingId: pingId,
        sentAtMs: sentAtMs,
      );
      if (rtt != null) {
        return rtt;
      }
      await Future<void>.delayed(const Duration(milliseconds: 250));
    }
    throw TimeoutException('等待 Ping 响应超时');
  }

  int? _pongRttMs({
    required String body,
    required String fromDeviceId,
    required String expectedDeviceId,
    required String expectedPingId,
    required int sentAtMs,
  }) {
    if (fromDeviceId != expectedDeviceId ||
        !body.startsWith(_clientPongPrefix)) {
      return null;
    }
    final parts = body.substring(_clientPongPrefix.length).split(':');
    if (parts.length < 2 ||
        parts[0] != expectedPingId ||
        int.tryParse(parts[1]) != sentAtMs) {
      return null;
    }
    final nowMs = DateTime.now().millisecondsSinceEpoch;
    return nowMs >= sentAtMs ? nowMs - sentAtMs : 0;
  }

  Future<void> _startBridge() async {
    await widget.bridge.start();
    await widget.bridge.prepareAndroidNetworkAuthorization();
  }

  void _logStateChange() {
    final state = widget.bridge.state.value;
    final snapshot = [
      state.signedIn,
      state.networkEnabled,
      state.syncing,
      state.syncReason,
      state.switchEnabled,
      state.virtualIp,
      state.notice,
      state.error,
      state.errorSource,
    ].join('|');
    if (_lastDiagnosticsSnapshot == snapshot) {
      return;
    }
    _lastDiagnosticsSnapshot = snapshot;
    ClientUiDiagnostics.unawaitedLog('home.state.changed', state: state);
    if (state.signedIn && !_lastSignedIn) {
      unawaited(widget.bridge.prepareAndroidNetworkAuthorization());
    }
    _lastSignedIn = state.signedIn;
    _showErrorDialogIfNeeded(state);
  }

  void _showErrorDialogIfNeeded(ClientViewState state) {
    if (state.errorSource != ClientErrorSource.networkSwitch) {
      return;
    }
    final error = state.error?.trim();
    if (error == null || error.isEmpty || error == _lastShownError) {
      return;
    }
    _lastShownError = error;
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) {
        return;
      }
      showDialog<void>(
        context: context,
        builder: (context) {
          return Dialog(
            insetPadding: const EdgeInsets.symmetric(
              horizontal: 18,
              vertical: 12,
            ),
            child: ConstrainedBox(
              constraints: const BoxConstraints(
                maxWidth: 320,
                maxHeight: 180,
              ),
              child: Padding(
                padding: const EdgeInsets.fromLTRB(18, 16, 18, 12),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    const Text(
                      '操作失败',
                      style: TextStyle(
                        fontSize: 20,
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                    const SizedBox(height: 10),
                    Flexible(
                      child: SingleChildScrollView(
                        child: Text(
                          _friendlyError(error),
                          style: const TextStyle(fontSize: 14, height: 1.35),
                        ),
                      ),
                    ),
                    const SizedBox(height: 8),
                    Align(
                      alignment: Alignment.centerRight,
                      child: TextButton(
                        onPressed: () => Navigator.of(context).pop(),
                        child: const Text('确定'),
                      ),
                    ),
                  ],
                ),
              ),
            ),
          );
        },
      );
    });
  }

  String _friendlyError(String error) {
    final normalized = error.toLowerCase();
    if (normalized.contains('disabled by network admin') ||
        normalized.contains('disabled by network administrator') ||
        normalized.contains('has been disabled') ||
        normalized.contains('current device is disabled') ||
        normalized.contains('suspended') ||
        normalized.contains('revoked') ||
        normalized.contains('blocked')) {
      return '服务端停用，请联系管理员。';
    }
    if (normalized.contains('device unavailable')) {
      return '设备不可用，请联系管理员重新启用。';
    }
    if (normalized.contains('no active network attachment')) {
      return '当前设备没有可用的网络绑定，请先在 Web Console 中绑定设备。';
    }
    if (normalized.contains('session expired')) {
      return '登录状态已失效，请重新登录。';
    }
    if (normalized.contains('local service')) {
      return '本地服务未连接，请稍后重试或重启客户端。';
    }
    return error;
  }

  Widget _buildSignedInHeader({
    required ClientViewState state,
  }) {
    return _SignedInStatusPanel(
      userLabel: _userLabel(state),
      currentIp: _ipText(state),
      state: state,
      onToggle: _toggleNetwork,
    );
  }

  Widget _buildAndroidAuthorizationPanel() {
    return ValueListenableBuilder<AndroidNetworkAuthorizationState>(
      valueListenable: widget.bridge.androidNetworkAuthorization,
      builder: (context, authorization, _) {
        if (!authorization.visible) {
          return const SizedBox.shrink();
        }
        return Padding(
          padding: const EdgeInsets.only(top: 8),
          child: _AndroidAuthorizationPanel(
            authorization: authorization,
            onRefresh: widget.bridge.prepareAndroidNetworkAuthorization,
          ),
        );
      },
    );
  }

  String _userLabel(ClientViewState state) {
    final user = state.userLabel?.trim();
    return user == null || user.isEmpty ? '-' : user;
  }

  String _ipText(ClientViewState state) {
    final virtualIp = state.virtualIp?.trim();
    if (!state.signedIn ||
        !state.networkEnabled ||
        virtualIp == null ||
        virtualIp.isEmpty) {
      return '未启用';
    }
    return virtualIp;
  }
}

class _SignedInStatusPanel extends StatelessWidget {
  const _SignedInStatusPanel({
    required this.userLabel,
    required this.currentIp,
    required this.state,
    required this.onToggle,
  });

  final String userLabel;
  final String currentIp;
  final ClientViewState state;
  final ValueChanged<bool> onToggle;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Container(
      padding: const EdgeInsets.fromLTRB(14, 12, 14, 10),
      decoration: BoxDecoration(
        color: theme.colorScheme.surfaceContainerLowest,
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: theme.colorScheme.outlineVariant),
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.center,
            children: [
              Expanded(child: _CompactIdentity(userLabel: userLabel)),
              const SizedBox(width: 12),
              _NetworkControl(state: state, onToggle: onToggle),
            ],
          ),
          const SizedBox(height: 10),
          Divider(height: 1, color: theme.colorScheme.outlineVariant),
          const SizedBox(height: 9),
          _CompactInfoRow(
            valueKey: const Key('network-ip-value'),
            icon: Icons.router_rounded,
            label: '当前 IP',
            value: currentIp,
          ),
          if (_hasTraffic(state)) ...[
            const SizedBox(height: 7),
            _CompactInfoRow(
              valueKey: const Key('client-traffic-total-value'),
              icon: Icons.speed_rounded,
              label: '已用',
              value: _trafficTotalText(state),
            ),
            const SizedBox(height: 7),
            _CompactInfoRow(
              valueKey: const Key('client-traffic-current-value'),
              icon: Icons.swap_vert_rounded,
              label: '当前',
              value: _trafficCurrentText(state),
            ),
          ],
          if (_deviceIdText(state) != null) ...[
            const SizedBox(height: 7),
            _CompactInfoRow(
              valueKey: const Key('client-device-id-value'),
              icon: Icons.devices_other_rounded,
              label: '设备 ID',
              value: _deviceIdText(state)!,
            ),
          ],
          if (_lastClientMessageText(state) != null) ...[
            const SizedBox(height: 7),
            _CompactInfoRow(
              valueKey: const Key('last-client-message-value'),
              icon: Icons.mark_chat_unread_rounded,
              label: '最近消息',
              value: _lastClientMessageText(state)!,
            ),
          ],
        ],
      ),
    );
  }

  String? _lastClientMessageText(ClientViewState state) {
    final body = state.lastClientMessageBody?.trim();
    if (body == null || body.isEmpty) {
      return null;
    }
    final from = state.lastClientMessageFromDeviceId?.trim();
    if (from == null || from.isEmpty) {
      return body;
    }
    return '$from: $body';
  }

  String? _deviceIdText(ClientViewState state) {
    final deviceId = state.deviceId?.trim();
    if (deviceId == null || deviceId.isEmpty) {
      return null;
    }
    return deviceId;
  }

  bool _hasTraffic(ClientViewState state) {
    return state.trafficTxBytes != null || state.trafficRxBytes != null;
  }

  String _trafficTotalText(ClientViewState state) {
    final txTotal = _formatBytes(state.trafficTxBytes ?? 0);
    final rxTotal = _formatBytes(state.trafficRxBytes ?? 0);
    return '↑$txTotal ↓$rxTotal';
  }

  String _trafficCurrentText(ClientViewState state) {
    final txRate = _formatBytes(state.trafficTxBytesPerMinute ?? 0);
    final rxRate = _formatBytes(state.trafficRxBytesPerMinute ?? 0);
    return '↑$txRate/分 ↓$rxRate/分';
  }

  String _formatBytes(int bytes) {
    if (bytes >= 1024 * 1024) {
      return '${(bytes / (1024 * 1024)).toStringAsFixed(bytes >= 10 * 1024 * 1024 ? 0 : 1)}M';
    }
    if (bytes >= 1024) {
      return '${(bytes / 1024).toStringAsFixed(bytes >= 10 * 1024 ? 0 : 1)}K';
    }
    return '${bytes}B';
  }
}

class _NetworkControl extends StatelessWidget {
  const _NetworkControl({required this.state, required this.onToggle});

  final ClientViewState state;
  final ValueChanged<bool> onToggle;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final enabled = state.networkEnabled;
    final label = state.syncing
        ? '同步中'
        : enabled
            ? '网络已启用'
            : '网络未启用';
    return SizedBox(
      width: 112,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.end,
        mainAxisSize: MainAxisSize.min,
        children: [
          _NetworkSwitch(state: state, onToggle: onToggle),
          const SizedBox(height: 3),
          Text(
            label,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: theme.textTheme.labelSmall?.copyWith(
              color: enabled
                  ? theme.colorScheme.primary
                  : theme.colorScheme.onSurfaceVariant,
              fontWeight: FontWeight.w800,
            ),
          ),
        ],
      ),
    );
  }
}

class _AndroidAuthorizationPanel extends StatelessWidget {
  const _AndroidAuthorizationPanel({
    required this.authorization,
    required this.onRefresh,
  });

  final AndroidNetworkAuthorizationState authorization;
  final VoidCallback onRefresh;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final config = authorization.networkConfig;
    final error = authorization.error?.trim();
    final status = _statusText();
    final statusColor = error != null && error.isNotEmpty
        ? theme.colorScheme.error
        : authorization.granted
            ? theme.colorScheme.primary
            : theme.colorScheme.onSurfaceVariant;
    return Container(
      padding: const EdgeInsets.fromLTRB(12, 9, 12, 9),
      decoration: BoxDecoration(
        color: theme.colorScheme.surfaceContainerLowest,
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: theme.colorScheme.outlineVariant),
      ),
      child: Row(
        children: [
          Icon(
            authorization.granted
                ? Icons.verified_user_outlined
                : Icons.vpn_key_outlined,
            size: 18,
            color: statusColor,
          ),
          const SizedBox(width: 8),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  status,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: theme.textTheme.labelMedium?.copyWith(
                    color: statusColor,
                    fontWeight: FontWeight.w900,
                  ),
                ),
                if (config != null || (error != null && error.isNotEmpty)) ...[
                  const SizedBox(height: 2),
                  Text(
                    error != null && error.isNotEmpty
                        ? _compactError(error)
                        : _configText(config!),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: theme.textTheme.labelSmall?.copyWith(
                      color: theme.colorScheme.onSurfaceVariant,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                ],
              ],
            ),
          ),
          if (authorization.checking)
            SizedBox(
              width: 18,
              height: 18,
              child: CircularProgressIndicator(
                strokeWidth: 2,
                color: theme.colorScheme.primary,
              ),
            )
          else if (authorization.needsUserConsent)
            SizedBox(
              height: 30,
              child: FilledButton.icon(
                onPressed: onRefresh,
                icon: const Icon(Icons.check_circle_outline, size: 16),
                label: const Text('授权'),
              ),
            )
          else
            IconButton(
              tooltip: '刷新',
              onPressed: onRefresh,
              icon: const Icon(Icons.refresh_rounded, size: 18),
            ),
        ],
      ),
    );
  }

  String _statusText() {
    final error = authorization.error?.trim();
    if (error != null && error.isNotEmpty) {
      return 'Android 网络配置异常';
    }
    if (authorization.checking) {
      return 'Android 网络检查中';
    }
    if (authorization.needsUserConsent) {
      return 'Android 网络待授权';
    }
    if (authorization.granted && authorization.networkConfig != null) {
      return 'Android 网络配置已就绪';
    }
    if (authorization.granted) {
      return 'Android 网络已授权';
    }
    return 'Android 网络状态待确认';
  }

  String _configText(AndroidVpnSessionConfig config) {
    final dns = config.dnsServers.isEmpty ? '-' : config.dnsServers.join(',');
    return '${config.virtualIp}/${config.prefixLen}  DNS $dns';
  }

  String _compactError(String error) {
    return error.replaceAll(RegExp(r'\s+'), ' ');
  }
}

class _NetworkSwitch extends StatelessWidget {
  const _NetworkSwitch({required this.state, required this.onToggle});

  final ClientViewState state;
  final ValueChanged<bool> onToggle;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final switchBusy = state.syncing && !state.switchEnabled;
    return Align(
      alignment: Alignment.centerRight,
      child: Row(
        mainAxisAlignment: MainAxisAlignment.end,
        mainAxisSize: MainAxisSize.min,
        children: [
          SizedBox(
            width: 20,
            height: 16,
            child: switchBusy
                ? CircularProgressIndicator(
                    strokeWidth: 2,
                    color: theme.colorScheme.primary,
                  )
                : null,
          ),
          const SizedBox(width: 4),
          SizedBox(
            height: 36,
            child: Switch(
              key: const Key('network-switch'),
              value: state.networkEnabled,
              onChanged:
                  state.switchEnabled && !state.syncing ? onToggle : null,
            ),
          ),
        ],
      ),
    );
  }
}

class _CompactIdentity extends StatelessWidget {
  const _CompactIdentity({required this.userLabel});

  final String userLabel;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Row(
      children: [
        Icon(
          Icons.account_circle_outlined,
          size: 24,
          color: theme.colorScheme.primary,
        ),
        const SizedBox(width: 9),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(
                '当前用户邮箱',
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: theme.textTheme.labelMedium?.copyWith(
                  color: theme.colorScheme.onSurfaceVariant,
                  fontWeight: FontWeight.w800,
                ),
              ),
              const SizedBox(height: 2),
              Text(
                userLabel,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: theme.textTheme.bodyMedium?.copyWith(
                  fontWeight: FontWeight.w900,
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }
}

class _CompactInfoRow extends StatelessWidget {
  const _CompactInfoRow({
    this.valueKey,
    required this.icon,
    required this.label,
    required this.value,
  });

  final Key? valueKey;
  final IconData icon;
  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Row(
      children: [
        Icon(icon, size: 18, color: theme.colorScheme.primary),
        const SizedBox(width: 9),
        Text(
          label,
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
          style: theme.textTheme.labelMedium?.copyWith(
            color: theme.colorScheme.onSurfaceVariant,
            fontWeight: FontWeight.w800,
          ),
        ),
        const SizedBox(width: 12),
        Expanded(
          child: Text(
            key: valueKey,
            value,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            textAlign: TextAlign.right,
            style: theme.textTheme.bodyMedium?.copyWith(
              fontWeight: FontWeight.w900,
            ),
          ),
        ),
      ],
    );
  }
}

class _SignedOutStatus extends StatelessWidget {
  const _SignedOutStatus();

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
      decoration: BoxDecoration(
        border: Border.all(color: const Color(0xffe2d6cf)),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            '登录后启用组网',
            style: Theme.of(context).textTheme.titleMedium?.copyWith(
                  fontWeight: FontWeight.w700,
                ),
          ),
        ],
      ),
    );
  }
}

class _SignedInActions extends StatelessWidget {
  const _SignedInActions({
    required this.showConsole,
    required this.onOpenConsole,
    required this.onLogout,
  });

  final bool showConsole;
  final VoidCallback? onOpenConsole;
  final VoidCallback onLogout;

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        if (showConsole) ...[
          Expanded(
            child: SizedBox(
              height: 38,
              child: OutlinedButton.icon(
                onPressed: onOpenConsole,
                icon: const Icon(Icons.open_in_browser_rounded, size: 17),
                label: const Text('Web Console'),
              ),
            ),
          ),
          const SizedBox(width: 8),
        ],
        Expanded(
          child: SizedBox(
            height: 38,
            child: OutlinedButton.icon(
              onPressed: onLogout,
              icon: const Icon(Icons.logout_rounded, size: 17),
              label: const Text('Logout'),
            ),
          ),
        ),
      ],
    );
  }
}

class _ClientPingTool extends StatelessWidget {
  const _ClientPingTool({
    required this.targetController,
    required this.syncing,
    required this.resultText,
    required this.onPing,
  });

  final TextEditingController targetController;
  final bool syncing;
  final String? resultText;
  final Future<void> Function() onPing;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Container(
      padding: const EdgeInsets.fromLTRB(12, 10, 12, 12),
      decoration: BoxDecoration(
        color: theme.colorScheme.surfaceContainerLowest,
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: theme.colorScheme.outlineVariant),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          Row(
            children: [
              Icon(
                Icons.network_ping_rounded,
                size: 18,
                color: theme.colorScheme.primary,
              ),
              const SizedBox(width: 8),
              Text(
                'Ping 工具',
                style: theme.textTheme.labelLarge?.copyWith(
                  fontWeight: FontWeight.w900,
                ),
              ),
            ],
          ),
          const SizedBox(height: 9),
          TextField(
            key: const Key('client-ping-target'),
            controller: targetController,
            enabled: !syncing,
            textInputAction: TextInputAction.send,
            onSubmitted: (_) {
              if (!syncing) {
                unawaited(onPing());
              }
            },
            decoration: const InputDecoration(
              labelText: '目标设备 ID',
              border: OutlineInputBorder(),
              isDense: true,
            ),
          ),
          const SizedBox(height: 8),
          Row(
            children: [
              Expanded(
                child: Text(
                  resultText?.trim().isNotEmpty == true
                      ? resultText!.trim()
                      : '输入目标设备 ID 后检测连通性',
                  key: const Key('client-ping-result'),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: theme.textTheme.labelMedium?.copyWith(
                    color: theme.colorScheme.onSurfaceVariant,
                    fontWeight: FontWeight.w800,
                  ),
                ),
              ),
              const SizedBox(width: 8),
              SizedBox(
                height: 42,
                child: FilledButton.icon(
                  key: const Key('client-ping-send'),
                  onPressed: syncing ? null : () => unawaited(onPing()),
                  icon: const Icon(Icons.network_ping_rounded, size: 17),
                  label: Text(syncing ? '检测中' : 'Ping'),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _ClientMessageComposer extends StatelessWidget {
  const _ClientMessageComposer({
    required this.targetController,
    required this.bodyController,
    required this.syncing,
    required this.resultText,
    required this.onSend,
  });

  final TextEditingController targetController;
  final TextEditingController bodyController;
  final bool syncing;
  final String? resultText;
  final Future<void> Function() onSend;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Container(
      padding: const EdgeInsets.fromLTRB(12, 10, 12, 12),
      decoration: BoxDecoration(
        color: theme.colorScheme.surfaceContainerLowest,
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: theme.colorScheme.outlineVariant),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          Row(
            children: [
              Icon(
                Icons.send_to_mobile_rounded,
                size: 18,
                color: theme.colorScheme.primary,
              ),
              const SizedBox(width: 8),
              Text(
                '设备消息',
                style: theme.textTheme.labelLarge?.copyWith(
                  fontWeight: FontWeight.w900,
                ),
              ),
            ],
          ),
          const SizedBox(height: 9),
          TextField(
            key: const Key('client-message-target'),
            controller: targetController,
            enabled: !syncing,
            textInputAction: TextInputAction.next,
            decoration: const InputDecoration(
              labelText: '目标设备 ID',
              border: OutlineInputBorder(),
              isDense: true,
            ),
          ),
          const SizedBox(height: 8),
          TextField(
            key: const Key('client-message-body'),
            controller: bodyController,
            enabled: !syncing,
            textInputAction: TextInputAction.send,
            onSubmitted: (_) {
              if (!syncing) {
                unawaited(onSend());
              }
            },
            decoration: const InputDecoration(
              labelText: '消息',
              border: OutlineInputBorder(),
              isDense: true,
            ),
          ),
          const SizedBox(height: 8),
          Row(
            children: [
              Expanded(
                child: Text(
                  resultText?.trim().isNotEmpty == true ? resultText! : '',
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: theme.textTheme.labelMedium?.copyWith(
                    color: theme.colorScheme.onSurfaceVariant,
                    fontWeight: FontWeight.w800,
                  ),
                ),
              ),
              const SizedBox(width: 8),
              SizedBox(
                height: 42,
                child: FilledButton.icon(
                  key: const Key('client-message-send'),
                  onPressed: syncing ? null : () => unawaited(onSend()),
                  icon: const Icon(Icons.send_rounded, size: 17),
                  label: Text(syncing ? '发送中' : '发送'),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _PasswordLoginForm extends StatelessWidget {
  const _PasswordLoginForm({
    required this.emailController,
    required this.passwordController,
    required this.serverBaseUrl,
    required this.syncing,
    required this.onSettings,
    required this.onSubmit,
  });

  final TextEditingController emailController;
  final TextEditingController passwordController;
  final String? serverBaseUrl;
  final bool syncing;
  final VoidCallback onSettings;
  final VoidCallback onSubmit;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        _ServerSettingsSummary(
          serverBaseUrl: serverBaseUrl,
          syncing: syncing,
          onSettings: onSettings,
        ),
        const SizedBox(height: 10),
        TextField(
          key: const Key('login-email'),
          controller: emailController,
          enabled: !syncing,
          keyboardType: TextInputType.emailAddress,
          textInputAction: TextInputAction.next,
          decoration: const InputDecoration(
            labelText: '账号',
            border: OutlineInputBorder(),
            isDense: true,
          ),
        ),
        const SizedBox(height: 10),
        TextField(
          key: const Key('login-password'),
          controller: passwordController,
          enabled: !syncing,
          obscureText: true,
          textInputAction: TextInputAction.done,
          onSubmitted: (_) => syncing ? null : onSubmit(),
          decoration: const InputDecoration(
            labelText: '密码',
            border: OutlineInputBorder(),
            isDense: true,
          ),
        ),
        const SizedBox(height: 12),
        SizedBox(
          height: 42,
          child: FilledButton(
            key: const Key('login-submit'),
            onPressed: syncing ? null : onSubmit,
            child: Text(syncing ? '登录中' : '登录'),
          ),
        ),
      ],
    );
  }
}

class _ServerSettingsSummary extends StatelessWidget {
  const _ServerSettingsSummary({
    required this.serverBaseUrl,
    required this.syncing,
    required this.onSettings,
  });

  final String? serverBaseUrl;
  final bool syncing;
  final VoidCallback onSettings;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final value = serverBaseUrl?.trim();
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: theme.colorScheme.outlineVariant),
      ),
      child: Row(
        children: [
          Icon(
            Icons.dns_rounded,
            size: 18,
            color: theme.colorScheme.primary,
          ),
          const SizedBox(width: 8),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  '服务器',
                  style: theme.textTheme.labelSmall?.copyWith(
                    color: theme.colorScheme.onSurfaceVariant,
                    fontWeight: FontWeight.w800,
                  ),
                ),
                Text(
                  value == null || value.isEmpty ? '未设置' : value,
                  key: const Key('server-base-url-value'),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: theme.textTheme.bodyMedium?.copyWith(
                    fontWeight: FontWeight.w800,
                  ),
                ),
              ],
            ),
          ),
          IconButton(
            key: const Key('server-settings'),
            tooltip: '服务器设置',
            onPressed: syncing ? null : onSettings,
            icon: const Icon(Icons.settings_rounded),
          ),
        ],
      ),
    );
  }
}
