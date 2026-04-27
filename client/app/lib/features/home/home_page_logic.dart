part of 'home_page.dart';

extension _HomePageLogic on _HomePageState {
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
      _setNetworkConsoleStatus(statusMessage);
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
    final currentDeviceId = AppCoreScope.sessionStore.device?.deviceId?.trim();
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

  Future<void> _logoutFromClient({required bool hasActiveNetwork}) async {
    final sessionController = AppCoreScope.sessionController;
    if (hasActiveNetwork) {
      try {
        await sessionController.disableActiveNetwork();
      } catch (_) {
        // Best effort. Still continue with local sign out.
      }
    }
    await sessionController.signOut();
  }

  String _formatLoginTime(int? authenticatedAtMs) {
    if (authenticatedAtMs == null || authenticatedAtMs <= 0) {
      return 'Unknown';
    }
    final time =
        DateTime.fromMillisecondsSinceEpoch(authenticatedAtMs).toLocal();
    final two = (int value) => value.toString().padLeft(2, '0');
    return '${time.year}-${two(time.month)}-${two(time.day)} ${two(time.hour)}:${two(time.minute)}:${two(time.second)}';
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
      _setNetworkConsoleStatus(
        'No network is assigned and no web console URL is configured.',
      );
      return;
    }
    _openedNetworkConsole = false;
    _setNetworkConsoleStatus(
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
