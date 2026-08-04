import 'dart:async';

import 'package:flutter/material.dart';

import '../../bridge/android_network_authorization.dart';
import '../../bridge/client_commands.dart';
import '../../bridge/client_core_bridge.dart';
import '../../bridge/client_ui_diagnostics.dart';
import '../../bridge/client_view_state.dart';
import 'widgets/android_authorization_panel.dart';
import 'widgets/network_status_panel.dart';
import 'widgets/device_activation_form.dart';
import 'widgets/server_settings_summary.dart';
import 'widgets/inactive_status.dart';

/// SLAN 桌面/移动客户端首页。
///
/// 页面只负责呈现状态和收集输入；设备激活和网络开关等
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

  /// 设备授权 Key 输入框。
  final TextEditingController _authorizationKeyController =
      TextEditingController();

  /// 最近一次已记录的状态快照，用于避免重复写诊断日志。
  String? _lastDiagnosticsSnapshot;

  /// 最近一次已弹窗展示的网络错误，避免同一错误反复弹窗。
  String? _lastShownError;

  /// 当前移动端控制面 API 地址。
  String? _serverBaseUrl;

  /// 上一次设备激活态，用于首次激活时触发平台授权准备。
  bool _lastActivated = false;

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
    _authorizationKeyController.dispose();
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
                  _isDesktopLike ? 10 : 12,
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
                      if (state.activated) ...[
                        _buildActivatedHeader(state: state),
                        _buildAndroidAuthorizationPanel(),
                      ] else ...[
                        if (!_isDesktopLike) ...[
                          const InactiveStatus(),
                          const SizedBox(height: 14),
                          ServerSettingsSummary(
                            serverBaseUrl: _serverBaseUrl,
                            syncing: state.syncing,
                            onSettings: _showServerSettings,
                          ),
                          const SizedBox(height: 10),
                        ],
                        DeviceActivationForm(
                          keyController: _authorizationKeyController,
                          syncing: state.syncing,
                          onSettings:
                              _isDesktopLike ? _showServerSettings : null,
                          onSubmit: _activateDevice,
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

  bool get _isMobile =>
      Theme.of(context).platform == TargetPlatform.iOS ||
      Theme.of(context).platform == TargetPlatform.android;

  bool get _isDesktopLike => !_isMobile;

  /// 读取当前服务端地址并同步到设置输入框。
  Future<void> _loadServerBaseUrl() async {
    final value = await widget.bridge.serverBaseUrl();
    if (!mounted) {
      return;
    }
    setState(() {
      _serverBaseUrl = value;
    });
  }

  /// 弹出服务器 API 地址设置，并仅在后台确认保存后关闭。
  Future<void> _showServerSettings() async {
    final initialValue = _serverBaseUrl ?? await widget.bridge.serverBaseUrl();
    if (!mounted) {
      return;
    }
    final normalized = await showDialog<String>(
      context: context,
      builder: (context) => _ServerApiSettingsDialog(
        initialValue: initialValue,
        onConfirm: (value) async {
          await widget.bridge.updateServerBaseUrl(value);
          return widget.bridge.serverBaseUrl();
        },
      ),
    );
    if (normalized == null || !mounted) {
      return;
    }
    setState(() {
      _serverBaseUrl = normalized;
    });
    ScaffoldMessenger.of(
      context,
    ).showSnackBar(SnackBar(content: Text('服务器 API 已设置为 $normalized')));
  }

  Future<void> _activateDevice() async {
    final key = _authorizationKeyController.text.trim();
    if (key.isEmpty) {
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(const SnackBar(content: Text('请输入授权 Key')));
      return;
    }
    try {
      await widget.bridge.activateDevice(key);
      _authorizationKeyController.clear();
    } catch (error) {
      if (mounted)
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('设备激活失败：$error')));
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
      state.activated,
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
    if (state.activated && !_lastActivated) {
      unawaited(widget.bridge.prepareAndroidNetworkAuthorization());
    }
    _lastActivated = state.activated;
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
              constraints: const BoxConstraints(maxWidth: 320, maxHeight: 180),
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
      return '当前设备未加入任何网络，请在 Opt 中配置网络。';
    }
    if (normalized.contains('device unavailable')) {
      return '设备不可用，请联系管理员重新启用。';
    }
    if (normalized.contains('session expired')) {
      return '设备授权已失效，请使用新的授权 Key 激活。';
    }
    if (normalized.contains('local service')) {
      return '本地服务未连接，请稍后重试或重启客户端。';
    }
    return error;
  }

  /// 构建已登录状态卡片。
  Widget _buildActivatedHeader({required ClientViewState state}) {
    final displayedState = _pendingNetworkTarget != null
        ? state.copyWith(
            networkEnabled: _pendingNetworkTarget,
            syncing: true,
            switchEnabled: false,
          )
        : state;
    return ActivatedStatusPanel(
      desktop: _isDesktopLike,
      deviceLabel: _deviceLabel(displayedState),
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

  String _deviceLabel(ClientViewState state) {
    final deviceId = state.deviceId?.trim();
    return deviceId == null || deviceId.isEmpty ? '-' : deviceId;
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
    if (!state.activated || virtualIp == null || virtualIp.isEmpty) {
      return '待分配';
    }
    return virtualIp;
  }
}

class _ServerApiSettingsDialog extends StatefulWidget {
  const _ServerApiSettingsDialog({
    required this.initialValue,
    required this.onConfirm,
  });

  final String initialValue;
  final Future<String> Function(String value) onConfirm;

  @override
  State<_ServerApiSettingsDialog> createState() =>
      _ServerApiSettingsDialogState();
}

class _ServerApiSettingsDialogState extends State<_ServerApiSettingsDialog> {
  late final TextEditingController _controller;
  String? _errorText;
  bool _saving = false;

  @override
  void initState() {
    super.initState();
    _controller = TextEditingController(text: widget.initialValue);
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  Future<void> _confirm() async {
    final value = _controller.text.trim();
    if (value.isEmpty) {
      setState(() => _errorText = '请输入服务器 API 地址');
      return;
    }
    setState(() {
      _saving = true;
      _errorText = null;
    });
    try {
      final normalized = await widget.onConfirm(value);
      if (mounted) {
        Navigator.of(context).pop(normalized);
      }
    } catch (error) {
      if (mounted) {
        setState(() {
          _saving = false;
          _errorText = '保存失败：$error';
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Dialog(
      insetPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 6),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(8)),
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 420),
        child: Padding(
          padding: const EdgeInsets.all(12),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Row(
                children: [
                  Icon(Icons.dns_rounded, color: theme.colorScheme.primary),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      '服务器 API 设置',
                      style: theme.textTheme.titleMedium?.copyWith(
                        fontWeight: FontWeight.w800,
                      ),
                    ),
                  ),
                  IconButton(
                    tooltip: '关闭',
                    visualDensity: VisualDensity.compact,
                    onPressed:
                        _saving ? null : () => Navigator.of(context).pop(),
                    icon: const Icon(Icons.close_rounded),
                  ),
                ],
              ),
              const SizedBox(height: 6),
              TextField(
                key: const Key('server-base-url'),
                controller: _controller,
                enabled: !_saving,
                autofocus: true,
                keyboardType: TextInputType.url,
                textInputAction: TextInputAction.done,
                decoration: const InputDecoration(
                  labelText: '服务器 API 地址',
                  hintText: 'http://example.com:28080',
                  prefixIcon: Icon(Icons.link_rounded),
                ),
                onChanged: (_) {
                  if (_errorText != null) {
                    setState(() => _errorText = null);
                  }
                },
                onSubmitted: (_) {
                  if (!_saving) {
                    _confirm();
                  }
                },
              ),
              SizedBox(
                height: 20,
                child: Align(
                  alignment: Alignment.centerLeft,
                  child: Text(
                    _errorText ?? '用于连接客户端控制服务',
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: theme.textTheme.labelSmall?.copyWith(
                      color: _errorText == null
                          ? theme.colorScheme.onSurfaceVariant
                          : theme.colorScheme.error,
                    ),
                  ),
                ),
              ),
              const SizedBox(height: 4),
              Row(
                mainAxisAlignment: MainAxisAlignment.end,
                children: [
                  OutlinedButton(
                    key: const Key('server-cancel'),
                    onPressed:
                        _saving ? null : () => Navigator.of(context).pop(),
                    child: const Text('取消'),
                  ),
                  const SizedBox(width: 8),
                  FilledButton(
                    key: const Key('server-save'),
                    onPressed: _saving ? null : _confirm,
                    child: _saving
                        ? const SizedBox.square(
                            dimension: 16,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : const Text('确定'),
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
