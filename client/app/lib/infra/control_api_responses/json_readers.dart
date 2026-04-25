Map<String, dynamic> readMap(Map<String, dynamic> json, String key) {
  final value = json[key];
  if (value is Map<String, dynamic>) {
    return value;
  }
  if (value is Map) {
    return value.map(
      (mapKey, mapValue) => MapEntry(mapKey.toString(), mapValue),
    );
  }
  throw FormatException('Expected object for "$key", got ${value.runtimeType}');
}

Map<String, dynamic>? readNullableMap(Map<String, dynamic> json, String key) {
  final value = json[key];
  if (value == null) {
    return null;
  }
  return readMap(json, key);
}

List<Map<String, dynamic>> readMapList(Map<String, dynamic> json, String key) {
  final value = json[key];
  if (value == null) {
    return const [];
  }
  if (value is! List) {
    throw FormatException('Expected list for "$key", got ${value.runtimeType}');
  }
  return value.map(asMap).toList(growable: false);
}

Map<String, dynamic> asMap(dynamic value) {
  if (value is Map<String, dynamic>) {
    return value;
  }
  if (value is Map) {
    return value.map(
      (mapKey, mapValue) => MapEntry(mapKey.toString(), mapValue),
    );
  }
  throw FormatException('Expected object item, got ${value.runtimeType}');
}

String readString(Map<String, dynamic> json, String key) {
  final value = json[key];
  if (value is String) {
    return value;
  }
  throw FormatException('Expected string for "$key", got ${value.runtimeType}');
}

String? readNullableString(Map<String, dynamic> json, String key) {
  final value = json[key];
  if (value == null) {
    return null;
  }
  if (value is String) {
    return value;
  }
  throw FormatException('Expected string for "$key", got ${value.runtimeType}');
}

int readInt(Map<String, dynamic> json, String key) {
  final value = json[key];
  if (value is int) {
    return value;
  }
  throw FormatException('Expected int for "$key", got ${value.runtimeType}');
}

int? readNullableInt(Map<String, dynamic> json, String key) {
  final value = json[key];
  if (value == null) {
    return null;
  }
  if (value is int) {
    return value;
  }
  throw FormatException('Expected int for "$key", got ${value.runtimeType}');
}

bool readBool(Map<String, dynamic> json, String key) {
  final value = json[key];
  if (value is bool) {
    return value;
  }
  throw FormatException('Expected bool for "$key", got ${value.runtimeType}');
}

bool? readNullableBool(Map<String, dynamic> json, String key) {
  final value = json[key];
  if (value == null) {
    return null;
  }
  if (value is bool) {
    return value;
  }
  throw FormatException('Expected bool for "$key", got ${value.runtimeType}');
}

List<String> readStringList(Map<String, dynamic> json, String key) {
  final value = json[key];
  if (value == null) {
    return const [];
  }
  if (value is! List) {
    throw FormatException('Expected list for "$key", got ${value.runtimeType}');
  }
  return value.map((item) => item.toString()).toList(growable: false);
}
