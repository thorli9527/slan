import 'dart:convert';

import 'package:client_core_plugin/client_core_plugin.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';

import 'client_commands.dart';
import 'control_transport_status.dart';
import 'client_view_state.dart';

abstract interface class ClientCoreBridge {
  ValueListenable<ClientViewState> get state;

  Future<void> start();
  Future<void> dispatch(ClientCommand command);
  Future<ControlTransportStatus?> controlTransportStatus();
}

class MethodChannelClientCoreBridge implements ClientCoreBridge {
  MethodChannelClientCoreBridge()
      : _plugin = ClientCorePlugin(),
        _state = ValueNotifier<ClientViewState>(ClientViewState.initial());

  final ClientCorePlugin _plugin;
  final ValueNotifier<ClientViewState> _state;
  bool _watching = false;
  bool _networkToggleInFlight = false;
  bool? _optimisticNetworkEnabled;
  String? _networkToggleSyncReason;
  DateTime? _networkToggleStartedAt;
  int _lastStateRevision = 0;

  @override
  ValueListenable<ClientViewState> get state => _state;

  @override
  Future<void> start() async {
    await _invokeState(_plugin.start);
    _startStateWatchLoop();
  }

  @override
  Future<void> dispatch(ClientCommand command) async {
    final isNetworkToggle = command.type == ClientCommandType.enableNetwork ||
        command.type == ClientCommandType.disableNetwork;
    if (isNetworkToggle) {
      _networkToggleInFlight = true;
      _optimisticNetworkEnabled = command.type == ClientCommandType.enableNetwork;
      _networkToggleSyncReason = command.type.name;
      _networkToggleStartedAt = DateTime.now();
    }
    if (_shouldShowSyncing(command.type)) {
      _state.value = _state.value.copyWith(
        networkEnabled: _optimisticNetworkEnabled,
        syncing: true,
        syncReason: command.type.name,
        switchEnabled: false,
        error: null,
        notice: null,
      );
    }
    if (isNetworkToggle) {
      await _invokeState(() => _plugin.enqueueControlTask({
            'direction': 'upstream',
            'action': command.type == ClientCommandType.enableNetwork
                ? 'enableNetwork'
                : 'disableNetwork',
            'requireUiRefresh': false,
          }));
      return;
    }
    if (command.type == ClientCommandType.shutdownNetwork) {
      await _invokeState(_plugin.shutdownNetwork);
      return;
    }
    await _invokeState(() => _plugin.dispatch(command.toJson()));
  }

  void _startStateWatchLoop() {
    if (_watching) {
      return;
    }
    _watching = true;
    Future<void>(() async {
      final pollState = defaultTargetPlatform == TargetPlatform.windows;
      while (_watching) {
        try {
          if (pollState) {
            await Future<void>.delayed(const Duration(seconds: 1));
          }
          final result = pollState
              ? await _plugin.state()
              : await _plugin.watchState(_lastStateRevision);
          final json = _jsonMapFromResult(result);
          if (json == null) {
            await Future<void>.delayed(const Duration(seconds: 1));
            continue;
          }
          if (pollState) {
            final state = ClientViewState.fromJson(json);
            _state.value = _mergeStateWithNetworkToggle(state);
          } else {
            final revision = json['revision'];
            if (revision is int) {
              _lastStateRevision = revision;
            }
            final stateJson = json['state'];
            if (stateJson is Map) {
              final state =
                  ClientViewState.fromJson(stateJson.cast<String, Object?>());
              _state.value = _mergeStateWithNetworkToggle(state);
            }
          }
        } on Object {
          await Future<void>.delayed(const Duration(seconds: 2));
        }
      }
    });
  }

  @override
  Future<ControlTransportStatus?> controlTransportStatus() async {
    try {
      final result = await _plugin.controlTransportStatus();
      final json = _jsonMapFromResult(result);
      return json == null ? null : ControlTransportStatus.fromJson(json);
    } on Object {
      return null;
    }
  }

  bool _shouldShowSyncing(ClientCommandType type) {
    return type == ClientCommandType.enableNetwork ||
        type == ClientCommandType.disableNetwork ||
        type == ClientCommandType.logout ||
        type == ClientCommandType.shutdownNetwork;
  }

  Future<void> _invokeState(
    Future<Object?> Function() invoke, {
    bool completeNetworkToggle = false,
  }) async {
    try {
      final result = await invoke();
      final state = _stateFromResult(result);
      if (completeNetworkToggle) {
        _networkToggleInFlight = false;
        _optimisticNetworkEnabled = null;
        _networkToggleSyncReason = null;
        _networkToggleStartedAt = null;
      }
      if (state != null) {
        _state.value = state;
      }
    } on MissingPluginException {
      if (completeNetworkToggle) {
        _networkToggleInFlight = false;
        _optimisticNetworkEnabled = null;
        _networkToggleSyncReason = null;
        _networkToggleStartedAt = null;
      }
      _state.value = _state.value.copyWith(
        syncing: false,
        clearSyncReason: true,
        switchEnabled: true,
        notice: 'localServiceNotConnected',
      );
    } on PlatformException catch (error) {
      if (completeNetworkToggle) {
        _networkToggleInFlight = false;
        _optimisticNetworkEnabled = null;
        _networkToggleSyncReason = null;
        _networkToggleStartedAt = null;
      }
      _state.value = _state.value.copyWith(
        syncing: false,
        clearSyncReason: true,
        switchEnabled: true,
        error: error.message ?? error.code,
      );
    } on Object catch (error) {
      if (completeNetworkToggle) {
        _networkToggleInFlight = false;
        _optimisticNetworkEnabled = null;
        _networkToggleSyncReason = null;
        _networkToggleStartedAt = null;
      }
      _state.value = _state.value.copyWith(
        syncing: false,
        clearSyncReason: true,
        switchEnabled: true,
        error: error.toString(),
      );
    }
  }

  ClientViewState _mergeStateWithNetworkToggle(ClientViewState state) {
    if (!_networkToggleInFlight) {
      return state;
    }
    final targetEnabled = _optimisticNetworkEnabled;
    final startedAt = _networkToggleStartedAt;
    final timedOut = startedAt != null &&
        DateTime.now().difference(startedAt) > const Duration(seconds: 30);
    final settled =
        targetEnabled != null && state.networkEnabled == targetEnabled;
    if (state.error != null || settled || timedOut) {
      _networkToggleInFlight = false;
      _optimisticNetworkEnabled = null;
      _networkToggleSyncReason = null;
      _networkToggleStartedAt = null;
      if (timedOut && state.error == null) {
        return state.copyWith(
          syncing: false,
          clearSyncReason: true,
          switchEnabled: true,
          notice: '网络操作已提交，后台仍在同步状态。',
        );
      }
      return state;
    }
    return state.copyWith(
      networkEnabled: targetEnabled,
      syncing: true,
      syncReason: _networkToggleSyncReason,
      switchEnabled: false,
      error: _state.value.error,
      notice: _state.value.notice,
    );
  }

  ClientViewState? _stateFromResult(Object? result) {
    final json = _jsonMapFromResult(result);
    if (json == null) {
      return null;
    }
    if (!json.containsKey('networkEnabled') && !json.containsKey('signedIn')) {
      return null;
    }
    return ClientViewState.fromJson(json);
  }

  Map<String, Object?>? _jsonMapFromResult(Object? result) {
    if (result is Map) {
      return result.cast<String, Object?>();
    }
    if (result is String && result.trim().isNotEmpty) {
      return jsonDecode(result) as Map<String, Object?>;
    }
    return null;
  }
}
