import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/theme_manager.dart';
import 'package:provider/provider.dart';

class Input extends StatefulWidget {
  final ClientModel client;
  final FocusNode inputFocusNode;
  const Input(this.client, this.inputFocusNode, {Key? key}) : super(key: key);

  @override
  State<Input> createState() => _InputState();
}

class _InputState extends State<Input> {
  final controller = TextEditingController();
  ClientModel get client => widget.client;

  final FocusNode node = FocusNode();

  @override
  void initState() {
    setState(() {
      controller.text = client.filteredSearchString;
    });
    super.initState();
  }

  @override
  void dispose() {
    super.dispose();
  }

  @override
  void didUpdateWidget(Input oldWidget) {
    super.didUpdateWidget(oldWidget);
    widget.inputFocusNode.requestFocus();
  }

  void handleKeyPress(event) {
    if (event is RawKeyUpEvent) {
      bool modPressed = event.isShiftPressed || event.isControlPressed;
      final val = controller.value;
      client.filteredSearchString = val.text;
      if (event.data.logicalKey.keyLabel == "Enter" && !modPressed) {
        controller.value = const TextEditingValue(
            text: "", selection: TextSelection.collapsed(offset: 0));
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.focusColor; // MESSAGE TEXT COLOR
    var hoverColor = theme.hoverColor;
    var backgroundColor = theme.highlightColor;
    var hintTextColor = theme.dividerColor;
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => RawKeyboardListener(
              focusNode: node,
              onKey: handleKeyPress,
              child: Container(
                margin: const EdgeInsets.only(bottom: 5),
                child: Row(
                  children: [
                    Expanded(
                        child: TextField(
                      autofocus: true,
                      focusNode: widget.inputFocusNode,
                      style: TextStyle(
                        fontSize: theme.getMediumFont(),
                        color: textColor,
                      ),
                      controller: controller,
                      minLines: 1,
                      maxLines: null,
                      //textInputAction: TextInputAction.done,
                      //style: normalTextStyle,
                      keyboardType: TextInputType.multiline,
                      decoration: InputDecoration(
                        filled: true,
                        fillColor: backgroundColor,
                        hoverColor: hoverColor,
                        isDense: true,
                        hintText: 'Search Address book for Room or User',
                        hintStyle: TextStyle(
                          fontSize: theme.getMediumFont(),
                          color: hintTextColor,
                        ),
                        border: InputBorder.none,
                      ),
                    )),
                  ],
                ),
              ),
            ));
  }
}
