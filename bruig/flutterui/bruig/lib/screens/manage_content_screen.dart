import 'package:bruig/screens/manage_content/manage_content.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:bruig/models/downloads.dart';
import 'package:bruig/screens/manage_content/downloads.dart';
import 'package:bruig/components/manage_bar.dart';

/*

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var backgroundColor = theme.backgroundColor;
    return Consumer<ThemeNotifier>(
      builder: (context, theme, _) => Container(
        margin: const EdgeInsets.all(1),
        decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(3), color: backgroundColor),
        padding: const EdgeInsets.all(16),
        child: Column(
          children: [
            Row(children: [
              const Expanded(
                child: Text("News Feed",
                    style: TextStyle(
                      fontSize: 20,
                    )),
              ),
              ElevatedButton(
                  onPressed: () {
                    Navigator.of(context, rootNavigator: true)
                        .pushNamed('/newPost');
                  },
                  child: const Text("New Post")),
              const SizedBox(width: 20)
            ]),
            const SizedBox(height: 20),
            Expanded(
                child: 
            )),
            const SizedBox(height: 20),
          ],
        ),
      ),
    );
  }
}

*/

class ManageContentScreenTitle extends StatelessWidget {
  const ManageContentScreenTitle({super.key});

  @override
  Widget build(BuildContext context) {
    return Text("Bison Relay / Manage Content",
        style: TextStyle(fontSize: 15, color: Theme.of(context).focusColor));
  }
}

class ManageContentScreen extends StatefulWidget {
  static const routeName = '/manageContent';
  const ManageContentScreen({Key? key}) : super(key: key);

  @override
  State<ManageContentScreen> createState() => _ManageContentScreenState();
}

class _ManageContentScreenState extends State<ManageContentScreen> {
  int tabIndex = 0;

  Widget activeTab() {
    switch (tabIndex) {
      case 0:
        return const ManageContent(0);
      case 1:
        return const ManageContent(1);
      case 2:
        return Consumer<DownloadsModel>(
            builder: (context, downloads, child) => DownloadsScreen(downloads));
    }
    return Text("Active is $tabIndex");
  }

  void onItemChanged(int index) {
    setState(() => tabIndex = index);
  }

  @override
  void initState() {
    super.initState();
  }

  @override
  void didUpdateWidget(ManageContentScreen oldWidget) {
    super.didUpdateWidget(oldWidget);
  }

  @override
  void dispose() {
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Row(children: [
      ManageContentBar(onItemChanged, tabIndex),
      Expanded(child: activeTab())
    ]);
  }
}
