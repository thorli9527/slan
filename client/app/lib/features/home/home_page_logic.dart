part of 'home_page.dart';

extension _HomePageLogic on _HomePageState {
  String _currentUserLabel(
    AppSessionStore sessionStore, {
    required String virtualIp,
  }) {
    final sessionLabel = sessionStore.session?.userLabel?.trim();
    final ownerEmail = sessionStore.device?.ownerEmail?.trim();
    final userId = sessionStore.session?.userId.trim();
    final label = sessionLabel != null && sessionLabel.isNotEmpty
        ? sessionLabel
        : ownerEmail != null && ownerEmail.isNotEmpty
            ? ownerEmail
            : userId != null && userId.isNotEmpty
                ? userId
                : 'Unknown user';
    final ip = virtualIp.trim();
    if (ip.isEmpty || ip == 'No network' || ip == 'Pending allocation') {
      return label;
    }
    return '$label ($ip)';
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
      return;
    }
    final currentDeviceId = AppCoreScope.sessionStore.device?.deviceId.trim();
    final loginTargetDeviceId =
        currentDeviceId != null && currentDeviceId.isNotEmpty
            ? currentDeviceId
            : AppCoreScope.clientMachineId;
    await AuthCallbackService.preparePendingServerCallback(
      preferredKey: loginTargetDeviceId,
    );
    await _openExternalUrl(
      Uri.parse(target).replace(queryParameters: {
        ...Uri.parse(target).queryParameters,
        'deviceId': loginTargetDeviceId,
      }).toString(),
    );
  }

  Future<void> _openWebDetails() async {
    final target = AppCoreScope.webConsoleUrl;
    if (target == null || target.isEmpty) {
      return;
    }
    await _openExternalUrl(target);
  }

  Future<void> _logoutFromClient() async {
    await AuthCallbackService.clearPendingServerCallback();
    await AppCoreScope.sessionController.signOut();
  }

  Future<void> _enableActiveNetworkWithPrompt(
    NetworkMemberModel? currentMember,
  ) async {
    if (_memberIsDisabled(currentMember)) {
      await _showManagedDeviceDisabledDialog();
      return;
    }
    await AppCoreScope.sessionController.enableActiveNetwork();
    if (!mounted) {
      return;
    }
    if (_isDeviceUnavailableError(AppCoreScope.sessionStore.error)) {
      await _showManagedDeviceDisabledDialog();
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
              onPressed: () {
                AppCoreScope.configureHost(host: controller.text);
                Navigator.of(dialogContext).pop();
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
    await DesktopUrlLauncher.open(url);
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
