import 'package:client_core_plugin/client_core_plugin.dart';

/// Android VPN 授权和网络配置的 UI 状态。
///
/// Android 的 VpnService 需要用户授权，授权、VPN 启停、relay 配置和错误
/// 都通过原生插件事件异步回来。这个状态对象把事件规约成页面可以直接
/// 渲染的形态。
class AndroidNetworkAuthorizationState {
  const AndroidNetworkAuthorizationState({
    required this.checking,
    this.permissionState,
    this.consentRequest,
    this.networkConfig,
    this.error,
  });

  /// 是否正在检查 VPN 权限或刷新网络配置。
  final bool checking;

  /// 原生层返回的权限状态。
  final String? permissionState;

  /// 需要用户确认时，原生层返回的 consent 请求信息。
  final AndroidVpnConsentRequest? consentRequest;

  /// 已登录且授权后可应用到 Android VpnService 的网络配置。
  final AndroidVpnSessionConfig? networkConfig;

  /// 最近一次授权或配置错误。
  final String? error;

  /// 页面初始状态：不显示授权面板，不处于检查中。
  static const initial = AndroidNetworkAuthorizationState(checking: false);

  /// 是否需要展示“授权”按钮。
  bool get needsUserConsent =>
      permissionState == AndroidVpnPermissionState.needsUserConsent;

  /// VPN 权限是否已授予。
  bool get granted => permissionState == AndroidVpnPermissionState.granted;

  /// 授权面板是否需要显示。
  ///
  /// 只要存在检查、权限、配置或错误信息，就显示面板，避免用户不知道
  /// Android 网络为什么没有启动。
  bool get visible =>
      checking ||
      permissionState != null ||
      consentRequest != null ||
      networkConfig != null ||
      error != null;

  /// 根据原生插件事件生成新的授权状态。
  ///
  /// 这里不直接执行副作用，只更新 UI 状态；真正的 VPN 启停由 bridge
  /// 根据用户命令和网络配置调用原生插件。
  AndroidNetworkAuthorizationState applyEvent(AndroidNetworkEvent event) {
    switch (event.eventType) {
      case AndroidNetworkEventType.permissionRequired:
        return copyWith(
          checking: false,
          permissionState: AndroidVpnPermissionState.needsUserConsent,
          error: event.message,
        );
      case AndroidNetworkEventType.permissionGranted:
        return copyWith(
          checking: false,
          permissionState: AndroidVpnPermissionState.granted,
          error: null,
          clearConsentRequest: true,
        );
      case AndroidNetworkEventType.vpnStarted:
      case AndroidNetworkEventType.vpnStopped:
      case AndroidNetworkEventType.connectivityChanged:
      case AndroidNetworkEventType.relayChanged:
        return copyWith(
          checking: false,
          error: null,
        );
      case AndroidNetworkEventType.vpnRevoked:
        return copyWith(
          checking: false,
          permissionState: AndroidVpnPermissionState.needsUserConsent,
          error: event.message ?? 'Android VPN permission was revoked',
          clearNetworkConfig: true,
        );
      case AndroidNetworkEventType.error:
        return copyWith(
          checking: false,
          error: event.message ?? 'Android network error',
        );
    }
    return this;
  }

  /// 复制状态并支持显式清空 consent/config。
  ///
  /// Dart 的可空字段无法区分“保持原值”和“设置为 null”，所以通过
  /// clearConsentRequest、clearNetworkConfig 表达清空语义。
  AndroidNetworkAuthorizationState copyWith({
    bool? checking,
    String? permissionState,
    AndroidVpnConsentRequest? consentRequest,
    AndroidVpnSessionConfig? networkConfig,
    String? error,
    bool clearConsentRequest = false,
    bool clearNetworkConfig = false,
  }) {
    return AndroidNetworkAuthorizationState(
      checking: checking ?? this.checking,
      permissionState: permissionState ?? this.permissionState,
      consentRequest:
          clearConsentRequest ? null : consentRequest ?? this.consentRequest,
      networkConfig:
          clearNetworkConfig ? null : networkConfig ?? this.networkConfig,
      error: error,
    );
  }
}
