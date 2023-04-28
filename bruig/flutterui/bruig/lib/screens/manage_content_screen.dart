import 'package:bruig/screens/manage_content/manage_content.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:bruig/models/downloads.dart';
import 'package:bruig/screens/manage_content/downloads.dart';
import 'package:bruig/components/manage_bar.dart';
import 'package:bruig/models/snackbar.dart';

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
  final SnackBarModel snackBar;
  const ManageContentScreen(this.snackBar, {Key? key}) : super(key: key);

  @override
  State<ManageContentScreen> createState() => _ManageContentScreenState();
}

class _ManageContentScreenState extends State<ManageContentScreen> {
  SnackBarModel get snackBar => widget.snackBar;
  int tabIndex = 0;

  Widget activeTab() {
    switch (tabIndex) {
      case 0:
        return ManageContent(0, snackBar);
      case 1:
        return ManageContent(1, snackBar);
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
