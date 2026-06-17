import 'package:client_core_plugin/client_core_plugin.dart';
import 'package:flutter/material.dart';

import '../../../bridge/android_network_authorization.dart';

/// Android VPN 授权与网络配置提示面板。
///
/// 仅 Android 平台展示，用于提示用户授权 VpnService 或刷新当前网络配置。
class AndroidAuthorizationPanel extends StatelessWidget {
  const AndroidAuthorizationPanel({
    required this.authorization,
    required this.onRefresh,
    super.key,
  });

  /// Android 授权和网络配置状态。
  final AndroidNetworkAuthorizationState authorization;

  /// 刷新授权/配置或重新触发授权请求。
  final VoidCallback onRefresh;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final config = authorization.networkConfig;
    final error = authorization.error?.trim();
    final status = _statusText();
    final statusColor = error != null && error.isNotEmpty
        ? theme.colorScheme.error
        : authorization.granted
            ? theme.colorScheme.primary
            : theme.colorScheme.onSurfaceVariant;
    return Container(
      padding: const EdgeInsets.fromLTRB(12, 9, 12, 9),
      decoration: BoxDecoration(
        color: theme.colorScheme.surfaceContainerLowest,
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: theme.colorScheme.outlineVariant),
      ),
      child: Row(
        children: [
          Icon(
            authorization.granted
                ? Icons.verified_user_outlined
                : Icons.vpn_key_outlined,
            size: 18,
            color: statusColor,
          ),
          const SizedBox(width: 8),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  status,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: theme.textTheme.labelMedium?.copyWith(
                    color: statusColor,
                    fontWeight: FontWeight.w900,
                  ),
                ),
                if (config != null || (error != null && error.isNotEmpty)) ...[
                  const SizedBox(height: 2),
                  Text(
                    error != null && error.isNotEmpty
                        ? _compactError(error)
                        : _configText(config!),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: theme.textTheme.labelSmall?.copyWith(
                      color: theme.colorScheme.onSurfaceVariant,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                ],
              ],
            ),
          ),
          if (authorization.checking)
            SizedBox(
              width: 18,
              height: 18,
              child: CircularProgressIndicator(
                strokeWidth: 2,
                color: theme.colorScheme.primary,
              ),
            )
          else if (authorization.needsUserConsent)
            SizedBox(
              height: 30,
              child: FilledButton.icon(
                onPressed: onRefresh,
                icon: const Icon(Icons.check_circle_outline, size: 16),
                label: const Text('授权'),
              ),
            )
          else
            IconButton(
              tooltip: '刷新',
              onPressed: onRefresh,
              icon: const Icon(Icons.refresh_rounded, size: 18),
            ),
        ],
      ),
    );
  }

  /// 根据授权、检查中、错误和配置状态生成短状态文案。
  String _statusText() {
    final error = authorization.error?.trim();
    if (error != null && error.isNotEmpty) {
      return 'Android 网络配置异常';
    }
    if (authorization.checking) {
      return 'Android 网络检查中';
    }
    if (authorization.needsUserConsent) {
      return 'Android 网络待授权';
    }
    if (authorization.granted && authorization.networkConfig != null) {
      return 'Android 网络配置已就绪';
    }
    if (authorization.granted) {
      return 'Android 网络已授权';
    }
    return 'Android 网络状态待确认';
  }

  /// 展示核心 VPN 配置，便于现场确认虚拟 IP 和 DNS。
  String _configText(AndroidVpnSessionConfig config) {
    final dns = config.dnsServers.isEmpty ? '-' : config.dnsServers.join(',');
    return '${config.virtualIp}/${config.prefixLen}  DNS $dns';
  }

  /// 压缩错误里的换行和多空格，避免横向卡片被撑开。
  String _compactError(String error) {
    return error.replaceAll(RegExp(r'\s+'), ' ');
  }
}
