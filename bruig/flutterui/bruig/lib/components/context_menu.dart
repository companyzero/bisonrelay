import 'package:flutter/material.dart';

typedef PopMenuList = List<PopupMenuItem>;

class ContextMenu extends StatefulWidget {
  const ContextMenu(
      {super.key,
      required this.child,
      required this.items,
      required this.handleItemTap,
      this.disabled});

  final bool? disabled;
  final Widget child;
  final PopMenuList items;
  final void Function(dynamic) handleItemTap;

  @override
  State<ContextMenu> createState() => _ContextMenuState();
}

class _ContextMenuState extends State<ContextMenu> {
  void _showContextMenu(BuildContext context) async {
    final RenderBox referenceBox = context.findRenderObject() as RenderBox;
    final RenderObject overlay =
        Overlay.of(context).context.findRenderObject() as RenderBox;
    Offset offs = const Offset(10, 30);

    final result = await showMenu(
      context: context,
      position: RelativeRect.fromRect(
        Rect.fromPoints(
          referenceBox.localToGlobal(offs, ancestor: overlay),
          referenceBox.localToGlobal(
              referenceBox.size.bottomRight(Offset.zero) + offs,
              ancestor: overlay),
        ),
        Offset.zero & overlay.paintBounds.size,
      ),
      items: widget.items,
    );

    widget.handleItemTap(result);
  }

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      child: widget.child,
      onLongPress: () =>
          widget.disabled == true ? null : _showContextMenu(context),
      onSecondaryTap: () =>
          widget.disabled == true ? null : _showContextMenu(context),
    );
  }
}
