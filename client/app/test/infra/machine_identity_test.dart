import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:slan_app/infra/app_core/scope/machine_identity.dart';

void main() {
  test('uses hashed Windows MachineGuid before persisted fallback', () async {
    final machineId = await MachineIdentity.resolve(
      operatingSystem: 'windows',
      persistedMachineId: 'client-old-random',
      commandRunner: (executable, arguments) async => ProcessResult(
        1,
        0,
        r'''
HKEY_LOCAL_MACHINE\SOFTWARE\Microsoft\Cryptography
    MachineGuid    REG_SZ    01234567-89AB-CDEF-0123-456789ABCDEF
''',
        '',
      ),
    );

    expect(machineId, startsWith('client-windows-'));
    expect(machineId, isNot('client-old-random'));
    expect(machineId, isNot(contains('01234567')));
  });

  test('uses Linux machine-id when available', () async {
    final machineId = await MachineIdentity.resolve(
      operatingSystem: 'linux',
      fileReader: (path) async =>
          path == '/etc/machine-id' ? 'abcdef123456\n' : null,
    );

    expect(machineId, startsWith('client-linux-'));
    expect(machineId, isNot(contains('abcdef123456')));
  });

  test('falls back to persisted id when no system identity is available',
      () async {
    final machineId = await MachineIdentity.resolve(
      operatingSystem: 'unknown',
      persistedMachineId: 'client-persisted',
    );

    expect(machineId, 'client-persisted');
  });
}
