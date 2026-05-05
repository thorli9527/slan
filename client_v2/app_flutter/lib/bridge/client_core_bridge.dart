import 'dart:async';
import 'dart:io';

import 'package:client_core_plugin/client_core_plugin.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';

import 'android_network_authorization.dart';
import 'client_commands.dart';
import 'client_core_local_service.dart';
import 'control_transport_status.dart';
import 'client_ui_diagnostics.dart';
import 'client_view_state.dart';

abstract final class ClientBusinessEventType {
  static const sessionChanged = 'session.changed';
  static const networkSwitchFinished = 'network.switch.finished';
  static const networkRuntimeChanged = 'network.runtime.changed';
  static const networkSwitchFailed = 'network.switch.failed';
  static const controlSyncChanged = 'control.sync.changed';
  static const stateChanged = 'state.changed';
}

class _NetworkToggleOperation {
  const _NetworkToggleOperation({
    required this.epoch,
    required this.command,
    required this.method,
    required this.targetEnabled,
    required this.previousState,
  });

  final int epoch;
  final ClientCommandType command;
  final String method;
  final bool targetEnabled;
  final ClientViewState previousState;
}

abstract interface class ClientCoreBridge {
  ValueListenable<ClientViewState> get state;
  ValueListenable<AndroidNetworkAuthorizationState>
      get androidNetworkAuthorization;

  Future<void> start();
  Future<void> prepareAndroidNetworkAuthorization();
  Future<void> dispatch(ClientCommand command);
  Future<ControlTransportStatus?> localControlStatus();
}

class MethodChannelClientCoreBridge implements ClientCoreBridge {
  MethodChannelClientCoreBridge({String? localServiceHost})
      : _plugin = ClientCorePlugin(),
        _localService = ClientCoreLocalService(host: localServiceHost),
        _state = ValueNotifier<ClientViewState>(ClientViewState.initial()),
        _androidNetworkAuthorization =
            ValueNotifier<AndroidNetworkAuthorizationState>(
          AndroidNetworkAuthorizationState.initial,
        );

  final ClientCorePlugin _plugin;
  final ClientCoreLocalService _localService;
  final ValueNotifier<ClientViewState> _state;
  final ValueNotifier<AndroidNetworkAuthorizationState>
      _androidNetworkAuthorization;
  int _networkToggleEpoch = 0;
  int _lastBusinessEventRevision = 0;
  bool _networkToggleInFlight = false;
  _NetworkToggleOperation? _networkToggleOperation;
  bool _localLogoutRequested = false;
  bool _watchingBusinessEvents = false;
  bool _watchingAndroidNetworkEvents = false;

  @override
  ValueListenable<ClientViewState> get state => _state;

  @override
  ValueListenable<AndroidNetworkAuthorizationState>
      get androidNetworkAuthorization => _androidNetworkAuthorization;

  @override
  Future<void> start() async {
    ClientUiDiagnostics.unawaitedLog('bridge.start.begin', state: _state.value);
    await _invokeState(_plugin.start);
    _startBusinessEventWatchLoop();
    _startAndroidNetworkEventWatchLoop();
    ClientUiDiagnostics.unawaitedLog('bridge.start.end', state: _state.value);
  }

  @override
  Future<void> prepareAndroidNetworkAuthorization() async {
    if (!Platform.isAndroid) {
      return;
    }
    _androidNetworkAuthorization.value =
        _androidNetworkAuthorization.value.copyWith(
      checking: true,
      error: null,
    );
    try {
      final permissionStateResult = await _plugin.androidVpnPermissionState();
      final permissionState =
          ClientCoreLocalService.stringResult(permissionStateResult);
      AndroidVpnConsentRequest? consentRequest;
      AndroidVpnSessionConfig? networkConfig;
      if (permissionState == AndroidVpnPermissionState.needsUserConsent) {
        consentRequest = await _plugin.androidRequestVpnPermission();
      }
      if (permissionState == AndroidVpnPermissionState.granted &&
          _state.value.signedIn) {
        networkConfig = await _localService.localAndroidNetworkConfig();
      }
      _androidNetworkAuthorization.value = AndroidNetworkAuthorizationState(
        checking: false,
        permissionState: permissionState,
        consentRequest: consentRequest,
        networkConfig: networkConfig,
      );
      ClientUiDiagnostics.unawaitedLog(
        'bridge.android.authorization.prepared',
        state: _state.value,
        fields: {
          'permissionState': permissionState,
          'hasConsentRequest': consentRequest != null,
          'hasNetworkConfig': networkConfig != null,
        },
      );
    } on Object catch (error) {
      _androidNetworkAuthorization.value =
          _androidNetworkAuthorization.value.copyWith(
        checking: false,
        error: error.toString(),
        clearNetworkConfig: true,
      );
      ClientUiDiagnostics.unawaitedLog(
        'bridge.android.authorization.failed',
        state: _state.value,
        fields: {'message': error.toString()},
      );
    }
  }

  @override
  Future<void> dispatch(ClientCommand command) async {
    ClientUiDiagnostics.unawaitedLog(
      'bridge.dispatch.begin',
      state: _state.value,
      fields: {'command': command.type.name},
    );
    if (command.type == ClientCommandType.loginWithBrowser) {
      _localLogoutRequested = false;
    }
    if (command.type == ClientCommandType.logout) {
      _clearNetworkToggle();
      _localLogoutRequested = true;
      _setStateIfChanged(ClientViewState.initial());
      ClientUiDiagnostics.unawaitedLog(
        'bridge.logout.localCleared',
        state: _state.value,
      );
      try {
        await _requestLocalService('localLogout');
        ClientUiDiagnostics.unawaitedLog(
          'bridge.logout.serviceCleared',
          state: _state.value,
        );
      } on Object catch (error) {
        ClientUiDiagnostics.unawaitedLog(
          'bridge.logout.serviceClearFailed',
          state: _state.value,
          fields: {'message': error.toString()},
        );
        try {
          await _plugin.dispatch(command.toJson());
        } on Object {
          // Logout is local-first: the UI session is already cleared.
        }
      }
      await _refreshState();
      return;
    }
    if (_isNetworkToggle(command.type)) {
      if (Platform.isAndroid) {
        _startAsyncAndroidNetworkToggle(command);
        return;
      }
      _startAsyncNetworkToggle(command);
      return;
    }
    if (command.type == ClientCommandType.localNetworkShutdown) {
      await _requestLocalService('localNetworkShutdown');
      await _refreshState();
      return;
    }
    await _invokeState(() => _plugin.dispatch(command.toJson()));
  }

  @override
  Future<ControlTransportStatus?> localControlStatus() async {
    try {
      final json = await _localService.localControlStatus();
      return json == null ? null : ControlTransportStatus.fromJson(json);
    } on Object {
      return null;
    }
  }

  bool _isNetworkToggle(ClientCommandType type) {
    return type == ClientCommandType.enableNetwork ||
        type == ClientCommandType.disableNetwork;
  }

  void _clearNetworkToggle() {
    _networkToggleEpoch++;
    _networkToggleInFlight = false;
    _networkToggleOperation = null;
  }

  void _startAsyncNetworkToggle(ClientCommand command) {
    if (_networkToggleInFlight) {
      ClientUiDiagnostics.unawaitedLog(
        'bridge.switch.ignoredInFlight',
        state: _state.value,
        fields: {'command': command.type.name},
      );
      return;
    }
    final previousState = _state.value;
    final epoch = ++_networkToggleEpoch;
    _networkToggleInFlight = true;
    final targetEnabled = command.type == ClientCommandType.enableNetwork;
    final method =
        targetEnabled ? 'localNetworkActivate' : 'localNetworkDeactivate';
    final operation = _NetworkToggleOperation(
      epoch: epoch,
      command: command.type,
      method: method,
      targetEnabled: targetEnabled,
      previousState: previousState,
    );
    _networkToggleOperation = operation;
    _setStateIfChanged(_state.value.copyWith(
      networkEnabled: targetEnabled,
      syncing: true,
      syncReason: command.type.name,
      switchEnabled: false,
      error: null,
      notice: null,
      clearVirtualIp: command.type == ClientCommandType.disableNetwork,
    ));
    ClientUiDiagnostics.unawaitedLog(
      'bridge.switch.serviceApi',
      state: _state.value,
      fields: {'method': method, 'command': command.type.name},
    );
    ClientUiDiagnostics.unawaitedLog(
      'bridge.switch.pending',
      state: _state.value,
      fields: {
        'command': command.type.name,
        'epoch': epoch,
        'optimisticNetworkEnabled': targetEnabled,
      },
    );
    unawaited(
      Future<void>(() async {
        await Future<void>.delayed(_networkToggleTimeout);
        if (!_isCurrentNetworkToggle(operation)) {
          return;
        }
        _finishNetworkToggle(operation);
        _setStateIfChanged(_networkToggleFailureState(
          operation,
          'network switch timed out',
        ));
        ClientUiDiagnostics.unawaitedLog(
          'bridge.switch.eventTimeout',
          state: _state.value,
          fields: {'command': command.type.name, 'epoch': epoch},
        );
      }),
    );
    unawaited(
      Future<void>(() async {
        try {
          final result =
              await _requestLocalService(method).timeout(_networkToggleTimeout);
          ClientUiDiagnostics.unawaitedLog(
            'bridge.switch.serviceResult',
            state: _state.value,
            fields: {
              'command': command.type.name,
              'epoch': epoch,
              'stale': epoch != _networkToggleEpoch,
              'hasResult': result != null,
            },
          );
          if (!_isCurrentNetworkToggle(operation)) {
            return;
          }
          final state = _stateFromResult(result);
          if (state != null &&
              state.error != null &&
              state.error!.trim().isNotEmpty) {
            _finishNetworkToggle(operation);
            _setStateIfChanged(_networkToggleFailureState(
              operation,
              state.error!,
            ));
            ClientUiDiagnostics.unawaitedLog(
              'bridge.switch.serviceReturnedError',
              state: _state.value,
              fields: {
                'command': command.type.name,
                'epoch': epoch,
                'message': state.error,
              },
            );
            return;
          }
          ClientUiDiagnostics.unawaitedLog(
            'bridge.switch.waitingBusinessEvent',
            state: _state.value,
            fields: {'command': command.type.name, 'epoch': epoch},
          );
        } on MissingPluginException {
          if (!_isCurrentNetworkToggle(operation)) {
            return;
          }
          _finishNetworkToggle(operation);
          _setStateIfChanged(_networkToggleFailureState(
            operation,
            'local service not connected',
            notice: 'localServiceNotConnected',
          ));
          ClientUiDiagnostics.unawaitedLog(
            'bridge.switch.missingPlugin',
            state: _state.value,
            fields: {'command': command.type.name, 'epoch': epoch},
          );
        } on TimeoutException catch (error) {
          if (!_isCurrentNetworkToggle(operation)) {
            return;
          }
          _finishNetworkToggle(operation);
          _setStateIfChanged(_networkToggleFailureState(
            operation,
            'network switch timed out',
          ));
          ClientUiDiagnostics.unawaitedLog(
            'bridge.switch.timeout',
            state: _state.value,
            fields: {
              'command': command.type.name,
              'epoch': epoch,
              'message': error.toString(),
            },
          );
        } on PlatformException catch (error) {
          if (!_isCurrentNetworkToggle(operation)) {
            return;
          }
          _finishNetworkToggle(operation);
          _setStateIfChanged(_networkToggleFailureState(
            operation,
            error.message ?? error.code,
          ));
          ClientUiDiagnostics.unawaitedLog(
            'bridge.switch.platformError',
            state: _state.value,
            fields: {
              'command': command.type.name,
              'epoch': epoch,
              'code': error.code,
              'message': error.message,
            },
          );
        } on Object catch (error) {
          if (!_isCurrentNetworkToggle(operation)) {
            return;
          }
          _finishNetworkToggle(operation);
          _setStateIfChanged(_networkToggleFailureState(
            operation,
            error.toString(),
          ));
          ClientUiDiagnostics.unawaitedLog(
            'bridge.switch.error',
            state: _state.value,
            fields: {
              'command': command.type.name,
              'epoch': epoch,
              'message': error.toString(),
            },
          );
        }
      }),
    );
  }

  void _startAsyncAndroidNetworkToggle(ClientCommand command) {
    if (_networkToggleInFlight) {
      ClientUiDiagnostics.unawaitedLog(
        'bridge.android.switch.ignoredInFlight',
        state: _state.value,
        fields: {'command': command.type.name},
      );
      return;
    }
    final previousState = _state.value;
    final epoch = ++_networkToggleEpoch;
    _networkToggleInFlight = true;
    final targetEnabled = command.type == ClientCommandType.enableNetwork;
    final operation = _NetworkToggleOperation(
      epoch: epoch,
      command: command.type,
      method: targetEnabled ? 'androidStartVpn' : 'androidStopVpn',
      targetEnabled: targetEnabled,
      previousState: previousState,
    );
    _networkToggleOperation = operation;
    _setStateIfChanged(_state.value.copyWith(
      networkEnabled: targetEnabled,
      syncing: true,
      syncReason: command.type.name,
      switchEnabled: false,
      error: null,
      notice: null,
      clearVirtualIp: !targetEnabled,
    ));
    unawaited(
      Future<void>(() async {
        try {
          if (targetEnabled) {
            await prepareAndroidNetworkAuthorization();
            if (!_isCurrentNetworkToggle(operation)) {
              return;
            }
            final authorization = _androidNetworkAuthorization.value;
            if (authorization.needsUserConsent) {
              throw StateError('Android 网络需要授权后才能启用');
            }
            final config = authorization.networkConfig;
            if (!authorization.granted || config == null) {
              throw StateError('Android 网络配置未就绪');
            }
            await _plugin
                .androidStartVpn(config)
                .timeout(_networkToggleTimeout);
          } else {
            await _plugin.androidStopVpn().timeout(_networkToggleTimeout);
          }
          if (!_isCurrentNetworkToggle(operation)) {
            return;
          }
          _finishNetworkToggle(operation);
          _setStateIfChanged(_state.value.copyWith(
            networkEnabled: targetEnabled,
            syncing: false,
            clearSyncReason: true,
            switchEnabled: true,
            notice: targetEnabled ? 'networkEnabled' : 'networkDisabled',
            virtualIp: targetEnabled
                ? _androidNetworkAuthorization.value.networkConfig?.virtualIp
                : null,
            clearVirtualIp: !targetEnabled,
          ));
          ClientUiDiagnostics.unawaitedLog(
            'bridge.android.switch.finished',
            state: _state.value,
            fields: {'command': command.type.name, 'epoch': epoch},
          );
        } on Object catch (error) {
          if (!_isCurrentNetworkToggle(operation)) {
            return;
          }
          _finishNetworkToggle(operation);
          _setStateIfChanged(_networkToggleFailureState(
            operation,
            error.toString(),
          ));
          _androidNetworkAuthorization.value =
              _androidNetworkAuthorization.value.copyWith(
            checking: false,
            error: error.toString(),
          );
          ClientUiDiagnostics.unawaitedLog(
            'bridge.android.switch.failed',
            state: _state.value,
            fields: {
              'command': command.type.name,
              'epoch': epoch,
              'message': error.toString(),
            },
          );
        }
      }),
    );
  }

  static const Duration _networkToggleTimeout = Duration(seconds: 45);

  bool _isCurrentNetworkToggle(_NetworkToggleOperation operation) {
    return _networkToggleOperation == operation &&
        operation.epoch == _networkToggleEpoch;
  }

  void _finishNetworkToggle(_NetworkToggleOperation operation) {
    if (!_isCurrentNetworkToggle(operation)) {
      return;
    }
    _networkToggleInFlight = false;
    _networkToggleOperation = null;
    _networkToggleEpoch++;
  }

  ClientViewState _networkToggleFailureState(
    _NetworkToggleOperation operation,
    String error, {
    String? notice,
  }) {
    return operation.previousState.copyWith(
      syncing: false,
      clearSyncReason: true,
      switchEnabled: true,
      notice: notice,
      error: error,
      errorSource: ClientErrorSource.networkSwitch,
      clearVirtualIp: !operation.previousState.networkEnabled,
    );
  }

  Future<void> _invokeState(Future<Object?> Function() invoke) async {
    try {
      final result = await invoke();
      final state = _stateFromResult(result);
      if (state != null) {
        _setStateIfChanged(state);
      }
    } on MissingPluginException {
      _setStateIfChanged(_state.value.copyWith(
        syncing: false,
        clearSyncReason: true,
        switchEnabled: true,
        notice: 'localServiceNotConnected',
      ));
    } on PlatformException catch (error) {
      _setStateIfChanged(_state.value.copyWith(
        syncing: false,
        clearSyncReason: true,
        switchEnabled: true,
        error: error.message ?? error.code,
      ));
    } on Object catch (error) {
      _setStateIfChanged(_state.value.copyWith(
        syncing: false,
        clearSyncReason: true,
        switchEnabled: true,
        error: error.toString(),
      ));
    }
  }

  Future<void> _refreshState() async {
    await _invokeState(_plugin.refresh);
  }

  void _startBusinessEventWatchLoop() {
    if (_watchingBusinessEvents) {
      return;
    }
    _watchingBusinessEvents = true;
    unawaited(
      Future<void>(() async {
        while (_watchingBusinessEvents) {
          try {
            final json = await _localService.localBusinessEventWatch(
              lastRevision: _lastBusinessEventRevision,
            );
            if (json == null) {
              continue;
            }
            final revision = json['revision'];
            if (revision is! int || revision <= _lastBusinessEventRevision) {
              continue;
            }
            _lastBusinessEventRevision = revision;
            final next = await _stateAfterBusinessEvent(json);
            if (next != null) {
              _setStateIfChanged(next);
            }
          } on Object catch (error) {
            ClientUiDiagnostics.unawaitedLog(
              'bridge.businessEvent.watchError',
              state: _state.value,
              fields: {'message': error.toString()},
            );
            await Future<void>.delayed(const Duration(seconds: 2));
          }
        }
      }),
    );
  }

  void _startAndroidNetworkEventWatchLoop() {
    if (!Platform.isAndroid || _watchingAndroidNetworkEvents) {
      return;
    }
    _watchingAndroidNetworkEvents = true;
    unawaited(
      Future<void>(() async {
        while (_watchingAndroidNetworkEvents) {
          try {
            final event = await _plugin
                .androidPollNetworkEvent()
                .timeout(const Duration(seconds: 35));
            if (event != null) {
              _androidNetworkAuthorization.value =
                  _androidNetworkAuthorization.value.applyEvent(event);
              final runtimeState = event.runtimeState;
              if (runtimeState != null) {
                final networkEnabled = runtimeState['networkEnabled'] == true;
                _setStateIfChanged(_state.value.copyWith(
                  networkEnabled: networkEnabled,
                  virtualIp: runtimeState['virtualIp'] as String?,
                  syncing: false,
                  clearSyncReason: true,
                  switchEnabled: true,
                  notice: event.eventType,
                  error: event.eventType == AndroidNetworkEventType.error
                      ? event.message
                      : null,
                  errorSource: event.eventType == AndroidNetworkEventType.error
                      ? ClientErrorSource.networkSwitch
                      : null,
                  clearVirtualIp: !networkEnabled,
                ));
              }
              if (event.eventType == AndroidNetworkEventType.vpnStarted ||
                  event.eventType == AndroidNetworkEventType.vpnStopped ||
                  event.eventType == AndroidNetworkEventType.error) {
                _settleNetworkToggleFromEvent();
              }
            }
          } on MissingPluginException {
            await Future<void>.delayed(const Duration(seconds: 5));
          } on TimeoutException {
            // Polling methods may long-poll; a timeout simply starts the next cycle.
          } on Object catch (error) {
            ClientUiDiagnostics.unawaitedLog(
              'bridge.android.event.watchError',
              state: _state.value,
              fields: {'message': error.toString()},
            );
            await Future<void>.delayed(const Duration(seconds: 2));
          }
        }
      }),
    );
  }

  Future<ClientViewState?> _stateAfterBusinessEvent(
    Map<String, Object?> event,
  ) async {
    final type = event['businessType'] as String?;
    if (!_businessEventRequiresStateQuery(type)) {
      return _reduceBusinessEvent(event);
    }
    try {
      final state = _stateFromResult(await _localService.localState());
      if (state != null) {
        ClientUiDiagnostics.unawaitedLog(
          'bridge.businessEvent.stateQueried',
          state: _state.value,
          fields: {'businessType': type},
        );
        return _reduceBusinessEvent(event, queriedState: state);
      }
    } on Object catch (error) {
      ClientUiDiagnostics.unawaitedLog(
        'bridge.businessEvent.stateQueryFailed',
        state: _state.value,
        fields: {
          'businessType': type,
          'message': error.toString(),
        },
      );
    }
    return _reduceBusinessEvent(event);
  }

  bool _businessEventRequiresStateQuery(String? type) {
    if (type == ClientBusinessEventType.networkRuntimeChanged) {
      return _networkToggleInFlight;
    }
    return type == ClientBusinessEventType.networkSwitchFinished ||
        type == ClientBusinessEventType.networkSwitchFailed;
  }

  ClientViewState? _reduceBusinessEvent(
    Map<String, Object?> event, {
    ClientViewState? queriedState,
  }) {
    final type = event['businessType'] as String?;
    final businessData = event['businessData'];
    final snapshot = event['snapshot'];
    final dataState = businessData is Map
        ? _stateFromResult(businessData.cast<String, Object?>())
        : null;
    final snapshotState = snapshot is Map
        ? _stateFromResult(snapshot.cast<String, Object?>())
        : null;
    final incoming = queriedState ?? dataState ?? snapshotState;
    if (incoming == null) {
      return null;
    }

    switch (type) {
      case ClientBusinessEventType.sessionChanged:
        return _state.value.copyWith(
          signedIn: incoming.signedIn,
          userLabel: incoming.userLabel,
          deviceId: incoming.deviceId,
          authCallbackId: incoming.authCallbackId,
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
        _settleNetworkToggleFromEvent();
        return incoming.copyWith(
          syncing: false,
          clearSyncReason: true,
          switchEnabled: true,
          clearVirtualIp: !incoming.networkEnabled,
        );
      case ClientBusinessEventType.networkSwitchFailed:
        _settleNetworkToggleFromEvent();
        final error = incoming.error ??
            dataState?.error ??
            snapshotState?.error ??
            'network switch failed';
        return _state.value.copyWith(
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
        return incoming;
    }
  }

  void _settleNetworkToggleFromEvent() {
    if (!_networkToggleInFlight && _networkToggleOperation == null) {
      return;
    }
    _networkToggleInFlight = false;
    _networkToggleOperation = null;
    _networkToggleEpoch++;
  }

  void _setStateIfChanged(ClientViewState state) {
    final next = _localLogoutRequested ? ClientViewState.initial() : state;
    if (_state.value == next) {
      return;
    }
    _state.value = next;
  }

  Future<Object?> _requestLocalService(String method,
      [Object? arguments]) async {
    ClientUiDiagnostics.unawaitedLog(
      'bridge.localService.request',
      state: _state.value,
      fields: {'method': method},
    );
    final response = await _localService.request(method, arguments: arguments);
    ClientUiDiagnostics.unawaitedLog(
      'bridge.localService.response',
      state: _state.value,
      fields: {
        'method': method,
        'bytes': response is String ? response.length : 0,
      },
    );
    return response;
  }

  ClientViewState? _stateFromResult(Object? result) {
    final json = ClientCoreLocalService.jsonMapFromResult(result);
    if (json == null) {
      return null;
    }
    if (!json.containsKey('networkEnabled') && !json.containsKey('signedIn')) {
      return null;
    }
    return ClientViewState.fromJson(json);
  }
}
