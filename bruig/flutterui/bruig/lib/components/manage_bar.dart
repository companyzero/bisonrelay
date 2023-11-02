import 'package:flutter/material.dart';
import 'package:bruig/theme_manager.dart';
import 'package:provider/provider.dart';

class ManageContentBar extends StatefulWidget {
  final int selectedIndex;
  final Function tabChange;
  const ManageContentBar(this.tabChange, this.selectedIndex, {Key? key})
      : super(key: key);

  @override
  State<ManageContentBar> createState() => _ManageContentBarState();
}

class _ManageContentBarState extends State<ManageContentBar> {
  int get selectedIndex => widget.selectedIndex;
  Function get tabChange => widget.tabChange;
  @override
  void initState() {
    super.initState();
  }

  @override
  void didUpdateWidget(ManageContentBar oldWidget) {
    super.didUpdateWidget(oldWidget);
  }

  @override
  void dispose() {
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var unselectedTextColor = theme.dividerColor;
    var selectedTextColor = theme.focusColor; // MESSAGE TEXT COLOR
    var sidebarBackground = theme.backgroundColor;
    var hoverColor = theme.hoverColor;
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => Container(
            margin: const EdgeInsets.all(1),
            width: 118,
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(5),
              gradient: LinearGradient(
                  begin: Alignment.centerRight,
                  end: Alignment.centerLeft,
                  colors: [
                    hoverColor,
                    sidebarBackground,
                    sidebarBackground,
                  ],
                  stops: const [
                    0,
                    0.51,
                    1
                  ]),
            ),
            //color: theme.colorScheme.secondary,
            child: ListView(children: [
              ListTile(
                title: Text("Add",
                    style: TextStyle(
                        color: selectedIndex == 0
                            ? selectedTextColor
                            : unselectedTextColor,
                        fontSize: theme.getSmallFont(),
                        fontWeight: FontWeight.w400)),
                onTap: () {
                  tabChange(0);
                },
              ),
              ListTile(
                title: Text("Shared",
                    style: TextStyle(
                        color: selectedIndex == 1
                            ? selectedTextColor
                            : unselectedTextColor,
                        fontSize: theme.getSmallFont(),
                        fontWeight: FontWeight.w400)),
                onTap: () {
                  tabChange(1);
                },
              ),
              ListTile(
                title: Text("Downloads",
                    style: TextStyle(
                        color: selectedIndex == 2
                            ? selectedTextColor
                            : unselectedTextColor,
                        fontSize: theme.getSmallFont(),
                        fontWeight: FontWeight.w400)),
                onTap: () {
                  tabChange(2);
                },
              ),
            ])));
  }
}
