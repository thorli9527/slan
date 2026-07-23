import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

class NetworkInviteDialog extends StatefulWidget {
  const NetworkInviteDialog({required this.onAccept, super.key});

  final Future<void> Function(String inviteCode) onAccept;

  @override
  State<NetworkInviteDialog> createState() => _NetworkInviteDialogState();
}

class _NetworkInviteDialogState extends State<NetworkInviteDialog> {
  final _controller = TextEditingController();
  final _formKey = GlobalKey<FormState>();
  bool _submitting = false;
  String? _submitError;

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  Future<void> _paste() async {
    final data = await Clipboard.getData(Clipboard.kTextPlain);
    if (!mounted) return;
    final value = data?.text?.trim() ?? '';
    if (value.isEmpty) return;
    setState(() {
      _controller.text = value;
      _controller.selection = TextSelection.collapsed(offset: value.length);
      _submitError = null;
    });
  }

  Future<void> _submit() async {
    if (_submitting || !_formKey.currentState!.validate()) return;
    setState(() {
      _submitting = true;
      _submitError = null;
    });
    try {
      await widget.onAccept(_controller.text.trim());
      if (mounted) Navigator.of(context).pop(true);
    } catch (error) {
      if (!mounted) return;
      setState(() {
        _submitting = false;
        _submitError = error.toString().replaceFirst('Exception: ', '');
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return PopScope(
      canPop: !_submitting,
      child: AlertDialog(
        key: const Key('network-invite-dialog'),
        backgroundColor: Colors.white,
        surfaceTintColor: Colors.transparent,
        insetPadding: const EdgeInsets.symmetric(horizontal: 20, vertical: 16),
        titlePadding: const EdgeInsets.fromLTRB(20, 18, 20, 0),
        contentPadding: const EdgeInsets.fromLTRB(20, 14, 20, 6),
        actionsPadding: const EdgeInsets.fromLTRB(14, 2, 14, 12),
        title: Row(
          children: [
            CircleAvatar(
              radius: 15,
              backgroundColor: theme.colorScheme.primaryContainer,
              foregroundColor: theme.colorScheme.onPrimaryContainer,
              child: const Icon(Icons.link_rounded, size: 17),
            ),
            const SizedBox(width: 10),
            const Text(
              '确认接入',
              style: TextStyle(fontSize: 18, fontWeight: FontWeight.w700),
            ),
          ],
        ),
        content: SizedBox(
          width: 320,
          child: Form(
            key: _formKey,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                TextFormField(
                  key: const Key('network-invite-code-input'),
                  controller: _controller,
                  autofocus: true,
                  enabled: !_submitting,
                  textInputAction: TextInputAction.done,
                  textCapitalization: TextCapitalization.characters,
                  autocorrect: false,
                  enableSuggestions: false,
                  style: const TextStyle(fontSize: 14),
                  decoration: InputDecoration(
                    labelText: '接入码',
                    hintText: '请输入接入码',
                    border: const OutlineInputBorder(),
                    isDense: true,
                    contentPadding: const EdgeInsets.symmetric(
                      horizontal: 12,
                      vertical: 10,
                    ),
                    constraints: const BoxConstraints(minHeight: 40),
                    suffixIconConstraints: const BoxConstraints.tightFor(
                      width: 38,
                      height: 38,
                    ),
                    suffixIcon: IconButton(
                      key: const Key('network-invite-paste'),
                      tooltip: '粘贴',
                      onPressed: _submitting ? null : _paste,
                      padding: EdgeInsets.zero,
                      icon: const Icon(Icons.content_paste_rounded, size: 17),
                    ),
                  ),
                  validator: (value) =>
                      value == null || value.trim().isEmpty ? '请输入接入码' : null,
                  onChanged: (_) {
                    if (_submitError != null) {
                      setState(() => _submitError = null);
                    }
                  },
                  onFieldSubmitted: (_) => _submit(),
                ),
                if (_submitError != null) ...[
                  const SizedBox(height: 10),
                  Text(
                    '接入失败：$_submitError',
                    key: const Key('network-invite-error'),
                    style: theme.textTheme.bodySmall?.copyWith(
                      color: theme.colorScheme.error,
                    ),
                  ),
                ],
              ],
            ),
          ),
        ),
        actions: [
          TextButton(
            style: TextButton.styleFrom(
              minimumSize: const Size(0, 34),
              padding: const EdgeInsets.symmetric(horizontal: 12),
              textStyle: const TextStyle(fontSize: 13),
            ),
            onPressed:
                _submitting ? null : () => Navigator.of(context).pop(false),
            child: const Text('取消'),
          ),
          FilledButton(
            key: const Key('network-invite-submit'),
            style: FilledButton.styleFrom(
              minimumSize: const Size(0, 34),
              padding: const EdgeInsets.symmetric(horizontal: 14),
              textStyle: const TextStyle(fontSize: 13),
            ),
            onPressed: _submitting ? null : _submit,
            child: _submitting
                ? const SizedBox.square(
                    dimension: 15,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  )
                : const Text('确认接入'),
          ),
        ],
      ),
    );
  }
}
