import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:flutter/foundation.dart';

import '../app_core/models/identity_models.dart';

class DeviceMqttService {
  DeviceMqttService._();

  static final DeviceMqttService instance = DeviceMqttService._();

  Socket? _socket;
  StreamSubscription<Uint8List>? _socketSubscription;
  Timer? _pingTimer;
  String? _connectedClientId;

  Future<bool> connectForDevice(DeviceModel? device) async {
    final credential = device?.mqtt;
    if (credential == null || credential.clientId.trim().isEmpty) {
      return false;
    }
    if (_connectedClientId == credential.clientId && _socket != null) {
      return true;
    }
    await close();
    Socket? socket;
    try {
      final uri = Uri.parse(credential.brokerUrl);
      socket = await Socket.connect(
        uri.host,
        uri.hasPort ? uri.port : 1883,
        timeout: const Duration(seconds: 3),
      );
      final connAck = Completer<void>();
      final connAckBuffer = BytesBuilder(copy: false);
      _socketSubscription = socket.listen(
        (packet) {
          if (!connAck.isCompleted) {
            connAckBuffer.add(packet);
            try {
              final result = _tryReadConnAck(connAckBuffer.toBytes());
              if (result == _ConnAckResult.accepted) {
                connAck.complete();
              } else if (result == _ConnAckResult.rejected) {
                connAck.completeError(StateError('MQTT broker rejected connection'));
              }
            } catch (error, stackTrace) {
              connAck.completeError(error, stackTrace);
            }
          }
        },
        onDone: () {
          if (!connAck.isCompleted) {
            connAck.completeError(StateError('MQTT socket closed before CONNACK'));
          }
          _markClosed();
        },
        onError: (Object error, StackTrace stackTrace) {
          if (!connAck.isCompleted) {
            connAck.completeError(error, stackTrace);
          }
          _markClosed();
        },
        cancelOnError: true,
      );
      socket.add(_connectPacket(credential));
      await socket.flush();
      await connAck.future.timeout(const Duration(seconds: 3));
      final connectedSocket = socket;
      _socket = socket;
      _connectedClientId = credential.clientId;
      final topic = _deviceTopicFilter(credential);
      if (topic.isNotEmpty) {
        socket.add(_subscribePacket(topic));
        await socket.flush();
      }
      _pingTimer = Timer.periodic(const Duration(seconds: 20), (_) {
        connectedSocket.add([0xc0, 0x00]);
      });
      debugPrint('[device-mqtt] connected clientId=${credential.clientId}');
      return true;
    } catch (error) {
      await _socketSubscription?.cancel();
      _socketSubscription = null;
      try {
        await socket?.close();
      } catch (_) {
        // Closing is best effort.
      }
      _markClosed();
      debugPrint('[device-mqtt] connect failed: $error');
      return false;
    }
  }

  Future<void> close() async {
    _pingTimer?.cancel();
    _pingTimer = null;
    final socket = _socket;
    _socket = null;
    _connectedClientId = null;
    await _socketSubscription?.cancel();
    _socketSubscription = null;
    if (socket == null) {
      return;
    }
    try {
      socket.add([0xe0, 0x00]);
      await socket.close();
    } catch (_) {
      // Closing is best effort.
    }
  }

  void _markClosed() {
    _pingTimer?.cancel();
    _pingTimer = null;
    _socket = null;
    _connectedClientId = null;
    _socketSubscription = null;
  }

  Future<void> publishNetworkState({
    required DeviceModel? device,
    required String networkId,
    required bool controlReachable,
    required bool networkOnline,
    required bool tunnelUp,
    required bool lastProbeOk,
    String? virtualIp,
    int? reportedAt,
  }) async {
    final credential = device?.mqtt;
    final socket = _socket;
    if (credential == null || socket == null) {
      return;
    }
    final topic = '${credential.topicPrefix}/networks/$networkId/state';
    final payload = jsonEncode({
      'deviceId': device?.deviceId,
      'networkId': networkId,
      'controlReachable': controlReachable,
      'networkOnline': networkOnline,
      'tunnelUp': tunnelUp,
      'lastProbeOk': lastProbeOk,
      if (virtualIp != null && virtualIp.isNotEmpty) 'virtualIp': virtualIp,
      'reportedAt':
          reportedAt ?? DateTime.now().millisecondsSinceEpoch ~/ 1000,
    });
    socket.add(_publishPacket(topic, payload));
    await socket.flush();
  }
}

enum _ConnAckResult { pending, accepted, rejected }

_ConnAckResult _tryReadConnAck(Uint8List packet) {
  if (packet.length < 2) {
    return _ConnAckResult.pending;
  }
  if (packet[0] != 0x20 || packet[1] != 0x02) {
    throw const FormatException('invalid MQTT CONNACK packet');
  }
  if (packet.length < 4) {
    return _ConnAckResult.pending;
  }
  if (packet[3] != 0x00) {
    return _ConnAckResult.rejected;
  }
  return _ConnAckResult.accepted;
}

String _deviceTopicFilter(MqttCredentialModel credential) {
  final prefix = credential.topicPrefix.trim();
  return prefix.isEmpty ? '' : '$prefix/#';
}

Uint8List _connectPacket(MqttCredentialModel credential) {
  final variable = BytesBuilder();
  _writeString(variable, 'MQTT');
  variable.add([0x04, 0xc2, 0x00, 0x1e]);
  _writeString(variable, credential.clientId);
  _writeString(variable, credential.username);
  _writeString(variable, credential.password);
  return _packet(0x10, variable.toBytes());
}

Uint8List _subscribePacket(String topic) {
  final variable = BytesBuilder();
  variable.add([0x00, 0x01]);
  _writeString(variable, topic);
  variable.add([0x00]);
  return _packet(0x82, variable.toBytes());
}

Uint8List _publishPacket(String topic, String payload) {
  final variable = BytesBuilder();
  _writeString(variable, topic);
  variable.add(utf8.encode(payload));
  return _packet(0x30, variable.toBytes());
}

Uint8List _packet(int header, Uint8List body) {
  final builder = BytesBuilder();
  builder.add([header]);
  builder.add(_encodeRemainingLength(body.length));
  builder.add(body);
  return builder.toBytes();
}

void _writeString(BytesBuilder builder, String value) {
  final bytes = utf8.encode(value);
  builder.add([(bytes.length >> 8) & 0xff, bytes.length & 0xff]);
  builder.add(bytes);
}

List<int> _encodeRemainingLength(int length) {
  final encoded = <int>[];
  do {
    var digit = length % 128;
    length ~/= 128;
    if (length > 0) {
      digit |= 128;
    }
    encoded.add(digit);
  } while (length > 0);
  return encoded;
}
