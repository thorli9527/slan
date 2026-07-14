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
class _HomePageState extends State<HomePage> {
  /// 移动端密码登录账号输入框。
  final TextEditingController _emailController = TextEditingController();

  /// 移动端密码登录密码输入框。
  final TextEditingController _passwordController = TextEditingController();

  /// 移动端控制面 API 地址输入框。
  final TextEditingController _serverBaseUrlController =
      TextEditingController();

  /// 最近一次已记录的状态快照，用于避免重复写诊断日志。
  String? _lastDiagnosticsSnapshot;

  /// 最近一次已弹窗展示的网络错误，避免同一错误反复弹窗。
  String? _lastShownError;

  /// 当前移动端控制面 API 地址。
  String? _serverBaseUrl;

  /// 上一次登录态，用于从未登录变已登录时触发平台授权准备。
  bool _lastSignedIn = false;

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
                padding: EdgeInsets.fromLTRB(
                  _isDesktopLike ? 28 : 20,
                  _isDesktopLike ? 22 : 12,
                  _isDesktopLike ? 28 : 20,
                  24,
                ),
                child: ConstrainedBox(
                  constraints: BoxConstraints(
                    maxWidth: _isDesktopLike ? 860 : 560,
                  ),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      _buildShellHeader(state),
                      const SizedBox(height: 16),
                      _buildStateBanner(state),
                      if (state.notice?.trim().isNotEmpty == true ||
                          state.error?.trim().isNotEmpty == true)
                        const SizedBox(height: 12),
                      if (state.signedIn) ...[
                        _buildSignedInHeader(state: state),
                        _buildAndroidAuthorizationPanel(),
                        const SizedBox(height: 14),
                        SignedInActions(
                          desktop: _isDesktopLike,
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
                        SignedOutStatus(desktop: _isDesktopLike),
                        const SizedBox(height: 14),
                        if (_usesPasswordLogin)
                          PasswordLoginForm(
                            emailController: _emailController,
                            passwordController: _passwordController,
                            serverBaseUrl: _serverBaseUrl,
                            syncing: state.syncing,
                            onSettings: _showServerSettings,
                            onSubmit: _loginWithPassword,
                          )
                        else
                          SizedBox(
                            height: 46,
                            child: FilledButton.icon(
                              key: const Key('desktop-browser-login'),
                              onPressed: state.syncing
                                  ? null
                                  : () => widget.bridge.dispatch(
                                        const ClientCommand(
                                          ClientCommandType.openClientLogin,
                                        ),
                                      ),
                              icon: const Icon(Icons.login_rounded, size: 18),
                              label: const Text('打开浏览器登录'),
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

  /// 移动端使用内置账号密码登录；桌面端走浏览器登录同步。
  bool get _usesPasswordLogin =>
      Theme.of(context).platform == TargetPlatform.iOS ||
      Theme.of(context).platform == TargetPlatform.android;

  /// 桌面端登录后显示打开 Web Console 的入口。
  bool get _showWebConsoleAction => !_usesPasswordLogin;

  bool get _isDesktopLike => !_usesPasswordLogin;

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

  /// 弹出移动端服务器地址设置。
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

  /// 移动端账号密码登录。
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

  /// 处理用户点击网络开关。
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

  /// 构建已登录状态卡片。
  Widget _buildSignedInHeader({
    required ClientViewState state,
  }) {
    return SignedInStatusPanel(
      desktop: _isDesktopLike,
      userLabel: _userLabel(state),
      currentIp: _ipText(state),
      state: state,
      onToggle: _toggleNetwork,
    );
  }

  Widget _buildShellHeader(ClientViewState state) {
    final theme = Theme.of(context);
    return Container(
      padding: EdgeInsets.fromLTRB(
        _isDesktopLike ? 22 : 18,
        _isDesktopLike ? 20 : 16,
        _isDesktopLike ? 22 : 18,
        _isDesktopLike ? 20 : 16,
      ),
      decoration: BoxDecoration(
        gradient: const LinearGradient(
          colors: [Color(0xfffff4ec), Color(0xfff4ede7)],
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
        ),
        borderRadius: BorderRadius.circular(26),
        border: Border.all(color: const Color(0xffeadccf)),
      ),
      child: Row(
        children: [
          Container(
            width: _isDesktopLike ? 56 : 48,
            height: _isDesktopLike ? 56 : 48,
            decoration: BoxDecoration(
              color: const Color(0xffb85c2f),
              borderRadius: BorderRadius.circular(18),
            ),
            child: const Icon(
              Icons.hub_rounded,
              color: Colors.white,
              size: 26,
            ),
          ),
          const SizedBox(width: 14),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'SLAN Client',
                  style: theme.textTheme.headlineSmall?.copyWith(
                    fontWeight: FontWeight.w900,
                    color: const Color(0xff4c2a19),
                  ),
                ),
                const SizedBox(height: 4),
                Text(
                  _headerSubtitle(state),
                  style: theme.textTheme.bodyMedium?.copyWith(
                    color: const Color(0xff7a604d),
                    height: 1.35,
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(width: 12),
          _HeaderStateChip(label: _headerStatusLabel(state)),
        ],
      ),
    );
  }

  Widget _buildStateBanner(ClientViewState state) {
    final notice = state.notice?.trim();
    if (notice != null &&
        notice.isNotEmpty &&
        notice != 'macosStatusItemReady') {
      return _InlineBanner(
        icon: Icons.info_outline_rounded,
        tone: _BannerTone.info,
        text: notice,
      );
    }
    return const SizedBox.shrink();
  }

  String _headerSubtitle(ClientViewState state) {
    if (_isDesktopLike) {
      return state.signedIn
          ? '设备已接入本地控制服务，可直接切换虚拟网络并打开 Web Console。'
          : '使用浏览器完成登录，客户端会自动同步设备与网络状态。';
    }
    return state.signedIn ? '当前设备已登录，可直接启用虚拟网络。' : '输入账号密码后即可同步设备与网络配置。';
  }

  String _headerStatusLabel(ClientViewState state) {
    if (state.error?.trim().isNotEmpty == true) {
      return '异常';
    }
    if (state.syncing) {
      return '同步中';
    }
    if (state.networkEnabled) {
      return '已连接';
    }
    if (state.signedIn) {
      return '已登录';
    }
    return '未登录';
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

class _HeaderStateChip extends StatelessWidget {
  const _HeaderStateChip({required this.label});

  final String label;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
      decoration: BoxDecoration(
        color: Colors.white.withValues(alpha: 0.72),
        borderRadius: BorderRadius.circular(999),
        border: Border.all(color: const Color(0xffe5d6ca)),
      ),
      child: Text(
        label,
        style: Theme.of(context).textTheme.labelLarge?.copyWith(
              fontWeight: FontWeight.w800,
              color: const Color(0xff5c3924),
            ),
      ),
    );
  }
}

enum _BannerTone { info }

class _InlineBanner extends StatelessWidget {
  const _InlineBanner({
    required this.icon,
    required this.tone,
    required this.text,
  });

  final IconData icon;
  final _BannerTone tone;
  final String text;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
      decoration: BoxDecoration(
        color: const Color(0xffeef6ff),
        borderRadius: BorderRadius.circular(16),
        border: Border.all(
          color: const Color(0xffcfe1f8),
        ),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(
            icon,
            size: 18,
            color: const Color(0xff2f6fb8),
          ),
          const SizedBox(width: 10),
          Expanded(
            child: Text(
              text,
              style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                    color: const Color(0xff224e81),
                    height: 1.35,
                  ),
            ),
          ),
        ],
      ),
    );
  }
}
