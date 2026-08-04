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
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Row(
          crossAxisAlignment: CrossAxisAlignment.center,
          children: [
            Expanded(
              child: TextField(
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
                  suffixIcon: IconButton(
                    key: const Key('device-authorization-key-paste'),
                    tooltip: '粘贴授权 Key',
                    onPressed: syncing ? null : _pasteAuthorizationKey,
                    icon: const Icon(Icons.content_paste_rounded, size: 20),
                  ),
                ),
              ),
            ),
            if (onSettings != null) ...[
              const SizedBox(width: 8),
              IconButton.outlined(
                key: const Key('server-settings'),
                tooltip: '服务器 API 设置',
                onPressed: syncing ? null : onSettings,
                icon: const Icon(Icons.settings_rounded, size: 20),
              ),
            ],
          ],
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
    );
  }
}
