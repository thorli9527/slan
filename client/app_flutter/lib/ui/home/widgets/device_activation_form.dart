import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

class DeviceActivationForm extends StatelessWidget {
  const DeviceActivationForm({
    required this.keyController,
    required this.syncing,
    required this.onSubmit,
    this.onSettings,
    super.key,
  });

  final TextEditingController keyController;
  final bool syncing;
  final VoidCallback onSubmit;
  final VoidCallback? onSettings;

  Future<void> _pasteAuthorizationKey() async {
    final clipboardData = await Clipboard.getData(Clipboard.kTextPlain);
    final value = clipboardData?.text?.trim() ?? '';
    if (value.isEmpty) {
      return;
    }
    keyController.value = TextEditingValue(
      text: value,
      selection: TextSelection.collapsed(offset: value.length),
    );
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Container(
      padding: const EdgeInsets.fromLTRB(18, 18, 18, 18),
      decoration: BoxDecoration(
        color: Colors.white,
        borderRadius: BorderRadius.circular(18),
        border: Border.all(color: theme.colorScheme.outlineVariant),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          TextField(
            key: const Key('device-authorization-key'),
            controller: keyController,
            enabled: !syncing,
            obscureText: true,
            autocorrect: false,
            enableSuggestions: false,
            textInputAction: TextInputAction.done,
            onSubmitted: (_) => syncing ? null : onSubmit(),
            decoration: InputDecoration(
              labelText: '授权 Key',
              border: const OutlineInputBorder(),
              isDense: true,
              // 粘贴与服务器设置统一收纳进输入框尾部，避免与输入框并列造成视觉分散。
              suffixIcon: Padding(
                padding: const EdgeInsetsDirectional.only(end: 2),
                child: Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    _SuffixAction(
                      actionKey: const Key('device-authorization-key-paste'),
                      tooltip: '粘贴授权 Key',
                      icon: Icons.content_paste_rounded,
                      onPressed: syncing ? null : _pasteAuthorizationKey,
                    ),
                    if (onSettings != null)
                      _SuffixAction(
                        actionKey: const Key('server-settings'),
                        tooltip: '服务器 API 设置',
                        icon: Icons.settings_rounded,
                        onPressed: syncing ? null : onSettings,
                      ),
                  ],
                ),
              ),
            ),
          ),
          const SizedBox(height: 6),
          Text(
            '在 Opt 控制台创建设备后获取授权 Key',
            style: theme.textTheme.labelSmall?.copyWith(
              color: theme.colorScheme.onSurfaceVariant,
            ),
          ),
          const SizedBox(height: 12),
          SizedBox(
            height: 42,
            child: FilledButton(
              key: const Key('device-activate-submit'),
              onPressed: syncing ? null : onSubmit,
              child: Text(syncing ? '正在激活' : '激活设备'),
            ),
          ),
        ],
      ),
    );
  }
}

/// 输入框尾部的紧凑图标按钮，保持 36px 触控区同时避免默认内边距撑高输入框。
class _SuffixAction extends StatelessWidget {
  const _SuffixAction({
    required this.actionKey,
    required this.tooltip,
    required this.icon,
    required this.onPressed,
  });

  final Key actionKey;
  final String tooltip;
  final IconData icon;
  final VoidCallback? onPressed;

  @override
  Widget build(BuildContext context) {
    return IconButton(
      key: actionKey,
      tooltip: tooltip,
      visualDensity: VisualDensity.compact,
      constraints: const BoxConstraints.tightFor(width: 36, height: 36),
      padding: EdgeInsets.zero,
      onPressed: onPressed,
      icon: Icon(icon, size: 19),
    );
  }
}
