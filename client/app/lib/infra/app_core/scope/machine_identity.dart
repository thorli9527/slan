library slan_app.infra.app_core.scope.machine_identity;

import 'dart:io';

typedef MachineIdentityCommandRunner = Future<ProcessResult> Function(
  String executable,
  List<String> arguments,
);

typedef MachineIdentityFileReader = Future<String?> Function(String path);

class MachineIdentity {
  const MachineIdentity._();

  static Future<String> resolve({
    String? persistedMachineId,
    String? operatingSystem,
    String? localHostname,
    DateTime Function()? now,
    MachineIdentityCommandRunner? commandRunner,
    MachineIdentityFileReader? fileReader,
  }) async {
    final os = (operatingSystem ?? Platform.operatingSystem).toLowerCase();
    final raw = await _readRawMachineId(
      os,
      commandRunner ?? Process.run,
      fileReader ?? _readFile,
    );
    if (raw != null && raw.trim().isNotEmpty) {
      return _stableId(os, raw);
    }

    final persisted = persistedMachineId?.trim();
    if (persisted != null && persisted.isNotEmpty) {
      return persisted;
    }

    final host = (localHostname ?? Platform.localHostname)
        .trim()
        .replaceAll(RegExp(r'[^a-zA-Z0-9-]'), '-');
    final timestamp =
        (now ?? DateTime.now)().microsecondsSinceEpoch.toString();
    return 'client-$host-$timestamp';
  }

  static Future<String?> _readRawMachineId(
    String os,
    MachineIdentityCommandRunner commandRunner,
    MachineIdentityFileReader fileReader,
  ) async {
    return switch (os) {
      'windows' => _readWindowsMachineGuid(commandRunner),
      'macos' => _readMacPlatformUuid(commandRunner),
      'linux' => _readLinuxMachineId(fileReader),
      _ => null,
    };
  }

  static Future<String?> _readWindowsMachineGuid(
    MachineIdentityCommandRunner commandRunner,
  ) async {
    try {
      final result = await commandRunner('reg', [
        'query',
        r'HKLM\SOFTWARE\Microsoft\Cryptography',
        '/v',
        'MachineGuid',
      ]);
      if (result.exitCode != 0) {
        return null;
      }
      final output = '${result.stdout}\n${result.stderr}';
      final match =
          RegExp(r'MachineGuid\s+REG_\w+\s+([^\s]+)').firstMatch(output);
      return match?.group(1);
    } catch (_) {
      return null;
    }
  }

  static Future<String?> _readMacPlatformUuid(
    MachineIdentityCommandRunner commandRunner,
  ) async {
    try {
      final result = await commandRunner('ioreg', [
        '-rd1',
        '-c',
        'IOPlatformExpertDevice',
      ]);
      if (result.exitCode != 0) {
        return null;
      }
      final output = '${result.stdout}\n${result.stderr}';
      final match = RegExp(r'"IOPlatformUUID"\s+=\s+"([^"]+)"')
          .firstMatch(output);
      return match?.group(1);
    } catch (_) {
      return null;
    }
  }

  static Future<String?> _readLinuxMachineId(
    MachineIdentityFileReader fileReader,
  ) async {
    for (final path in const [
      '/etc/machine-id',
      '/var/lib/dbus/machine-id',
    ]) {
      final value = (await fileReader(path))?.trim();
      if (value != null && value.isNotEmpty) {
        return value;
      }
    }
    return null;
  }

  static Future<String?> _readFile(String path) async {
    try {
      return await File(path).readAsString();
    } catch (_) {
      return null;
    }
  }

  static String _stableId(String os, String raw) {
    final normalized = raw.trim().toLowerCase();
    return 'client-$os-${_fnv1a64Hex('slan:$os:$normalized')}';
  }

  static String _fnv1a64Hex(String value) {
    const mask = 0xffffffffffffffff;
    var hash = 0xcbf29ce484222325;
    for (final unit in value.codeUnits) {
      hash ^= unit;
      hash = (hash * 0x100000001b3) & mask;
    }
    return hash.toRadixString(16).padLeft(16, '0');
  }
}
