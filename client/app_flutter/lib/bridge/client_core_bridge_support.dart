import 'dart:convert';

import 'package:client_core_plugin/client_core_plugin.dart';
import 'client_view_state.dart';

abstract final class ClientBusinessEventType {
  static const sessionChanged = 'session.changed';
  static const networkSwitchFinished = 'network.switch.finished';
  static const networkRuntimeChanged = 'network.runtime.changed';
  static const networkSwitchFailed = 'network.switch.failed';
  static const controlSyncChanged = 'control.sync.changed';
  static const stateChanged = 'state.changed';
}

String? businessEventType(Map<String, Object?> event) {
  return event['businessType'] as String?;
}

String? stringField(Map<String, Object?>? map, String key) {
  final value = map?[key];
  return value is String ? value : null;
}

bool boolField(Map<String, Object?>? map, String key) {
  return map?[key] == true;
}

bool businessEventRequiresStateQuery(String? type,
    {required bool networkToggleInFlight}) {
  if (type == ClientBusinessEventType.networkRuntimeChanged) {
    return networkToggleInFlight;
  }
  return type == ClientBusinessEventType.networkSwitchFinished ||
      type == ClientBusinessEventType.networkSwitchFailed;
}

bool businessEventSettlesNetworkToggle(String? type) {
  return type == ClientBusinessEventType.networkSwitchFinished ||
      type == ClientBusinessEventType.networkSwitchFailed;
}

Map<String, Object?> businessEventReceivedLogFields(
  String? type, {
  Map<String, Object?>? businessDataMap,
  Map<String, Object?>? snapshotMap,
}) {
  return {
    'businessType': type,
    'businessDataMessageType': businessDataMap?['messageType'],
    'businessDataEventType': businessDataMap?['eventType'],
    'businessDataNetworkId': businessDataMap?['networkId'],
    'snapshotActivated': snapshotMap?['activated'],
    'snapshotNetworkEnabled': snapshotMap?['networkEnabled'],
  };
}

Map<String, Object?> businessEventStateQueriedLogFields(
  String? type,
  ClientViewState state,
) {
  return {
    'businessType': type,
    'queriedActivated': state.activated,
    'queriedDeviceId': state.deviceId,
    'queriedNetworkEnabled': state.networkEnabled,
    'queriedVirtualIp': state.virtualIp,
    'queriedNotice': state.notice,
  };
}

String controlSyncEventKey(
  Map<String, Object?> event,
  Map<String, Object?> businessData,
) {
  final revision = event['revision'];
  final eventId = stringField(businessData, 'eventId');
  final configVersion = businessData['configVersion'];
  final networkId = stringField(businessData, 'networkId') ?? '';
  final eventType = stringField(businessData, 'eventType') ?? '';
  final messageType = stringField(businessData, 'messageType') ?? '';
  return [
    if (revision != null) '$revision' else '',
    eventId ?? '',
    if (configVersion != null) '$configVersion' else '',
    networkId,
    eventType,
    messageType,
  ].join('|');
}

ClientViewState mergeBusinessState(
  ClientViewState current,
  ClientViewState incoming, {
  Map<String, Object?>? event,
  String? businessType,
}) {
  final businessData = event?['businessData'];
  final businessDataMap =
      businessData is Map ? businessData.cast<String, Object?>() : null;
  final isControlSync =
      businessType == ClientBusinessEventType.controlSyncChanged;
  final controlSyncMessageType = isControlSync
      ? businessDataMap == null
          ? null
          : stringField(businessDataMap, 'messageType')
      : incoming.lastControlSyncMessageType;
  final controlSyncReconfigureRequired = isControlSync
      ? businessDataMap != null &&
          boolField(businessDataMap, 'reconfigureRequired')
      : incoming.lastControlSyncReconfigureRequired;
  return current.copyWith(
    activated: incoming.activated,
    deviceId: incoming.deviceId,
    networkEnabled: incoming.networkEnabled,
    virtualIp: incoming.virtualIp,
    syncing: incoming.syncing,
    syncReason: incoming.syncReason,
    switchEnabled: incoming.switchEnabled,
    notice: incoming.notice,
    error: incoming.error,
    errorSource: incoming.errorSource,
    lastControlSyncMessageType: controlSyncMessageType,
    lastControlSyncReconfigureRequired: controlSyncReconfigureRequired,
    trafficTxBytes: incoming.trafficTxBytes,
    trafficRxBytes: incoming.trafficRxBytes,
    trafficTxBytesPerMinute: incoming.trafficTxBytesPerMinute,
    trafficRxBytesPerMinute: incoming.trafficRxBytesPerMinute,
    trafficUpdatedAtMs: incoming.trafficUpdatedAtMs,
    clearSyncReason: incoming.syncReason == null,
    clearVirtualIp: !incoming.activated,
  );
}

ClientViewState? reduceBusinessEvent(
  ClientViewState current,
  Map<String, Object?> event, {
  ClientViewState? queriedState,
  required ClientViewState? dataState,
  required ClientViewState? snapshotState,
  bool networkToggleInFlight = false,
}) {
  final type = businessEventType(event);
  // The event payload describes the state when the event was published, while
  // snapshotState is captured when Rust answers this watch request. A delayed
  // event must not roll the UI back after a newer runtime transition.
  final incoming = queriedState ?? snapshotState ?? dataState;
  if (incoming == null) {
    return null;
  }

  switch (type) {
    case ClientBusinessEventType.sessionChanged:
      return current.copyWith(
        activated: incoming.activated,
        deviceId: incoming.deviceId,
        networkEnabled: incoming.networkEnabled,
        virtualIp: incoming.virtualIp,
        syncing: false,
        clearSyncReason: true,
        switchEnabled: incoming.switchEnabled,
        notice: incoming.notice,
        error: incoming.error,
        clearVirtualIp: !incoming.activated,
      );
    case ClientBusinessEventType.networkSwitchFinished:
      return incoming.copyWith(
        syncing: false,
        clearSyncReason: true,
        switchEnabled: true,
        clearVirtualIp: !incoming.activated,
      );
    case ClientBusinessEventType.networkRuntimeChanged:
      if (!networkToggleInFlight) {
        return incoming.copyWith(
          syncing: false,
          clearSyncReason: true,
          switchEnabled: true,
          clearVirtualIp: !incoming.activated,
        );
      }
      final merged = mergeBusinessState(
        current,
        incoming,
        event: event,
        businessType: type,
      );
      // Runtime snapshots can arrive while the platform is still applying the
      // adapter and routes. Keep the optimistic target and the UI lock until a
      // terminal switch event (or the command result) settles the operation.
      return merged.copyWith(
        networkEnabled: current.networkEnabled,
        virtualIp: current.virtualIp,
        syncing: true,
        syncReason: current.syncReason,
        switchEnabled: false,
        notice: current.notice,
        error: current.error,
        errorSource: current.errorSource,
        clearVirtualIp: !current.activated,
      );
    case ClientBusinessEventType.networkSwitchFailed:
      final error = incoming.error ??
          dataState?.error ??
          snapshotState?.error ??
          'network switch failed';
      return current.copyWith(
        networkEnabled: incoming.networkEnabled,
        virtualIp: incoming.virtualIp,
        syncing: false,
        clearSyncReason: true,
        switchEnabled: true,
        error: error,
        errorSource: ClientErrorSource.networkSwitch,
        clearVirtualIp: !incoming.activated,
      );
    case ClientBusinessEventType.controlSyncChanged:
    case ClientBusinessEventType.stateChanged:
    default:
      return mergeBusinessState(
        current,
        incoming,
        event: event,
        businessType: type,
      );
  }
}

Map<String, Object?> iosPacketTunnelDiagnosticsFields(
  Map<String, Object?> stats,
) {
  return {
    'relaySessionCount': stats['relaySessionCount'],
    'relayAttachedSessionCount': stats['relayAttachedSessionCount'],
    'relayAttachFailures': stats['relayAttachFailures'],
    'lastRelayAttachError': stats['lastRelayAttachError'],
    'packetsRead': stats['packetsRead'],
    'bytesRead': stats['bytesRead'],
    'bytesWritten': stats['bytesWritten'],
    'routedPackets': stats['routedPackets'],
    'unroutablePackets': stats['unroutablePackets'],
    'nonIpv4Packets': stats['nonIpv4Packets'],
    'relayFramesSent': stats['relayFramesSent'],
    'relayFramesReceived': stats['relayFramesReceived'],
    'relayPacketsWritten': stats['relayPacketsWritten'],
    'relayDetachSent': stats['relayDetachSent'],
    'relayNoPeerPackets': stats['relayNoPeerPackets'],
    'directUdpAttachedPeerCount': stats['directUdpAttachedPeerCount'],
    'directUdpReadyPeerCount': stats['directUdpReadyPeerCount'],
    'directUdpProbesSent': stats['directUdpProbesSent'],
    'directUdpProbesReceived': stats['directUdpProbesReceived'],
    'directUdpPongsSent': stats['directUdpPongsSent'],
    'directUdpPongsReceived': stats['directUdpPongsReceived'],
    'directUdpFramesSent': stats['directUdpFramesSent'],
    'directUdpFramesReceived': stats['directUdpFramesReceived'],
    'lastDestination': stats['lastDestination'],
    'lastRoute': stats['lastRoute'],
    'lastRoutedAtMs': stats['lastRoutedAtMs'],
    'updatedAtMs': stats['updatedAtMs'],
  };
}

Map<String, Object?> androidRuntimeDiagnosticsFields(
  Map<String, Object?> state,
) {
  return {
    'adapterPresent': state['adapterPresent'],
    'networkEnabled': state['networkEnabled'],
    'virtualIp': state['virtualIp'],
    'mtu': state['mtu'],
    'relayAddress': state['relayAddress'],
    'relaySessionCount': state['relaySessionCount'],
    'requestedRelaySessionCount': state['requestedRelaySessionCount'],
    'attachedRelaySessionCount': state['attachedRelaySessionCount'],
    'relayAttachFailures': state['relayAttachFailures'],
    'lastRelayAttachError': state['lastRelayAttachError'],
    'packetsRead': state['packetsRead'],
    'bytesRead': state['bytesRead'],
    'bytesWritten': state['bytesWritten'],
    'packetsTooLarge': state['packetsTooLarge'],
    'relayFramesSent': state['relayFramesSent'],
    'relayFramesReceived': state['relayFramesReceived'],
    'relayDetachSent': state['relayDetachSent'],
    'relayNoPeerPackets': state['relayNoPeerPackets'],
    'relayWriteFailures': state['relayWriteFailures'],
    'tunWriteFailures': state['tunWriteFailures'],
    'embeddedServicePendingLimit': state['embeddedServicePendingLimit'],
    'embeddedServicePendingCount': state['embeddedServicePendingCount'],
    'embeddedServiceQueueDepth': state['embeddedServiceQueueDepth'],
    'embeddedServiceActiveCount': state['embeddedServiceActiveCount'],
    'embeddedServiceCompletedTotal': state['embeddedServiceCompletedTotal'],
    'embeddedServiceRejectedTotal': state['embeddedServiceRejectedTotal'],
    'embeddedWatchPendingLimit': state['embeddedWatchPendingLimit'],
    'embeddedWatchPendingCount': state['embeddedWatchPendingCount'],
    'embeddedWatchQueueDepth': state['embeddedWatchQueueDepth'],
    'embeddedWatchActiveCount': state['embeddedWatchActiveCount'],
    'embeddedWatchCompletedTotal': state['embeddedWatchCompletedTotal'],
    'embeddedWatchRejectedTotal': state['embeddedWatchRejectedTotal'],
  };
}

Map<String, Object?> relayDebugSummary(Object? relayDebug) {
  if (relayDebug is! Map) {
    return {'present': relayDebug != null};
  }
  final requested = relayDebug['requestedRelaySessionCount'] ??
      relayDebug['requestedSessionCount'] ??
      relayDebug['relaySessionCount'];
  final attached = relayDebug['attachedRelaySessionCount'] ??
      relayDebug['attachedSessionCount'];
  return {
    'present': true,
    if (relayDebug.containsKey('enabled')) 'enabled': relayDebug['enabled'],
    if (relayDebug.containsKey('relayAddress'))
      'relayAddress': relayDebug['relayAddress'],
    if (requested != null) 'requestedRelaySessionCount': requested,
    if (attached != null) 'attachedRelaySessionCount': attached,
    if (relayDebug.containsKey('lastRelayAttachError'))
      'lastRelayAttachError': relayDebug['lastRelayAttachError'],
  };
}

String androidVpnConfigFingerprint(AndroidVpnSessionConfig config) {
  return jsonEncode(config.toJson());
}
