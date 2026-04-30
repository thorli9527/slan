/// 数据面探测结果模型。
class DataPlaneProbeModel {
  const DataPlaneProbeModel({
    required this.probeId,
    required this.sampledAtMs,
    required this.activePath,
    required this.bytesSent,
    required this.replyObserved,
    this.replyBytesReceived,
    this.replySampledAtMs,
    this.replyRttMs,
    this.tunnelPeerVirtualIp,
    this.observedRttMs,
    this.packetLossPpm,
    this.pathScore,
    this.derpClusterId,
    this.derpNodeId,
  });

  final String probeId;
  final int sampledAtMs;
  final DataPlanePathModel activePath;
  final int bytesSent;
  final bool replyObserved;
  final int? replyBytesReceived;
  final int? replySampledAtMs;
  final int? replyRttMs;
  final String? tunnelPeerVirtualIp;
  final int? observedRttMs;
  final int? packetLossPpm;
  final int? pathScore;
  final String? derpClusterId;
  final String? derpNodeId;
}

class DataPlanePathModel {
  const DataPlanePathModel({
    required this.kind,
    this.details = const {},
  });

  final String kind;
  final Map<String, dynamic> details;

  bool get isRelay {
    final normalized = kind.toLowerCase();
    return normalized.contains('relay') || normalized.contains('derp');
  }

  bool get isDirect {
    final normalized = kind.toLowerCase();
    return normalized.contains('p2p') ||
        normalized.contains('direct') ||
        normalized.contains('reflexive');
  }
}

class PlatformDoctorModel {
  const PlatformDoctorModel({
    required this.platform,
    required this.tunnelBackend,
    this.checks = const [],
  });

  final PlatformInfoModel platform;
  final TunnelBackendDiagnosticsModel tunnelBackend;
  final List<PlatformCheckModel> checks;
}

class AppCoreHelperStatusModel {
  const AppCoreHelperStatusModel({
    required this.source,
    required this.helperReachable,
    this.configuredControlBaseUrl,
    this.persistedControlBaseUrl,
    this.stateFile,
    required this.sessionPresent,
    required this.refreshTokenPresent,
    this.deviceId,
    this.nodeId,
    this.currentNetworkId,
    required this.bootstrapPresent,
    required this.networkMapPresent,
    required this.tunnelRuntimePresent,
    this.tunnelPeerVirtualIp,
    required this.tunnelBackendRunning,
    this.tunnelLastError,
    this.tunnelLastAppliedAtMs,
    this.tunnelLastStartedAtMs,
    this.checkedAtMs,
  });

  final String source;
  final bool helperReachable;
  final String? configuredControlBaseUrl;
  final String? persistedControlBaseUrl;
  final String? stateFile;
  final bool sessionPresent;
  final bool refreshTokenPresent;
  final String? deviceId;
  final String? nodeId;
  final String? currentNetworkId;
  final bool bootstrapPresent;
  final bool networkMapPresent;
  final bool tunnelRuntimePresent;
  final String? tunnelPeerVirtualIp;
  final bool tunnelBackendRunning;
  final String? tunnelLastError;
  final int? tunnelLastAppliedAtMs;
  final int? tunnelLastStartedAtMs;
  final int? checkedAtMs;

  bool get isReady =>
      helperReachable && sessionPresent && currentNetworkId != null;
}

class PlatformInstallPlanModel {
  const PlatformInstallPlanModel({
    required this.platform,
    this.packages = const [],
    this.supportedDriverModes = const [],
    this.warnings = const [],
  });

  final PlatformInfoModel platform;
  final List<String> packages;
  final List<String> supportedDriverModes;
  final List<String> warnings;
}

class PlatformInfoModel {
  const PlatformInfoModel({
    required this.os,
    this.distroId,
    this.versionId,
    this.idLike = const [],
    this.family,
    this.kernelRelease,
    this.packageManager,
  });

  final String os;
  final String? distroId;
  final String? versionId;
  final List<String> idLike;
  final String? family;
  final String? kernelRelease;
  final String? packageManager;
}

class TunnelBackendDiagnosticsModel {
  const TunnelBackendDiagnosticsModel({
    required this.name,
    this.executionMode,
    this.executionBackend,
    this.interfaceName,
    required this.isUp,
    required this.plannedPeerCount,
    required this.recentCommandCount,
  });

  final String name;
  final String? executionMode;
  final String? executionBackend;
  final String? interfaceName;
  final bool isUp;
  final int plannedPeerCount;
  final int recentCommandCount;
}

class PlatformCheckModel {
  const PlatformCheckModel({
    required this.name,
    required this.status,
    required this.detail,
  });

  final String name;
  final String status;
  final String detail;

  bool get isOk => status.toLowerCase() == 'ok';
  bool get isWarning => status.toLowerCase() == 'warn';
  bool get isFailure => status.toLowerCase() == 'fail';
}

enum ProbeFailureKind {
  timeout,
  transport,
  relayAuth,
  relaySession,
  relayProtocol,
  unsupported,
  unknown,
}

enum DataPlaneFailureKind {
  timeout,
  transport,
  relayAuth,
  relaySession,
  relayProtocol,
  unsupported,
  unknown,
}

extension DataPlaneFailureKindPresentation on DataPlaneFailureKind {
  String get label => switch (this) {
        DataPlaneFailureKind.timeout => 'timeout',
        DataPlaneFailureKind.transport => 'transport',
        DataPlaneFailureKind.relayAuth => 'relay auth',
        DataPlaneFailureKind.relaySession => 'relay session',
        DataPlaneFailureKind.relayProtocol => 'relay protocol',
        DataPlaneFailureKind.unsupported => 'unsupported',
        DataPlaneFailureKind.unknown => 'unknown',
      };

  String? get hint => switch (this) {
        DataPlaneFailureKind.timeout => 'check reachability and retry',
        DataPlaneFailureKind.transport =>
          'inspect network path or socket state',
        DataPlaneFailureKind.relayAuth =>
          'refresh bootstrap or relay ticket before retrying',
        DataPlaneFailureKind.relaySession =>
          'reconnect the session before retrying',
        DataPlaneFailureKind.relayProtocol => 'inspect relay logs and payloads',
        DataPlaneFailureKind.unsupported => 'switch to a supported path',
        DataPlaneFailureKind.unknown => null,
      };
}

class DataPlaneFailureDetails {
  const DataPlaneFailureDetails({
    required this.kind,
    required this.code,
    required this.message,
  });

  final DataPlaneFailureKind kind;
  final String? code;
  final String message;

  String get label => kind.label;
  String? get hint => kind.hint;
  String get summary => hint == null ? message : '$message; $hint';
}

class ProbeFailure {
  const ProbeFailure({
    required this.kind,
    required this.code,
    required this.message,
  });

  factory ProbeFailure.fromDataPlane(DataPlaneFailureDetails details) {
    return ProbeFailure(
      kind: switch (details.kind) {
        DataPlaneFailureKind.timeout => ProbeFailureKind.timeout,
        DataPlaneFailureKind.transport => ProbeFailureKind.transport,
        DataPlaneFailureKind.relayAuth => ProbeFailureKind.relayAuth,
        DataPlaneFailureKind.relaySession => ProbeFailureKind.relaySession,
        DataPlaneFailureKind.relayProtocol => ProbeFailureKind.relayProtocol,
        DataPlaneFailureKind.unsupported => ProbeFailureKind.unsupported,
        DataPlaneFailureKind.unknown => ProbeFailureKind.unknown,
      },
      code: details.code,
      message: details.message,
    );
  }

  final ProbeFailureKind kind;
  final String? code;
  final String message;

  String get label => switch (kind) {
        ProbeFailureKind.timeout => 'timeout',
        ProbeFailureKind.transport => 'transport',
        ProbeFailureKind.relayAuth => 'relay auth',
        ProbeFailureKind.relaySession => 'relay session',
        ProbeFailureKind.relayProtocol => 'relay protocol',
        ProbeFailureKind.unsupported => 'unsupported',
        ProbeFailureKind.unknown => 'unknown',
      };

  String? get hint => switch (kind) {
        ProbeFailureKind.timeout => 'check reachability and retry',
        ProbeFailureKind.transport => 'inspect network path or socket state',
        ProbeFailureKind.relayAuth =>
          'refresh bootstrap or relay ticket before retrying',
        ProbeFailureKind.relaySession =>
          'reconnect the session before retrying',
        ProbeFailureKind.relayProtocol => 'inspect relay logs and payloads',
        ProbeFailureKind.unsupported => 'switch to a supported path',
        ProbeFailureKind.unknown => null,
      };

  String get summary => hint == null ? message : '$message; $hint';
}

class ProbeException implements Exception {
  const ProbeException(this.failure);

  final ProbeFailure failure;

  @override
  String toString() => failure.message;
}

enum SendFailureKind {
  timeout,
  transport,
  relayAuth,
  relaySession,
  relayProtocol,
  unsupported,
  unknown,
}

class SendFailure {
  const SendFailure({
    required this.kind,
    required this.code,
    required this.message,
  });

  factory SendFailure.fromDataPlane(DataPlaneFailureDetails details) {
    return SendFailure(
      kind: switch (details.kind) {
        DataPlaneFailureKind.timeout => SendFailureKind.timeout,
        DataPlaneFailureKind.transport => SendFailureKind.transport,
        DataPlaneFailureKind.relayAuth => SendFailureKind.relayAuth,
        DataPlaneFailureKind.relaySession => SendFailureKind.relaySession,
        DataPlaneFailureKind.relayProtocol => SendFailureKind.relayProtocol,
        DataPlaneFailureKind.unsupported => SendFailureKind.unsupported,
        DataPlaneFailureKind.unknown => SendFailureKind.unknown,
      },
      code: details.code,
      message: details.message,
    );
  }

  final SendFailureKind kind;
  final String? code;
  final String message;

  String get label => switch (kind) {
        SendFailureKind.timeout => 'timeout',
        SendFailureKind.transport => 'transport',
        SendFailureKind.relayAuth => 'relay auth',
        SendFailureKind.relaySession => 'relay session',
        SendFailureKind.relayProtocol => 'relay protocol',
        SendFailureKind.unsupported => 'unsupported',
        SendFailureKind.unknown => 'unknown',
      };

  String? get hint => switch (kind) {
        SendFailureKind.timeout => 'check reachability and retry',
        SendFailureKind.transport => 'inspect network path or socket state',
        SendFailureKind.relayAuth =>
          'refresh bootstrap or relay ticket before retrying',
        SendFailureKind.relaySession => 'reconnect the session before retrying',
        SendFailureKind.relayProtocol => 'inspect relay logs and payloads',
        SendFailureKind.unsupported => 'switch to a supported path',
        SendFailureKind.unknown => null,
      };

  String get summary => hint == null ? message : '$message; $hint';
}

class SendException implements Exception {
  const SendException(this.failure);

  final SendFailure failure;

  @override
  String toString() => failure.message;
}
