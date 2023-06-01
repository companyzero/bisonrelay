import 'package:bruig/models/client.dart';
import 'package:flutter/material.dart';
import 'package:bruig/util.dart';
import 'package:bruig/components/interactive_avatar.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:file_picker/file_picker.dart';

class _AddressBookListingW extends StatefulWidget {
  final ChatModel chat;
  final ClientModel client;

  const _AddressBookListingW(this.chat, this.client, {Key? key})
      : super(key: key);

  @override
  State<_AddressBookListingW> createState() => _AddressBookListingWState();
}

class _AddressBookListingWState extends State<_AddressBookListingW> {
  ChatModel get chat => widget.chat;
  ClientModel get client => widget.client;

  void chatUpdated() => setState(() {});

  void startChat() {
    client.startChat(chat);
  }

  @override
  void initState() {
    super.initState();
    chat.addListener(chatUpdated);
  }

  @override
  void didUpdateWidget(_AddressBookListingW oldWidget) {
    super.didUpdateWidget(oldWidget);
    oldWidget.chat.removeListener(chatUpdated);
    chat.addListener(chatUpdated);
  }

  @override
  void dispose() {
    chat.removeListener(chatUpdated);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.dividerColor;
    var hightLightTextColor = theme.focusColor;
    var selectedBackgroundColor = theme.highlightColor;
    var darkTextColor = theme.indicatorColor;

    var avatarColor = colorFromNick(chat.nick);
    var avatarTextColor =
        ThemeData.estimateBrightnessForColor(avatarColor) == Brightness.dark
            ? hightLightTextColor
            : darkTextColor;
    var popMenuButton = InteractiveAvatar(
        bgColor: selectedBackgroundColor,
        chatNick: chat.nick,
        onTap: () {},
        avatarColor: avatarColor,
        avatarTextColor: avatarTextColor);

    return Container(
      decoration: BoxDecoration(
        color: chat.active ? selectedBackgroundColor : null,
        borderRadius: BorderRadius.circular(3),
      ),
      child: ListTile(
        enabled: true,
        title:
            Text(chat.nick, style: TextStyle(fontSize: 11, color: textColor)),
        leading: popMenuButton,
        selected: chat.active,
        trailing: Material(
            color: textColor.withOpacity(0),
            child: IconButton(
                splashRadius: 15,
                iconSize: 15,
                hoverColor: selectedBackgroundColor,
                tooltip: "Start Chat",
                onPressed: () => startChat(),
                icon: Icon(color: darkTextColor, Icons.add))),
        onTap: () => {},
        selectedColor: selectedBackgroundColor,
      ),
    );
  }
}

Future<void> loadInvite(BuildContext context) async {
  // Decode the invite and send to the user verification screen.
  var filePickRes = await FilePicker.platform.pickFiles();
  if (filePickRes == null) return;
  var filePath = filePickRes.files.first.path;
  if (filePath == null) return;
  filePath = filePath.trim();
  if (filePath == "") return;
  var invite = await Golib.decodeInvite(filePath);
  Navigator.of(context, rootNavigator: true)
      .pushNamed('/verifyInvite', arguments: invite);
}

class AddressBook extends StatefulWidget {
  final ClientModel client;
  const AddressBook(this.client, {Key? key}) : super(key: key);

  @override
  State<AddressBook> createState() => _AddressBookState();
}

class _AddressBookState extends State<AddressBook> {
  ClientModel get client => widget.client;

  @override
  void initState() {
    super.initState();
  }

  void hideAddressBook() async {
    client.hideAddressBookScreen();
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var textColor = theme.focusColor;
    var darkTextColor = theme.indicatorColor;
    var dividerColor = theme.highlightColor;
    return Stack(children: [
      Container(
          padding: const EdgeInsets.all(50),
          child:
              Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
            client.hiddenGCs.isNotEmpty
                ? Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                        Row(children: [
                          Text("Available Group Chats",
                              textAlign: TextAlign.left,
                              style: TextStyle(
                                  color: darkTextColor, fontSize: 15)),
                          Expanded(
                              child: Divider(
                            color: dividerColor, //color of divider
                            height: 10, //height spacing of divider
                            thickness: 1, //thickness of divier line
                            indent: 8, //spacing at the start of divider
                            endIndent: 5, //spacing at the end of divider
                          )),
                        ]),
                        const SizedBox(height: 21),
                        Expanded(
                            child: ListView.builder(
                                itemCount: client.hiddenGCs.length,
                                itemBuilder: (context, index) =>
                                    _AddressBookListingW(
                                        client.hiddenGCs[index], client))),
                        const SizedBox(height: 21),
                      ])
                : Empty(),
            Row(children: [
              Text("Available Users",
                  textAlign: TextAlign.left,
                  style: TextStyle(color: darkTextColor, fontSize: 15)),
              Expanded(
                  child: Divider(
                color: dividerColor, //color of divider
                height: 10, //height spacing of divider
                thickness: 1, //thickness of divier line
                indent: 8, //spacing at the start of divider
                endIndent: 5, //spacing at the end of divider
              )),
            ]),
            const SizedBox(height: 21),
            Expanded(
                child: ListView.builder(
              itemCount: client.hiddenUsers.length,
              itemBuilder: (context, index) =>
                  _AddressBookListingW(client.hiddenUsers[index], client),
            )),
          ])),
      Positioned(
          top: 0,
          right: 0,
          child: Material(
              color: dividerColor.withOpacity(0),
              child: IconButton(
                  splashRadius: 15,
                  iconSize: 15,
                  hoverColor: dividerColor,
                  tooltip: "Close AddressBook",
                  onPressed: () => hideAddressBook(),
                  icon: Icon(color: darkTextColor, Icons.cancel)))),
      Positioned(
          top: 0,
          left: 0,
          child: Material(
              color: dividerColor.withOpacity(0),
              child: IconButton(
                  hoverColor: dividerColor,
                  splashRadius: 15,
                  iconSize: 15,
                  tooltip: client.isOnline
                      ? "Load Invite"
                      : "Cannot load invite while client is offline",
                  onPressed: () =>
                      client.isOnline ? () => loadInvite(context) : null,
                  icon: Icon(size: 15, color: darkTextColor, Icons.add))))
    ]);
  }
}
