library slan_app.application.local_dns_service;

import 'dart:async';
import 'dart:io';
import 'dart:typed_data';

import '../infra/app_core/models/network_models.dart';

class LocalDnsService {
  LocalDnsService._();

  static final LocalDnsService instance = LocalDnsService._();

  RawDatagramSocket? _socket;
  StreamSubscription<RawSocketEvent>? _subscription;
  Map<String, String> _records = const {};

  bool get isRunning => _socket != null;
  int get port => _socket?.port ?? 0;

  Future<void> configureFromNetwork(NetworkModel network) async {
    if (!network.dns.enabled) {
      await stop();
      return;
    }
    final records = buildRecords(network);
    if (records.isEmpty) {
      await stop();
      return;
    }
    await start(records: records);
  }

  Future<void> start({
    required Map<String, String> records,
    InternetAddress? bindAddress,
    int bindPort = 53,
  }) async {
    _records = Map.unmodifiable(records.map(
      (key, value) => MapEntry(normalizeName(key), value.trim()),
    ));
    if (_socket != null) {
      return;
    }
    final socket = await RawDatagramSocket.bind(
      bindAddress ?? InternetAddress.loopbackIPv4,
      bindPort,
      reuseAddress: true,
    );
    _socket = socket;
    _subscription = socket.listen((event) {
      if (event != RawSocketEvent.read) {
        return;
      }
      Datagram? datagram;
      while ((datagram = socket.receive()) != null) {
        final response = buildResponse(datagram!.data, _records);
        if (response != null) {
          socket.send(response, datagram!.address, datagram!.port);
        }
      }
    });
  }

  Future<void> stop() async {
    await _subscription?.cancel();
    _subscription = null;
    _socket?.close();
    _socket = null;
    _records = const {};
  }

  static Map<String, String> buildRecords(NetworkModel network) {
    if (!network.dns.enabled) {
      return const {};
    }
    final records = <String, String>{};
    for (final wildcard in network.dns.wildcards) {
      final record = normalizeWildcardRecord(wildcard);
      if (record != null) {
        records[record.key] = record.value;
      }
    }
    return records;
  }

  static Uint8List? buildResponse(
    Uint8List request,
    Map<String, String> records,
  ) {
    if (request.length < 12) {
      return null;
    }
    final id = request.sublist(0, 2);
    final qdCount = _readU16(request, 4);
    if (qdCount == 0) {
      return null;
    }
    final question = _readQuestion(request, 12);
    if (question == null) {
      return null;
    }
    final ip = resolveName(question.name, records);
    final address = ip == null ? null : InternetAddress.tryParse(ip);
    final canAnswer = question.type == 1 &&
        address != null &&
        address.type == InternetAddressType.IPv4;
    final builder = BytesBuilder();
    builder.add(id);
    builder.add(_u16(canAnswer ? 0x8180 : 0x8183));
    builder.add(_u16(1));
    builder.add(_u16(canAnswer ? 1 : 0));
    builder.add(_u16(0));
    builder.add(_u16(0));
    builder.add(request.sublist(12, question.endOffset));
    if (canAnswer) {
      builder.add(_u16(0xc00c));
      builder.add(_u16(1));
      builder.add(_u16(1));
      builder.add(_u32(30));
      builder.add(_u16(4));
      builder.add(address!.rawAddress);
    }
    return builder.toBytes();
  }

  static String normalizeName(String value) {
    return value.trim().toLowerCase().replaceAll(RegExp(r'\.$'), '');
  }

  static String? resolveName(String name, Map<String, String> records) {
    final normalized = normalizeName(name);
    final exact = records[normalized];
    if (exact != null) {
      return exact;
    }
    String? matchedIp;
    var matchedLabels = -1;
    for (final entry in records.entries) {
      final pattern = normalizeName(entry.key);
      if (!pattern.startsWith('*.') && !pattern.startsWith('*.*.')) {
        continue;
      }
      if (!_wildcardMatches(pattern, normalized)) {
        continue;
      }
      final labelCount = pattern.split('.').length;
      if (labelCount > matchedLabels) {
        matchedLabels = labelCount;
        matchedIp = entry.value;
      }
    }
    return matchedIp;
  }

  static MapEntry<String, String>? normalizeWildcardRecord(String value) {
    final parts = value.split('=');
    if (parts.length != 2) {
      return null;
    }
    var host = normalizeName(parts[0]);
    if (host.startsWith('*') && !host.startsWith('*.')) {
      host = '*. ${host.substring(1)}'.replaceAll(' ', '');
    }
    final ip = parts[1].trim();
    if (!_isAllowedWildcardHost(host)) {
      return null;
    }
    final address = InternetAddress.tryParse(ip);
    if (address == null || address.type != InternetAddressType.IPv4) {
      return null;
    }
    return MapEntry(host, ip);
  }

  static bool _wildcardMatches(String pattern, String name) {
    final patternLabels = pattern.split('.');
    final nameLabels = name.split('.');
    if (patternLabels.length != nameLabels.length) {
      return false;
    }
    for (var index = 0; index < patternLabels.length; index++) {
      final patternLabel = patternLabels[index];
      if (patternLabel == '*') {
        continue;
      }
      if (patternLabel != nameLabels[index]) {
        return false;
      }
    }
    return true;
  }

  static bool _isAllowedWildcardHost(String host) {
    final labels = host.split('.');
    var wildcardCount = 0;
    while (wildcardCount < labels.length && labels[wildcardCount] == '*') {
      wildcardCount++;
    }
    if (wildcardCount == 0 || labels.length - wildcardCount < 2) {
      return false;
    }
    for (final label in labels.skip(wildcardCount)) {
      if (label == '*' || label.isEmpty) {
        return false;
      }
    }
    return true;
  }

  static _DnsQuestion? _readQuestion(Uint8List data, int offset) {
    final labels = <String>[];
    var cursor = offset;
    while (cursor < data.length) {
      final length = data[cursor];
      cursor++;
      if (length == 0) {
        break;
      }
      if ((length & 0xc0) != 0 || cursor + length > data.length) {
        return null;
      }
      labels.add(String.fromCharCodes(data.sublist(cursor, cursor + length)));
      cursor += length;
    }
    if (cursor + 4 > data.length || labels.isEmpty) {
      return null;
    }
    return _DnsQuestion(
      name: labels.join('.'),
      type: _readU16(data, cursor),
      endOffset: cursor + 4,
    );
  }

  static int _readU16(Uint8List data, int offset) =>
      (data[offset] << 8) | data[offset + 1];

  static List<int> _u16(int value) => [(value >> 8) & 0xff, value & 0xff];

  static List<int> _u32(int value) => [
        (value >> 24) & 0xff,
        (value >> 16) & 0xff,
        (value >> 8) & 0xff,
        value & 0xff,
      ];
}

class _DnsQuestion {
  const _DnsQuestion({
    required this.name,
    required this.type,
    required this.endOffset,
  });

  final String name;
  final int type;
  final int endOffset;
}
