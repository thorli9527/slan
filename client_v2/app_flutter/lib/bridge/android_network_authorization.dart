import 'package:client_core_plugin/client_core_plugin.dart';

class AndroidNetworkAuthorizationState {
  const AndroidNetworkAuthorizationState({
    required this.checking,
    this.permissionState,
    this.consentRequest,
    this.networkConfig,
    this.error,
  });

  final bool checking;
  final String? permissionState;
  final AndroidVpnConsentRequest? consentRequest;
  final AndroidVpnSessionConfig? networkConfig;
  final String? error;

  static const initial = AndroidNetworkAuthorizationState(checking: false);

  bool get needsUserConsent =>
      permissionState == AndroidVpnPermissionState.needsUserConsent;

  bool get granted => permissionState == AndroidVpnPermissionState.granted;

  bool get visible =>
      checking ||
      permissionState != null ||
      consentRequest != null ||
      networkConfig != null ||
      error != null;

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
