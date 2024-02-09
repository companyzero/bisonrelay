import 'package:flutter/material.dart';

typedef PopMenuList = List<PopupMenuItem?>;

class ContextMenu extends StatefulWidget {
  const ContextMenu(
      {super.key,
      required this.child,
      required this.items,
      required this.handleItemTap,
      this.disabled,
      this.mobile,
      this.pageContextMenu});

  final bool? disabled;
  final Widget child;
  final PopMenuList items;
  final void Function(dynamic) handleItemTap;
  final void Function(BuildContext)? mobile;
  final bool? pageContextMenu;

  @override
  State<ContextMenu> createState() => _ContextMenuState();
}

class _ContextMenuState extends State<ContextMenu> {
  void _showContextMenu(BuildContext context) async {
    final RenderBox referenceBox = context.findRenderObject() as RenderBox;
    final RenderObject overlay =
        Overlay.of(context).context.findRenderObject() as RenderBox;
    Offset offs = const Offset(10, 30);
    final List<PopupMenuItem> items =
        widget.items.whereType<PopupMenuItem>().toList();
    final result = await showMenu(
      shadowColor: Theme.of(context).backgroundColor,
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
      items: items,
    );

    widget.handleItemTap(result);
  }

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: () => widget.pageContextMenu ?? _showContextMenu(context),
      onLongPress: () => widget.disabled == true
          ? null
          : widget.mobile != null
              ? widget.mobile!(context)
              : _showContextMenu(context),
      onSecondaryTap: () =>
          widget.disabled == true ? null : _showContextMenu(context),
      child: widget.pageContextMenu == true
          ? IconButton(
              iconSize: 20,
              splashRadius: 20,
              onPressed: () => _showContextMenu(context),
              icon: Icon(Icons.keyboard_arrow_left_rounded,
                  color: Theme.of(context).focusColor))
          : widget.child,
    );
  }
}
