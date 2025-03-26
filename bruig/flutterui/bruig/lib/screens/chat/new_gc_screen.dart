import 'package:bruig/components/text.dart';
import 'package:bruig/components/usersearch/user_search_panel.dart';
import 'package:bruig/components/usersearch/user_search_model.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/snackbar.dart';
import 'package:flutter/widgets.dart';

class NewGcScreen extends StatefulWidget {
  static const routeName = "/chat/newGC";

  final ClientModel client;
  const NewGcScreen(this.client, {super.key});

  @override
  State<NewGcScreen> createState() => _NewGcScreenState();
}

class _NewGcScreenState extends State<NewGcScreen> {
  ClientModel get client => widget.client;
  UserSelectionModel userSel = UserSelectionModel(allowMultiple: true);

  // Screen state is either selecting users or typing GC name.
  bool selectingUsers = true;

  String newGcName = "";
  bool creating = false;

  void goBack() {
    if (!selectingUsers) {
      setState(() {
        selectingUsers = true;
      });
    } else {
      Navigator.of(context).pop();
    }
  }

  void createNewGCFromList() async {
    if (newGcName == "" || creating) return;

    var snackbar = SnackBarModel.of(context);
    try {
      setState(() => creating = true);
      await client.createNewGCAndInvite(newGcName, userSel.selected);
      if (mounted) {
        Navigator.of(context).pop();
      }
      snackbar.success("Created GC $newGcName");
    } catch (exception) {
      snackbar.error("Unable to create GC: $exception");
    } finally {
      setState(() => creating = false);
    }
  }

  void confirm() {
    if (selectingUsers) {
      setState(() {
        selectingUsers = false;
      });
    } else {
      createNewGCFromList();
    }
  }

  void searchInputChanged(String value) {
    if (!selectingUsers) {
      setState(() {
        newGcName = value;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(10),
      child: Column(children: [
        if (selectingUsers)
          const Txt.L("Select GC members")
        else
          const Txt.L("Type name of new GC"),
        const SizedBox(height: 10),
        Expanded(
            child: UserSearchPanel(
          client,
          userSelModel: selectingUsers ? userSel : null,
          targets: selectingUsers
              ? UserSearchPanelTargets.users
              : UserSearchPanelTargets.gcs,
          searchInputHintText:
              selectingUsers ? "Search for users" : "Name of new GC",
          confirmLabel:
              selectingUsers ? "Confirm Users" : "Confirm GC Creation",
          onCancel: goBack,
          onConfirm:
              (selectingUsers || newGcName != "") && !creating ? confirm : null,
          onSearchInputChanged: searchInputChanged,
        ))
      ]),
    );
  }
}
