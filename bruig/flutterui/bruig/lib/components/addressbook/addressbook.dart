import 'package:bruig/models/client.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:bruig/util.dart';
import 'package:bruig/components/interactive_avatar.dart';
import 'package:bruig/components/empty_widget.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:file_picker/file_picker.dart';
import 'package:bruig/components/addressbook/input.dart';
import 'package:bruig/theme_manager.dart';
import 'package:provider/provider.dart';

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

  void startChat(bool open) {
    client.startChat(chat, open);
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
    var alreadyOpened = false;
    if (chat.isGC) {
      alreadyOpened = client.gcChats.contains(chat);
    } else {
      alreadyOpened = client.userChats.contains(chat);
    }
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

    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => Container(
              decoration: BoxDecoration(
                borderRadius: BorderRadius.circular(3),
              ),
              child: ListTile(
                enabled: true,
                title: Text(chat.nick,
                    style: TextStyle(
                        fontSize: theme.getSmallFont(), color: textColor)),
                leading: popMenuButton,
                trailing: Material(
                    color: textColor.withOpacity(0),
                    child: IconButton(
                        splashRadius: 15,
                        iconSize: 15,
                        hoverColor: selectedBackgroundColor,
                        tooltip: alreadyOpened ? "Open Chat" : "Start Chat",
                        onPressed: () => startChat(alreadyOpened),
                        icon: Icon(
                            color: darkTextColor,
                            alreadyOpened
                                ? Icons.arrow_right_alt_outlined
                                : Icons.add))),
                onTap: () => {},
              ),
            ));
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
  final FocusNode inputFocusNode;
  const AddressBook(this.client, this.inputFocusNode, {Key? key})
      : super(key: key);

  @override
  State<AddressBook> createState() => _AddressBookState();
}

class _AddressBookState extends State<AddressBook> {
  ClientModel get client => widget.client;
  FocusNode get inputFocusNode => widget.inputFocusNode;

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
    var darkTextColor = theme.indicatorColor;
    var dividerColor = theme.highlightColor;
    var combinedGCList = client.hiddenGCs + client.gcChats;
    combinedGCList
        .sort((a, b) => a.nick.toLowerCase().compareTo(b.nick.toLowerCase()));
    var combinedUserList = client.hiddenUsers + client.userChats;
    combinedUserList
        .sort((a, b) => a.nick.toLowerCase().compareTo(b.nick.toLowerCase()));
    return Consumer<ThemeNotifier>(
        builder: (context, theme, _) => Column(children: [
              Row(children: [
                Material(
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
                        icon: Icon(size: 15, color: darkTextColor, Icons.add))),
                Expanded(child: Input(client, inputFocusNode)),
                Material(
                    color: dividerColor.withOpacity(0),
                    child: IconButton(
                        splashRadius: 15,
                        iconSize: 15,
                        hoverColor: dividerColor,
                        tooltip: "Close Address book",
                        onPressed: () => hideAddressBook(),
                        icon: Icon(color: darkTextColor, Icons.cancel))),
              ]),
              Expanded(
                  child: Container(
                      padding: const EdgeInsets.all(20),
                      child: client.filteredSearchString != ""
                          ? Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                  Row(children: [
                                    Text("Matching Chats",
                                        textAlign: TextAlign.left,
                                        style: TextStyle(
                                            color: darkTextColor,
                                            fontSize: theme.getMediumFont())),
                                    Expanded(
                                        child: Divider(
                                      color: dividerColor, //color of divider
                                      height: 10, //height spacing of divider
                                      thickness: 1, //thickness of divier line
                                      indent:
                                          8, //spacing at the start of divider
                                      endIndent:
                                          5, //spacing at the end of divider
                                    )),
                                  ]),
                                  const SizedBox(height: 21),
                                  client.filteredSearch.isNotEmpty
                                      ? Expanded(
                                          child: ListView.builder(
                                              itemCount:
                                                  client.filteredSearch.length,
                                              itemBuilder: (context, index) =>
                                                  _AddressBookListingW(
                                                      client.filteredSearch[
                                                          index],
                                                      client)))
                                      : Center(
                                          //padding: const EdgeInsets.only(left: 50),
                                          child: Text("No Matching Chats",
                                              textAlign: TextAlign.center,
                                              style: TextStyle(
                                                  color: darkTextColor,
                                                  fontSize:
                                                      theme.getMediumFont()))),
                                  const SizedBox(height: 21),
                                ])
                          : Column(children: [
                              combinedGCList.isNotEmpty
                                  ? Expanded(
                                      child: Column(
                                          crossAxisAlignment:
                                              CrossAxisAlignment.start,
                                          children: [
                                          Row(children: [
                                            Text("Available Rooms",
                                                textAlign: TextAlign.left,
                                                style: TextStyle(
                                                    color: darkTextColor,
                                                    fontSize:
                                                        theme.getMediumFont())),
                                            Expanded(
                                                child: Divider(
                                              color:
                                                  dividerColor, //color of divider
                                              height:
                                                  10, //height spacing of divider
                                              thickness:
                                                  1, //thickness of divier line
                                              indent:
                                                  8, //spacing at the start of divider
                                              endIndent:
                                                  5, //spacing at the end of divider
                                            )),
                                          ]),
                                          const SizedBox(height: 21),
                                          Expanded(
                                              child: ListView.builder(
                                                  itemCount:
                                                      combinedGCList.length,
                                                  itemBuilder: (context,
                                                          index) =>
                                                      _AddressBookListingW(
                                                          combinedGCList[index],
                                                          client))),
                                          const SizedBox(height: 21),
                                        ]))
                                  : const Empty(),
                              combinedUserList.isNotEmpty
                                  ? Expanded(
                                      child: Column(
                                          crossAxisAlignment:
                                              CrossAxisAlignment.start,
                                          children: [
                                          Row(children: [
                                            Text("Available Users",
                                                textAlign: TextAlign.left,
                                                style: TextStyle(
                                                    color: darkTextColor,
                                                    fontSize:
                                                        theme.getMediumFont())),
                                            Expanded(
                                                child: Divider(
                                              color:
                                                  dividerColor, //color of divider
                                              height:
                                                  10, //height spacing of divider
                                              thickness:
                                                  1, //thickness of divier line
                                              indent:
                                                  8, //spacing at the start of divider
                                              endIndent:
                                                  5, //spacing at the end of divider
                                            )),
                                          ]),
                                          const SizedBox(height: 21),
                                          Expanded(
                                              child: ListView.builder(
                                            itemCount: combinedUserList.length,
                                            itemBuilder: (context, index) =>
                                                _AddressBookListingW(
                                                    combinedUserList[index],
                                                    client),
                                          )),
                                        ]))
                                  : const Empty()
                            ]))),
            ]));
  }
}
