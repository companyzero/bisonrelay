import 'package:bruig/theme_manager.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

class TextDialog extends StatelessWidget {
  final String text;

  const TextDialog(this.text, {super.key});

  @override
  Widget build(BuildContext context) {
    var theme = Provider.of<ThemeNotifier>(context, listen: false);
    return Dialog(
      child: Container(
        margin: const EdgeInsets.all(10),
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(10),
          controller: ScrollController(keepScrollOffset: false),
          child:
              SelectionArea(child: Text(text, style: theme.mdStyleSheet.code)),
        ),
      ),
    );
  }
}
