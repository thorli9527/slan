import 'dart:convert';

import 'package:client_core_plugin/client_core_plugin.dart';
import 'package:flutter/foundation.dart';

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

bool businessEventPayloadCarriesClientMessage(
  String? type,
  Map<String, Object?>? payload,
) {
  if (payload == null) {
    return false;
  }
  if (type != ClientBusinessEventType.controlSyncChanged &&
      type != ClientBusinessEventType.stateChanged) {
    return false;
  }
  final messageId = stringField(payload, 'lastClientMessageId');
  final fromDeviceId = stringField(payload, 'lastClientMessageFromDeviceId');
  final body = stringField(payload, 'lastClientMessageBody');
  return (messageId != null && messageId.isNotEmpty) ||
      (fromDeviceId != null && fromDeviceId.isNotEmpty) ||
      (body != null && body.isNotEmpty);
}

Map<String, Object?> businessEventReceivedLogFields(
  String? type, {
  Map<String, Object?>? businessDataMap,
  Map<String, Object?>? snapshotMap,
}) {
  return {
    'businessType': type,
    'businessDataMessageType': businessDataMap?['messageType'],
    'businessDataLastClientMessageId': businessDataMap?['lastClientMessageId'],
    'businessDataLastClientMessageFromDeviceId':
        businessDataMap?['lastClientMessageFromDeviceId'],
    'businessDataLastClientMessageBodyLength':
        (businessDataMap?['lastClientMessageBody'] as String?)?.length,
    'snapshotLastClientMessageId': snapshotMap?['lastClientMessageId'],
    'snapshotLastClientMessageFromDeviceId':
        snapshotMap?['lastClientMessageFromDeviceId'],
    'snapshotLastClientMessageBodyLength':
        (snapshotMap?['lastClientMessageBody'] as String?)?.length,
  };
}

Map<String, Object?> businessEventPayloadClientMessagePreferredLogFields(
  String? type, {
  Map<String, Object?>? businessDataMap,
  Map<String, Object?>? snapshotMap,
}) {
  return {
    'businessType': type,
    'businessDataLastClientMessageId': businessDataMap?['lastClientMessageId'],
    'snapshotLastClientMessageId': snapshotMap?['lastClientMessageId'],
  };
}

Map<String, Object?> businessEventStateQueriedLogFields(
  String? type,
  ClientViewState state,
) {
  return {
    'businessType': type,
    'queriedLastClientMessageId': state.lastClientMessageId,
    'queriedLastClientMessageFromDeviceId': state.lastClientMessageFromDeviceId,
    'queriedLastClientMessageBodyLength': state.lastClientMessageBody?.length,
    'queriedNotice': state.notice,
  };
}

bool shouldLogEmptyClientMessageQuery(String? type, ClientViewState state) {
  if (type != ClientBusinessEventType.controlSyncChanged &&
      type != ClientBusinessEventType.stateChanged) {
    return false;
  }
  return state.lastClientMessageId == null &&
      state.lastClientMessageFromDeviceId == null &&
      state.lastClientMessageBody == null;
}

Map<String, Object?> businessEventEmptyClientMessageQueryLogFields(
  String? type,
  ClientViewState state,
) {
  return {
    'businessType': type,
    'queriedSignedIn': state.signedIn,
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
    signedIn: incoming.signedIn,
    userLabel: incoming.userLabel,
    deviceId: incoming.deviceId,
    networkEnabled: incoming.networkEnabled,
    virtualIp: incoming.virtualIp,
    syncing: incoming.syncing,
    syncReason: incoming.syncReason,
    switchEnabled: incoming.switchEnabled,
    notice: incoming.notice,
    error: incoming.error,
    errorSource: incoming.errorSource,
    lastClientMessageId: incoming.lastClientMessageId,
    lastClientMessageFromDeviceId: incoming.lastClientMessageFromDeviceId,
    lastClientMessageBody: incoming.lastClientMessageBody,
    lastControlSyncMessageType: controlSyncMessageType,
    lastControlSyncReconfigureRequired: controlSyncReconfigureRequired,
    trafficTxBytes: incoming.trafficTxBytes,
    trafficRxBytes: incoming.trafficRxBytes,
    trafficTxBytesPerMinute: incoming.trafficTxBytesPerMinute,
    trafficRxBytesPerMinute: incoming.trafficRxBytesPerMinute,
    trafficUpdatedAtMs: incoming.trafficUpdatedAtMs,
    clearSyncReason: incoming.syncReason == null,
    clearVirtualIp: !incoming.networkEnabled,
  );
}

ClientViewState? reduceBusinessEvent(
  ClientViewState current,
  Map<String, Object?> event, {
  ClientViewState? queriedState,
  required ClientViewState? dataState,
  required ClientViewState? snapshotState,
}) {
  final type = businessEventType(event);
  final incoming = queriedState ?? dataState ?? snapshotState;
  if (incoming == null) {
    return null;
  }

  switch (type) {
    case ClientBusinessEventType.sessionChanged:
      return current.copyWith(
        signedIn: incoming.signedIn,
        userLabel: incoming.userLabel,
        deviceId: incoming.deviceId,
        networkEnabled: incoming.networkEnabled,
        virtualIp: incoming.virtualIp,
        syncing: false,
        clearSyncReason: true,
        switchEnabled: incoming.switchEnabled,
        notice: incoming.notice,
        error: incoming.error,
        clearVirtualIp: !incoming.networkEnabled,
      );
    case ClientBusinessEventType.networkSwitchFinished:
    case ClientBusinessEventType.networkRuntimeChanged:
      return incoming.copyWith(
        syncing: false,
        clearSyncReason: true,
        switchEnabled: true,
        clearVirtualIp: !incoming.networkEnabled,
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
        clearVirtualIp: !incoming.networkEnabled,
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

@visibleForTesting
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

@visibleForTesting
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
