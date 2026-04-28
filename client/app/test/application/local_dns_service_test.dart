import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:slan_app/application/local_dns_service.dart';
import 'package:slan_app/infra/app_core/models/network_models.dart';

void main() {
  test('buildRecords returns empty when server DNS config is disabled', () {
    final records = LocalDnsService.buildRecords(const NetworkModel(
      networkId: 'net-1',
      name: 'My Network',
      cidr: '10.0.0.0/24',
      members: [
        NetworkMemberModel(
          deviceId: 'dev-1',
          role: 'owner',
          virtualIp: '10.0.0.2',
          remark: 'Laptop',
        ),
      ],
    ));

    expect(records, isEmpty);
  });

  test('buildRecords ignores legacy search domains without wildcard records',
      () {
    final records = LocalDnsService.buildRecords(const NetworkModel(
      networkId: 'net-1',
      name: 'My Network',
      cidr: '10.0.0.0/24',
      dns: DNSConfigModel(searchDomains: ['slan']),
      members: [
        NetworkMemberModel(
          deviceId: 'dev-1',
          role: 'owner',
          virtualIp: '10.0.0.2',
          remark: 'Living Room PC',
        ),
      ],
    ));

    expect(records, isEmpty);
  });

  test('buildResponse answers A record without forwarding public DNS', () {
    final response = LocalDnsService.buildResponse(
      _query('Laptop.slan'),
      {'laptop.slan': '10.0.0.2'},
    );

    expect(response, isNotNull);
    expect(response![3], 0x80);
    expect(response[7], 1);
    expect(response.sublist(response.length - 4), [10, 0, 0, 2]);
  });

  test('wildcard records support only left-side wildcard labels', () {
    final records = LocalDnsService.buildRecords(const NetworkModel(
      networkId: 'net-1',
      name: 'My Network',
      cidr: '10.0.0.0/24',
      dns: DNSConfigModel(wildcards: [
        '*.xx.com=10.0.0.2',
        '*.*.xx.net=10.0.0.3',
        '*.com=10.0.0.4',
        'api.*.xx.com=10.0.0.5',
      ]),
    ));

    expect(LocalDnsService.resolveName('api.xx.com', records), '10.0.0.2');
    expect(LocalDnsService.resolveName('a.b.xx.net', records), '10.0.0.3');
    expect(LocalDnsService.resolveName('xx.com', records), isNull);
    expect(LocalDnsService.resolveName('anything.com', records), isNull);
    expect(LocalDnsService.resolveName('api.foo.xx.com', records), isNull);
  });
}

Uint8List _query(String name) {
  final bytes = <int>[
    0x12,
    0x34,
    0x01,
    0x00,
    0x00,
    0x01,
    0x00,
    0x00,
    0x00,
    0x00,
    0x00,
    0x00,
  ];
  for (final label in name.split('.')) {
    bytes.add(label.length);
    bytes.addAll(label.codeUnits);
  }
  bytes.addAll([0, 0, 1, 0, 1]);
  return Uint8List.fromList(bytes);
}
