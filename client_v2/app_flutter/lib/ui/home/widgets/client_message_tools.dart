import 'dart:async';

import 'package:flutter/material.dart';

/// 客户端点对点 Ping 工具。
///
/// 实际传输复用客户端消息通道，页面层负责构造 Ping/Pong 协议内容和 RTT 计算。
class ClientPingTool extends StatelessWidget {
  const ClientPingTool({
    required this.targetController,
    required this.syncing,
    required this.resultText,
    required this.onPing,
    super.key,
  });

  /// 目标设备 ID 输入框控制器。
  final TextEditingController targetController;

  /// 是否正在同步或检测中。
  final bool syncing;

  /// 最近一次检测结果。
  final String? resultText;

  /// 点击 Ping 或回车时触发。
  final Future<void> Function() onPing;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Container(
      padding: const EdgeInsets.fromLTRB(12, 10, 12, 12),
      decoration: BoxDecoration(
        color: theme.colorScheme.surfaceContainerLowest,
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: theme.colorScheme.outlineVariant),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          Row(
            children: [
              Icon(
                Icons.network_ping_rounded,
                size: 18,
                color: theme.colorScheme.primary,
              ),
              const SizedBox(width: 8),
              Text(
                'Ping 工具',
                style: theme.textTheme.labelLarge?.copyWith(
                  fontWeight: FontWeight.w900,
                ),
              ),
            ],
          ),
          const SizedBox(height: 9),
          TextField(
            key: const Key('client-ping-target'),
            controller: targetController,
            enabled: !syncing,
            textInputAction: TextInputAction.send,
            onSubmitted: (_) {
              if (!syncing) {
                unawaited(onPing());
              }
            },
            decoration: const InputDecoration(
              labelText: '目标设备 ID',
              border: OutlineInputBorder(),
              isDense: true,
            ),
          ),
          const SizedBox(height: 8),
          Row(
            children: [
              Expanded(
                child: Text(
                  resultText?.trim().isNotEmpty == true
                      ? resultText!.trim()
                      : '输入目标设备 ID 后检测连通性',
                  key: const Key('client-ping-result'),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: theme.textTheme.labelMedium?.copyWith(
                    color: theme.colorScheme.onSurfaceVariant,
                    fontWeight: FontWeight.w800,
                  ),
                ),
              ),
              const SizedBox(width: 8),
              SizedBox(
                height: 42,
                child: FilledButton.icon(
                  key: const Key('client-ping-send'),
                  onPressed: syncing ? null : () => unawaited(onPing()),
                  icon: const Icon(Icons.network_ping_rounded, size: 17),
                  label: Text(syncing ? '检测中' : 'Ping'),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

/// 客户端设备消息发送组件。
class ClientMessageComposer extends StatelessWidget {
  const ClientMessageComposer({
    required this.targetController,
    required this.bodyController,
    required this.syncing,
    required this.resultText,
    required this.onSend,
    super.key,
  });

  /// 目标设备 ID 输入框控制器。
  final TextEditingController targetController;

  /// 消息正文输入框控制器。
  final TextEditingController bodyController;

  /// 是否正在发送或桥接层同步中。
  final bool syncing;

  /// 最近一次发送结果。
  final String? resultText;

  /// 点击发送或回车时触发。
  final Future<void> Function() onSend;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Container(
      padding: const EdgeInsets.fromLTRB(12, 10, 12, 12),
      decoration: BoxDecoration(
        color: theme.colorScheme.surfaceContainerLowest,
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: theme.colorScheme.outlineVariant),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          Row(
            children: [
              Icon(
                Icons.send_to_mobile_rounded,
                size: 18,
                color: theme.colorScheme.primary,
              ),
              const SizedBox(width: 8),
              Text(
                '设备消息',
                style: theme.textTheme.labelLarge?.copyWith(
                  fontWeight: FontWeight.w900,
                ),
              ),
            ],
          ),
          const SizedBox(height: 9),
          TextField(
            key: const Key('client-message-target'),
            controller: targetController,
            enabled: !syncing,
            textInputAction: TextInputAction.next,
            decoration: const InputDecoration(
              labelText: '目标设备 ID',
              border: OutlineInputBorder(),
              isDense: true,
            ),
          ),
          const SizedBox(height: 8),
          TextField(
            key: const Key('client-message-body'),
            controller: bodyController,
            enabled: !syncing,
            textInputAction: TextInputAction.send,
            onSubmitted: (_) {
              if (!syncing) {
                unawaited(onSend());
              }
            },
            decoration: const InputDecoration(
              labelText: '消息',
              border: OutlineInputBorder(),
              isDense: true,
            ),
          ),
          const SizedBox(height: 8),
          Row(
            children: [
              Expanded(
                child: Text(
                  resultText?.trim().isNotEmpty == true ? resultText! : '',
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: theme.textTheme.labelMedium?.copyWith(
                    color: theme.colorScheme.onSurfaceVariant,
                    fontWeight: FontWeight.w800,
                  ),
                ),
              ),
              const SizedBox(width: 8),
              SizedBox(
                height: 42,
                child: FilledButton.icon(
                  key: const Key('client-message-send'),
                  onPressed: syncing ? null : () => unawaited(onSend()),
                  icon: const Icon(Icons.send_rounded, size: 17),
                  label: Text(syncing ? '发送中' : '发送'),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
}
