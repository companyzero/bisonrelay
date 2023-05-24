import 'package:bruig/components/snackbars.dart';
import 'package:bruig/models/client.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';

class ContactsLastMsgTimesScreen extends StatefulWidget {
  static const routeName = "contactsLastMsgTimes";
  final ClientModel client;
  const ContactsLastMsgTimesScreen(this.client, {super.key});

  @override
  State<ContactsLastMsgTimesScreen> createState() =>
      _ContactsLastMsgTimesScreenState();
}

class _UserLastMsgTime extends StatelessWidget {
  final LastUserReceivedTime info;
  final ChatModel chat;
  const _UserLastMsgTime(this.info, this.chat);

  void requestRatchetReset(BuildContext context) async {
    chat.requestKXReset();
    showSuccessSnackbar(context, "Attempting to reset ratchet with user");
  }

  @override
  Widget build(BuildContext context) {
    var textColor = const Color(0xFF8E8D98);
    return Container(
      margin: const EdgeInsets.only(top: 5),
      child: Row(children: [
        Flexible(
            flex: 1,
            child: Text(
              DateTime.fromMillisecondsSinceEpoch(info.lastDecrypted * 1000)
                  .toIso8601String(),
              style: TextStyle(color: textColor),
              overflow: TextOverflow.ellipsis,
            )),
        const SizedBox(width: 10),
        Text(
          chat.nick,
          style: TextStyle(color: textColor),
        ),
        const SizedBox(width: 10),
        Expanded(
            flex: 3,
            child: Text(
              info.uid,
              style: TextStyle(color: textColor),
              overflow: TextOverflow.ellipsis,
            )),
        IconButton(
            onPressed: () => requestRatchetReset(context),
            tooltip: "Request a Ratchet Reset",
            icon: Icon(
              Icons.restore,
              color: textColor,
            ))
      ]),
    );
  }
}

class _ContactsLastMsgTimesScreenState
    extends State<ContactsLastMsgTimesScreen> {
  ClientModel get client => widget.client;
  List<LastUserReceivedTime> users = [];

  void loadList() async {
    try {
      var newList = await Golib.listUsersLastMsgTimes();
      setState(() {
        users = newList;
      });
    } catch (exception) {
      showErrorSnackbar(context,
          "Unable to fetch list of contact's last msg time: $exception");
    }
  }

  @override
  void initState() {
    super.initState();
    loadList();
  }

  void onDone() {
    Navigator.pop(context);
  }

  @override
  Widget build(BuildContext context) {
    var textColor = const Color(0xFF8E8D98);
    return Scaffold(
      body: Center(
          child: Container(
              padding: const EdgeInsets.all(40),
              child: Column(
                children: [
                  Text(
                    "Last Message Time",
                    style: TextStyle(color: textColor, fontSize: 20),
                  ),
                  Expanded(
                      child: ListView.builder(
                          itemCount: users.length,
                          itemBuilder: (context, index) => _UserLastMsgTime(
                              users[index],
                              client.getExistingChat(users[index].uid)!))),
                  ElevatedButton(onPressed: onDone, child: const Text("Done"))
                ],
              ))),
    );
  }
}
