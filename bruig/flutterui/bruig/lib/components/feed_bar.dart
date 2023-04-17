import 'package:flutter/material.dart';

class FeedBar extends StatefulWidget {
  final int selectedIndex;
  final Function tabChange;
  const FeedBar(this.tabChange, this.selectedIndex, {Key? key})
      : super(key: key);

  @override
  State<FeedBar> createState() => _FeedBarState();
}

class _FeedBarState extends State<FeedBar> {
  int get selectedIndex => widget.selectedIndex;
  Function get tabChange => widget.tabChange;
  @override
  void initState() {
    super.initState();
  }

  @override
  void didUpdateWidget(FeedBar oldWidget) {
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
    return Container(
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
      child: ListView(
        children: [
          ListTile(
            title: Text("News Feed",
                style: TextStyle(
                    color: selectedIndex == 0
                        ? selectedTextColor
                        : unselectedTextColor,
                    fontSize: 11,
                    fontWeight: FontWeight.w400)),
            onTap: () {
              tabChange(0, null);
            },
          ),
          ListTile(
            title: Text("Your Posts",
                style: TextStyle(
                    color: selectedIndex == 1
                        ? selectedTextColor
                        : unselectedTextColor,
                    fontSize: 11,
                    fontWeight: FontWeight.w400)),
            onTap: () {
              tabChange(1, null);
            },
          ),
          ListTile(
            title: Text("Subscriptions",
                style: TextStyle(
                    color: selectedIndex == 2
                        ? selectedTextColor
                        : unselectedTextColor,
                    fontSize: 11,
                    fontWeight: FontWeight.w400)),
            onTap: () {
              tabChange(2, null);
            },
          ),
          ListTile(
            title: Text("New Post",
                style: TextStyle(
                    color: selectedIndex == 3
                        ? selectedTextColor
                        : unselectedTextColor,
                    fontSize: 11,
                    fontWeight: FontWeight.w400)),
            onTap: () {
              tabChange(3, null);
            },
          ),
        ],
      ),
    );
  }
}
