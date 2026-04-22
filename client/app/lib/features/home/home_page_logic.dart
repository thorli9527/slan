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
    final statusMessage = await AppCoreScope.sessionController.ensureHomeWorkspaceReady(
      deviceName: host,
      platform: DesktopPlatform.currentId,
      machineId: AppCoreScope.clientMachineId,
      devicePublicKey: 'device-key-${DateTime.now().microsecondsSinceEpoch}',
    );
    if (!mounted) {
      return;
    }

    final activeNetwork =
        sessionStore.networks.isNotEmpty ? sessionStore.networks.first : null;
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

  Future<void> _showServerSettingsDialog(BuildContext context) async {
    final controller = TextEditingController(
      text: AppCoreScope.hostInput ?? '127.0.0.1',
    );
    await showDialog<void>(
      context: context,
      builder: (dialogContext) {
        return AlertDialog(
          title: const Text('服务器地址'),
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
                  '只需要填写一个 host，客户端会自动推导控制面和网页控制台地址。',
                  style: Theme.of(dialogContext).textTheme.bodySmall,
                ),
              ],
            ),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.of(dialogContext).pop(),
              child: const Text('取消'),
            ),
            FilledButton(
              onPressed: () {
                AppCoreScope.configureHost(host: controller.text);
                Navigator.of(dialogContext).pop();
              },
              child: const Text('保存'),
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
      'No network is assigned yet. Open the web console manually if you want to manage networks, subnets, or DHCP: $target',
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
