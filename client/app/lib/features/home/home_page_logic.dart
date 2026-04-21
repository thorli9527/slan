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
    final callbackId = await AuthCallbackService.preparePendingServerCallback();
    await _openExternalUrl(
      Uri.parse(target).replace(queryParameters: {
        ...Uri.parse(target).queryParameters,
        'auth': 'login',
        'deviceId': AppCoreScope.clientMachineId,
        'callbackId': callbackId,
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
    if (_openedNetworkConsole) {
      _setNetworkConsoleStatus(
        'No network is assigned to this client yet. Use the web console for network, subnet, and DHCP management.',
      );
      return;
    }

    final target = AppCoreScope.webConsoleUrl;
    if (target == null || target.isEmpty) {
      _setNetworkConsoleStatus(
        'No network is assigned and no web console URL is configured.',
      );
      return;
    }

    try {
      await _openExternalUrl(target);
      _openedNetworkConsole = true;
      _setNetworkConsoleStatus(
        'No network is assigned. The web console has been opened for network, subnet, and DHCP management.',
      );
    } catch (_) {
      _setNetworkConsoleStatus(
        'No network is assigned. Open the web console manually: $target',
      );
    }
  }

  void _stopMissingNetworkPolling() {
    _networkPollingTimer?.cancel();
    _networkPollingTimer = null;
  }

  Future<void> _openExternalUrl(String url) async {
    if (Platform.isMacOS) {
      await Process.start('open', [url]);
      return;
    }
    if (Platform.isLinux) {
      await Process.start('xdg-open', [url]);
      return;
    }
    if (Platform.isWindows) {
      await Process.start('cmd', ['/c', 'start', '', url]);
      return;
    }
    throw UnsupportedError('unsupported desktop platform');
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
