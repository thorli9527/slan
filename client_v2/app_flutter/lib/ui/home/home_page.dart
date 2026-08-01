import 'dart:async';

import 'package:flutter/material.dart';

import '../../bridge/android_network_authorization.dart';
import '../../bridge/client_commands.dart';
import '../../bridge/client_core_bridge.dart';
import '../../bridge/client_ui_diagnostics.dart';
import '../../bridge/client_view_state.dart';
import 'widgets/android_authorization_panel.dart';
import 'widgets/network_status_panel.dart';
import 'widgets/password_login_form.dart';
import 'widgets/signed_in_actions.dart';
import 'widgets/signed_out_status.dart';

/// SLAN 桌面/移动客户端首页。
///
/// 页面只负责呈现状态和收集用户输入；登录和网络开关等
/// 实际业务都通过 [ClientCoreBridge] 派发到本地服务或移动端原生插件。
class HomePage extends StatefulWidget {
  const HomePage({required this.bridge, super.key});

  /// 客户端核心桥接对象，屏蔽 macOS/Windows/Linux/Android/iOS 的实现差异。
  final ClientCoreBridge bridge;

  @override
  State<HomePage> createState() => _HomePageState();
}

/// 首页内部状态。
///
/// 这里保存表单输入、临时操作结果和诊断快照；可持久化业务状态统一来自 bridge。
class _HomePageState extends State<HomePage> with WidgetsBindingObserver {
  static const _connectivityRecoveryBackgroundThreshold = Duration(seconds: 3);

  /// 密码登录账号输入框。
  final TextEditingController _emailController = TextEditingController();

  /// 密码登录密码输入框。
  final TextEditingController _passwordController = TextEditingController();

  /// 控制面 API 地址输入框。
  final TextEditingController _serverBaseUrlController =
      TextEditingController();

  /// 最近一次已记录的状态快照，用于避免重复写诊断日志。
  String? _lastDiagnosticsSnapshot;

  /// 最近一次已弹窗展示的网络错误，避免同一错误反复弹窗。
  String? _lastShownError;

  /// 当前控制面 API 地址。
  String? _serverBaseUrl;

  /// 上一次登录态，用于从未登录变已登录时触发平台授权准备。
  bool _lastSignedIn = false;

  /// UI 刚派发的网络目标；非空时同时表示操作锁和乐观显示值。
  bool? _pendingNetworkTarget;

  DateTime? _backgroundedAt;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    widget.bridge.state.addListener(_logStateChange);
    _logStateChange();
    unawaited(_loadServerBaseUrl());
    unawaited(_startBridge());
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    widget.bridge.state.removeListener(_logStateChange);
    _emailController.dispose();
    _passwordController.dispose();
    _serverBaseUrlController.dispose();
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    switch (state) {
      case AppLifecycleState.paused:
      case AppLifecycleState.hidden:
      case AppLifecycleState.detached:
        _backgroundedAt ??= DateTime.now();
        break;
      case AppLifecycleState.resumed:
        final backgroundedAt = _backgroundedAt;
        _backgroundedAt = null;
        if (backgroundedAt != null &&
            DateTime.now().difference(backgroundedAt) >=
                _connectivityRecoveryBackgroundThreshold) {
          unawaited(widget.bridge.notifyAppResumed());
        }
        break;
      case AppLifecycleState.inactive:
        break;
    }
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
                padding: EdgeInsets.fromLTRB(
                  _isDesktopLike ? 18 : 20,
                  _isDesktopLike && state.hasUserSession ? 26 : 10,
                  _isDesktopLike ? 18 : 20,
                  _isDesktopLike ? 10 : 24,
                ),
                child: ConstrainedBox(
                  constraints: BoxConstraints(
                    maxWidth: _isDesktopLike ? 520 : 560,
                  ),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      if (state.hasUserSession) ...[
                        _buildSignedInHeader(state: state),
                        _buildAndroidAuthorizationPanel(),
                        SizedBox(height: _isDesktopLike ? 8 : 14),
                        SignedInActions(
                          desktop: _isDesktopLike,
                          onLogout: () => widget.bridge.dispatch(
                            const ClientCommand(ClientCommandType.logout),
                          ),
                        ),
                      ] else ...[
                        if (!_isDesktopLike) ...[
                          const SignedOutStatus(),
                          const SizedBox(height: 14),
                        ],
                        PasswordLoginForm(
                          desktop: _isDesktopLike,
                          emailController: _emailController,
                          passwordController: _passwordController,
                          serverBaseUrl: _serverBaseUrl,
                          syncing: state.syncing,
                          onSettings: _showServerSettings,
                          onSubmit: _loginWithPassword,
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

  bool get _isDesktopLike =>
      Theme.of(context).platform != TargetPlatform.iOS &&
      Theme.of(context).platform != TargetPlatform.android;

  /// 读取当前服务端地址并同步到设置输入框。
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

  /// 弹出服务器地址设置。
  Future<void> _showServerSettings() async {
    _serverBaseUrlController.text =
        _serverBaseUrl ?? await widget.bridge.serverBaseUrl();
    final saved = await showDialog<bool>(
      context: context,
      builder: (context) => _ServerSettingsDialog(
        controller: _serverBaseUrlController,
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
    } catch (error) {
      if (!mounted) {
        return;
      }
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('保存服务器失败：$error')),
      );
    }
  }

  /// 账号密码登录。
  ///
  /// 成功后 bridge 会完成设备注册并启动 MQTT 控制通道，页面只负责显示错误。
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
        const SnackBar(content: Text('请输入用户名和密码')),
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

  /// 处理用户点击网络开关。
  void _toggleNetwork(bool enabled) {
    final state = widget.bridge.state.value;
    if (_pendingNetworkTarget != null ||
        state.syncing ||
        !state.switchEnabled) {
      ClientUiDiagnostics.unawaitedLog(
        'home.switch.ignoredInFlight',
        state: state,
        fields: {'targetEnabled': enabled},
      );
      return;
    }
    setState(() {
      _pendingNetworkTarget = enabled;
    });
    _lastShownError = null;
    ClientUiDiagnostics.unawaitedLog(
      'home.switch.tap',
      state: widget.bridge.state.value,
      fields: {'targetEnabled': enabled},
    );
    unawaited(_dispatchNetworkToggle(enabled));
  }

  Future<void> _dispatchNetworkToggle(bool enabled) async {
    try {
      await widget.bridge.dispatch(
        ClientCommand(
          enabled
              ? ClientCommandType.enableNetwork
              : ClientCommandType.disableNetwork,
        ),
      );
    } finally {
      if (!mounted) return;
      setState(() {
        _pendingNetworkTarget = null;
      });
    }
  }

  /// 启动 bridge 并准备 Android 授权状态。
  Future<void> _startBridge() async {
    await widget.bridge.start();
    await widget.bridge.prepareAndroidNetworkAuthorization();
  }

  /// 监听 bridge 状态变化，写入诊断日志并处理一次性 UI 副作用。
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

  /// 网络切换失败时展示对用户友好的错误弹窗。
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

  /// 将底层英文错误归一化为客户端用户可理解的中文提示。
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
    if (normalized.contains('not assigned to any network') ||
        normalized.contains('no active network attachment') ||
        normalized.contains('no active network')) {
      return '当前设备未加入任何网络，请联系管理员配置网络。';
    }
    if (normalized.contains('device unavailable')) {
      return '设备不可用，请联系管理员重新启用。';
    }
    if (normalized.contains('session expired')) {
      return '登录状态已失效，请重新登录。';
    }
    if (normalized.contains('local service')) {
      return '本地服务未连接，请稍后重试或重启客户端。';
    }
    return error;
  }

  /// 构建已登录状态卡片。
  Widget _buildSignedInHeader({
    required ClientViewState state,
  }) {
    final displayedState = _pendingNetworkTarget != null
        ? state.copyWith(
            networkEnabled: _pendingNetworkTarget,
            syncing: true,
            switchEnabled: false,
          )
        : state;
    return SignedInStatusPanel(
      desktop: _isDesktopLike,
      userLabel: _userLabel(displayedState),
      currentIp: _ipText(displayedState),
      state: displayedState,
      onToggle: _toggleNetwork,
    );
  }

  /// 构建 Android VPN 授权提示；非 Android 或无需提示时隐藏。
  Widget _buildAndroidAuthorizationPanel() {
    return ValueListenableBuilder<AndroidNetworkAuthorizationState>(
      valueListenable: widget.bridge.androidNetworkAuthorization,
      builder: (context, authorization, _) {
        if (!authorization.visible) {
          return const SizedBox.shrink();
        }
        return Padding(
          padding: const EdgeInsets.only(top: 8),
          child: AndroidAuthorizationPanel(
            authorization: authorization,
            onRefresh: widget.bridge.prepareAndroidNetworkAuthorization,
          ),
        );
      },
    );
  }

  /// 归一化用户显示名。
  String _userLabel(ClientViewState state) {
    final user = state.userLabel?.trim();
    return user == null || user.isEmpty ? '-' : user;
  }

  /// 归一化当前虚拟 IP 展示文案。
  String _ipText(ClientViewState state) {
    if (!state.networkEnabled) {
      return '未启用';
    }
    if (state.syncing) {
      return '启用中';
    }
    final virtualIp = state.virtualIp?.trim();
    if (!state.signedIn || virtualIp == null || virtualIp.isEmpty) {
      return '待分配';
    }
    return virtualIp;
  }
}

/// 桌面和移动端共用的紧凑服务器设置弹窗。
class _ServerSettingsDialog extends StatelessWidget {
  const _ServerSettingsDialog({required this.controller});

  final TextEditingController controller;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Dialog(
      backgroundColor: Colors.white,
      clipBehavior: Clip.antiAlias,
      insetPadding: const EdgeInsets.symmetric(horizontal: 24, vertical: 20),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
      child: ConstrainedBox(
        key: const Key('server-settings-dialog'),
        constraints: const BoxConstraints(maxWidth: 340),
        child: Padding(
          padding: const EdgeInsets.fromLTRB(16, 14, 16, 18),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Row(
                children: [
                  Icon(
                    Icons.dns_outlined,
                    size: 18,
                    color: theme.colorScheme.primary,
                  ),
                  const SizedBox(width: 7),
                  Text(
                    '服务器设置',
                    style: theme.textTheme.bodyLarge?.copyWith(
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 12),
              SizedBox(
                height: 38,
                child: TextField(
                  key: const Key('server-base-url'),
                  controller: controller,
                  autofocus: true,
                  keyboardType: TextInputType.url,
                  textInputAction: TextInputAction.done,
                  style: theme.textTheme.bodySmall,
                  decoration: const InputDecoration(
                    hintText: 'http://example.com:28080',
                    contentPadding: EdgeInsets.symmetric(
                      horizontal: 10,
                      vertical: 9,
                    ),
                  ),
                  onSubmitted: (_) => Navigator.of(context).pop(true),
                ),
              ),
              const SizedBox(height: 12),
              Row(
                children: [
                  Expanded(
                    child: SizedBox(
                      height: 34,
                      child: OutlinedButton(
                        key: const Key('server-cancel'),
                        onPressed: () => Navigator.of(context).pop(false),
                        style: OutlinedButton.styleFrom(
                          minimumSize: Size.zero,
                          tapTargetSize: MaterialTapTargetSize.shrinkWrap,
                          textStyle: theme.textTheme.labelMedium,
                        ),
                        child: const Text('取消'),
                      ),
                    ),
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: SizedBox(
                      height: 34,
                      child: FilledButton(
                        key: const Key('server-confirm'),
                        onPressed: () => Navigator.of(context).pop(true),
                        style: FilledButton.styleFrom(
                          minimumSize: Size.zero,
                          tapTargetSize: MaterialTapTargetSize.shrinkWrap,
                          textStyle: theme.textTheme.labelMedium,
                        ),
                        child: const Text('确认'),
                      ),
                    ),
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}
