part of 'home_page.dart';

extension _HomePageLogic on _HomePageState {
  String _currentUserLabel(
    AppSessionStore sessionStore,
  ) {
    final sessionLabel = sessionStore.session?.userLabel?.trim();
    final ownerEmail = sessionStore.device?.ownerEmail?.trim();
    final userId = sessionStore.session?.userId.trim();
    return sessionLabel != null && sessionLabel.isNotEmpty
        ? sessionLabel
        : ownerEmail != null && ownerEmail.isNotEmpty
            ? ownerEmail
            : userId != null && userId.isNotEmpty
                ? userId
                : 'Unknown user';
  }

  bool _runtimeStateIsEnabled(String runtimeState) {
    final normalized = runtimeState.toLowerCase().trim();
    return normalized != 'idle' &&
        normalized != 'inactive' &&
        normalized != 'disabled';
  }

  Future<void> _ensureWorkspaceReady(AppSessionStore sessionStore) async {
    if (sessionStore.session == null) {
      return;
    }
    await StartupLog.write(
      'home ensureWorkspaceReady start userId=${sessionStore.session?.userId} deviceId=${sessionStore.device?.deviceId}',
    );

    final host = Platform.localHostname.replaceAll('.', '-');
    final statusMessage =
        await AppCoreScope.sessionController.ensureHomeWorkspaceReady(
      deviceName: host,
      platform: DesktopPlatform.currentId,
      deviceVersion: DesktopPlatform.currentVersion,
      machineId: AppCoreScope.clientMachineId,
      devicePublicKey: 'device-key-${DateTime.now().microsecondsSinceEpoch}',
    );
    if (!mounted) {
      return;
    }
    await AppCoreScope.sessionController.refreshHelperStatus();

    final activeNetwork = sessionStore.selectedNetwork;
    _stopMissingNetworkPolling();
    if (activeNetwork == null) {
      await StartupLog.write('home ensureWorkspaceReady no active network');
      await _openNetworkConsoleIfNeeded();
      return;
    }
    if (statusMessage != null) {
      await StartupLog.write('home network status: $statusMessage');
    }
    await StartupLog.write(
      'home ensureWorkspaceReady done activeNetwork=${activeNetwork.networkId} status=${statusMessage ?? '-'}',
    );
  }

  Future<void> _openBrowserLogin() async {
    final target =
        AppCoreScope.hostConfig?.authLoginUrl ?? AppCoreScope.webConsoleUrl;
    if (target == null || target.isEmpty) {
      await StartupLog.write('home browser login skipped: missing target');
      return;
    }
    final currentDeviceId = AppCoreScope.sessionStore.device?.deviceId.trim();
    final loginTargetDeviceId =
        currentDeviceId != null && currentDeviceId.isNotEmpty
            ? currentDeviceId
            : AppCoreScope.clientMachineId;
    try {
      await widget.authCallbackGateway.preparePendingServerCallback(
        preferredKey: loginTargetDeviceId,
      );
      final loginUrl = Uri.parse(target).replace(queryParameters: {
        ...Uri.parse(target).queryParameters,
        'deviceId': loginTargetDeviceId,
      }).toString();
      await StartupLog.write('home browser login target=$loginUrl');
      await _openExternalUrl(loginUrl);
      await StartupLog.write('home browser login launched');
    } catch (error) {
      AppCoreScope.sessionStore.error = '打开浏览器登录失败：$error';
      AppCoreScope.sessionStore.notifyListeners();
      await StartupLog.write('home browser login failed: $error');
    }
  }

  Future<void> _openWebDetails() async {
    final target = AppCoreScope.webConsoleUrl;
    if (target == null || target.isEmpty) {
      return;
    }
    await _enableExistingNetworkBeforeConsole();
    final loginKey = await _createConsoleLoginKey();
    final uri = Uri.parse(target);
    await _openExternalUrl(
      uri.replace(queryParameters: {
        ...uri.queryParameters,
        if (loginKey != null) 'consoleLoginKey': loginKey,
        if (AppCoreScope.sessionStore.device?.deviceId.trim().isNotEmpty ==
            true)
          'deviceId': AppCoreScope.sessionStore.device!.deviceId.trim(),
      }).toString(),
    );
  }

  Future<void> _enableExistingNetworkBeforeConsole() async {
    final sessionStore = AppCoreScope.sessionStore;
    if (sessionStore.selectedNetwork == null) {
      return;
    }
    final runtimeState =
        AppCoreScope.tunnelStore.tunnelRuntimeView?.state.toLowerCase().trim();
    final alreadyEnabled = runtimeState != null &&
        runtimeState != 'idle' &&
        runtimeState != 'inactive' &&
        runtimeState != 'disabled';
    if (alreadyEnabled || sessionStore.busy) {
      return;
    }
    try {
      await AppCoreScope.sessionController.enableActiveNetwork();
    } catch (error) {
      await StartupLog.write('open web console auto-enable skipped: $error');
    }
  }

  Future<String?> _createConsoleLoginKey() async {
    final baseUrl = AppCoreScope.controlBaseUrl;
    final token = AppCoreScope.sessionStore.session?.accessToken.trim();
    if (baseUrl == null || baseUrl.isEmpty || token == null || token.isEmpty) {
      return null;
    }
    final client = HttpClient();
    client.connectionTimeout = const Duration(seconds: 8);
    try {
      final uri = Uri.parse(baseUrl).replace(path: '/auth/console-login-key');
      final request = await client.postUrl(uri);
      request.headers.contentType = ContentType.json;
      request.headers.set(HttpHeaders.authorizationHeader, 'Bearer $token');
      request.write(jsonEncode({
        'deviceId': AppCoreScope.sessionStore.device?.deviceId.trim(),
      }));
      final response = await request.close().timeout(
            const Duration(seconds: 8),
          );
      final body = await utf8.decoder.bind(response).join().timeout(
            const Duration(seconds: 8),
          );
      if (response.statusCode < 200 || response.statusCode >= 300) {
        await StartupLog.write(
          'create console login key failed status=${response.statusCode} body=$body',
        );
        return null;
      }
      final payload = jsonDecode(body) as Map<String, dynamic>;
      return payload['loginKey'] as String?;
    } catch (error) {
      await StartupLog.write('create console login key skipped: $error');
      return null;
    } finally {
      client.close(force: true);
    }
  }

  Future<void> _logoutFromClient() async {
    await widget.authCallbackGateway.clearPendingServerCallback();
    await AppCoreScope.sessionController.signOut();
  }

  Future<void> _enableActiveNetworkWithPrompt(
    NetworkMemberModel? currentMember,
  ) async {
    if (_memberIsDisabled(currentMember)) {
      await _showManagedDeviceDisabledDialog();
      return;
    }
    await _runNetworkToggle(enable: true);
    if (!mounted) {
      return;
    }
    if (_isDeviceUnavailableError(AppCoreScope.sessionStore.error)) {
      await _showManagedDeviceDisabledDialog();
    }
  }

  Future<void> _disableActiveNetworkSmoothly() async {
    await _runNetworkToggle(enable: false);
  }

  Future<void> _runNetworkToggle({required bool enable}) async {
    if (_networkToggleBusy) {
      return;
    }
    _markNetworkToggleBusy(enable);
    try {
      await StartupLog.write(
        'home network switch requested enable=$enable '
        'selectedNetwork=${AppCoreScope.sessionStore.selectedNetworkId ?? '-'} '
        'device=${AppCoreScope.sessionStore.device?.deviceId ?? '-'}',
      );
      if (enable) {
        await AppCoreScope.sessionController.enableActiveNetwork();
      } else {
        await AppCoreScope.sessionController.disableActiveNetwork();
      }
      if (AppCoreScope.sessionStore.error != null) {
        await StartupLog.write(
          'home network switch preflight failed enable=$enable '
          'error=${AppCoreScope.sessionStore.error}',
        );
        return;
      }
      await _waitForNetworkToggleSync(enable: enable);
      await StartupLog.write(
        'home network switch completed enable=$enable '
        'error=${AppCoreScope.sessionStore.error ?? '-'} '
        'runtime=${AppCoreScope.tunnelStore.tunnelRuntimeView?.state ?? '-'}',
      );
    } finally {
      if (mounted) {
        _clearNetworkToggleBusy();
      }
    }
  }

  Future<void> _waitForNetworkToggleSync({required bool enable}) async {
    const pollInterval = Duration(milliseconds: 500);
    final deadline = DateTime.now().add(const Duration(seconds: 20));
    while (mounted && DateTime.now().isBefore(deadline)) {
      final status = await AppCoreScope.sessionController.refreshHelperStatus();
      final helperNetworkId = status.currentNetworkId?.trim();
      final serviceRuntimeActive = status.helperReachable &&
          status.tunnelBackendRunning &&
          helperNetworkId != null &&
          helperNetworkId.isNotEmpty;
      final expectedRefreshReason =
          enable ? 'network_enabled' : 'network_disabled';
      final completedRequestedTask = status.mqttControlUiRefreshRequired &&
          status.mqttControlUiRefreshReason?.trim().toLowerCase() ==
              expectedRefreshReason;
      final runtimeState =
          AppCoreScope.tunnelStore.tunnelRuntimeView?.state ?? 'idle';
      final uiRuntimeEnabled = _runtimeStateIsEnabled(runtimeState);
      final settled = enable
          ? completedRequestedTask && serviceRuntimeActive && uiRuntimeEnabled
          : completedRequestedTask && !serviceRuntimeActive && !uiRuntimeEnabled;
      if (settled) {
        await StartupLog.write(
          'home network switch synced enable=$enable '
          'helperActive=$serviceRuntimeActive runtime=$runtimeState '
          'task=${status.mqttControlUiRefreshTaskId ?? '-'}',
        );
        return;
      }
      await Future<void>.delayed(pollInterval);
    }
    await StartupLog.write(
      'home network switch sync timeout enable=$enable '
      'runtime=${AppCoreScope.tunnelStore.tunnelRuntimeView?.state ?? '-'}',
    );
    if (mounted) {
      AppCoreScope.sessionStore.notice = enable
          ? '网络启用任务已提交，后台仍在同步状态。'
          : '网络停用任务已提交，后台仍在同步状态。';
      AppCoreScope.sessionStore.emit();
    }
  }

  bool _memberIsDisabled(NetworkMemberModel? currentMember) {
    final status = currentMember?.status?.trim().toLowerCase();
    return status == 'disabled' || status == 'suspended';
  }

  bool _isDeviceUnavailableError(String? error) {
    final normalized = error?.toLowerCase() ?? '';
    return normalized.contains('device unavailable') ||
        normalized.contains('contact administrator') ||
        normalized.contains('联系管理员') ||
        (normalized.contains('forbidden') && normalized.contains('disabled'));
  }

  Future<void> _showManagedDeviceDisabledDialog() async {
    if (!mounted) {
      return;
    }
    await showDialog<void>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('设备不可用'),
        content: const Text('当前设备已被网络管理员停用，请联系管理员重新启用后再连接。'),
        actions: [
          FilledButton(
            onPressed: () => Navigator.of(dialogContext).pop(),
            child: const Text('知道了'),
          ),
        ],
      ),
    );
  }

  // ignore: unused_element
  Future<void> _showDeviceUnavailableDialog() async {
    if (!mounted) {
      return;
    }
    await showDialog<void>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('设备不可用'),
        content: const Text('当前设备已被管理员停用，请联系管理员启用。'),
        actions: [
          FilledButton(
            onPressed: () => Navigator.of(dialogContext).pop(),
            child: const Text('知道了'),
          ),
        ],
      ),
    );
  }

  Future<void> _showServerSettingsDialog(BuildContext context) async {
    final controller = TextEditingController(
      text: AppCoreScope.hostInput ?? '127.0.0.1',
    );
    await showDialog<void>(
      context: context,
      builder: (dialogContext) {
        return AlertDialog(
          title: const Text('Server Host'),
          content: SizedBox(
            width: 420,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                TextField(
                  controller: controller,
                  decoration: const InputDecoration(
                    labelText: 'Server Host',
                    hintText: '127.0.0.1 / slan.localhost / your-host',
                  ),
                ),
                const SizedBox(height: 12),
                Text(
                  'Only one host value is needed. The client derives control and web endpoints from it automatically.',
                  style: Theme.of(dialogContext).textTheme.bodySmall,
                ),
              ],
            ),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.of(dialogContext).pop(),
              child: const Text('Cancel'),
            ),
            FilledButton(
              onPressed: () async {
                AppCoreScope.configureHost(host: controller.text);
                await AppCoreScope.sessionController.refreshHelperStatus();
                if (dialogContext.mounted) {
                  Navigator.of(dialogContext).pop();
                }
              },
              child: const Text('Save'),
            ),
          ],
        );
      },
    );
  }

  Future<void> _openNetworkConsoleIfNeeded() async {
    final target = AppCoreScope.webConsoleUrl;
    if (target == null || target.isEmpty) {
      await StartupLog.write(
        'No network is assigned and no web console URL is configured.',
      );
      return;
    }
    await StartupLog.write(
      'No network is assigned yet. Open the web console manually to create a network, confirm invitations, or manage DNS: $target',
    );
  }

  void _stopMissingNetworkPolling() {
    _networkPollingTimer?.cancel();
    _networkPollingTimer = null;
  }

  Future<void> _openExternalUrl(String url) async {
    await StartupLog.write('open external url target=$url');
    await widget.urlLauncher.open(url);
  }

  NetworkMemberModel? _memberForCurrentDevice(
    String? deviceId,
    NetworkModel? network,
  ) {
    if (deviceId == null || network == null) {
      return null;
    }
    for (final member in network.members) {
      if (member.deviceId == deviceId) {
        return member;
      }
    }
    return null;
  }
}
