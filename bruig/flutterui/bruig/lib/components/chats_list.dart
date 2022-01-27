import 'package:bruig/models/client.dart';
import 'package:bruig/models/menus.dart';
import 'package:bruig/screens/contacts_msg_times.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:bruig/screens/feed/feed_posts.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:file_picker/file_picker.dart';

typedef MakeActiveCB = void Function(ChatModel? c);
typedef ShowSubMenuCB = void Function(String);

class _ChatHeadingW extends StatefulWidget {
  final ChatModel chat;
  final MakeActiveCB makeActive;
  final ShowSubMenuCB showSubMenu;

  const _ChatHeadingW(this.chat, this.makeActive, this.showSubMenu, {Key? key})
      : super(key: key);

  @override
  State<_ChatHeadingW> createState() => _ChatHeadingWState();
}

class _ChatHeadingWState extends State<_ChatHeadingW> {
  ChatModel get chat => widget.chat;

  void chatUpdated() => setState(() {});

  @override
  void initState() {
    super.initState();
    chat.addListener(chatUpdated);
  }

  @override
  void didUpdateWidget(_ChatHeadingW oldWidget) {
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
    var unreadMessageIconColor = theme.indicatorColor;
    var darkTextColor = theme.indicatorColor;

    Widget? trailing;
    if (chat.active) {
      // Do we want to do any text color changes on active?
    } else if (chat.unreadMsgCount > 0) {
      textColor = hightLightTextColor;
      trailing = Container(
          margin: const EdgeInsets.all(1),
          child: CircleAvatar(
              backgroundColor: unreadMessageIconColor,
              radius: 10,
              child: Text("${chat.unreadMsgCount}",
                  style: TextStyle(color: hightLightTextColor, fontSize: 10))));
    } else if (chat.unreadEventCount > 0) {
      textColor = hightLightTextColor;
      trailing = Container(
          margin: const EdgeInsets.all(1),
          child:
              CircleAvatar(backgroundColor: unreadMessageIconColor, radius: 3));
    }

    var avatarColor = colorFromNick(chat.nick);
    var avatarTextColor =
        ThemeData.estimateBrightnessForColor(avatarColor) == Brightness.dark
            ? hightLightTextColor
            : darkTextColor;
    var popMenuButton = Material(
        color: selectedBackgroundColor.withOpacity(0),
        child: IconButton(
          splashRadius: 20,
          hoverColor: selectedBackgroundColor,
          icon: CircleAvatar(
              backgroundColor: avatarColor,
              child: Text(chat.nick[0].toUpperCase(),
                  style: TextStyle(color: avatarTextColor, fontSize: 20))),
          padding: const EdgeInsets.all(0),
          tooltip: chat.nick,
          onPressed: () {
            widget.makeActive(chat);
            widget.showSubMenu(chat.id);
          },
        ));

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
          trailing: trailing,
          onTap: () => widget.makeActive(chat),
          selected: chat.active,
          selectedColor: selectedBackgroundColor,
        ));
  }
}

Future<void> generateInvite(BuildContext context) async {
  var filePath = await FilePicker.platform.saveFile(
    dialogTitle: "Select invitation file location",
    fileName: "invite.bin",
  );
  if (filePath == null) return;
  try {
    await Golib.generateInvite(filePath);
    showSuccessSnackbar(context, "Generated invitation at $filePath");
  } catch (exception) {
    showErrorSnackbar(context, "Unable to generate invitation: $exception");
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

void gotoContactsLastMsgTimeScreen(BuildContext context) {
  Navigator.of(context, rootNavigator: true)
      .pushNamed(ContactsLastMsgTimesScreen.routeName);
}

class _ChatsList extends StatefulWidget {
  final ClientModel chats;
  final FocusNode editLineFocusNode;
  const _ChatsList(this.chats, this.editLineFocusNode, {Key? key})
      : super(key: key);

  @override
  State<_ChatsList> createState() => _ChatsListState();
}

class _ChatsListState extends State<_ChatsList> {
  ClientModel get chats => widget.chats;
  FocusNode get editLineFocusNode => widget.editLineFocusNode;

  void chatsUpdated() => setState(() {});

  @override
  void initState() {
    super.initState();
    chats.addListener(chatsUpdated);
  }

  @override
  void didUpdateWidget(_ChatsList oldWidget) {
    super.didUpdateWidget(oldWidget);
    oldWidget.chats.removeListener(chatsUpdated);
    chats.addListener(chatsUpdated);
  }

  @override
  void dispose() {
    chats.removeListener(chatsUpdated);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    void createGC() async {
      Navigator.of(context, rootNavigator: true).pushNamed('/newGC');
    }

    void genInvite() async {
      await generateInvite(context);
      editLineFocusNode.requestFocus();
    }

    void closeMenus(ClientModel client) {
      editLineFocusNode.requestFocus();
    }

    var theme = Theme.of(context);
    var sidebarBackground = theme.backgroundColor;
    var hoverColor = theme.hoverColor;
    var darkTextColor = theme.indicatorColor;
    var selectedBackgroundColor = theme.highlightColor;

    var gcList = chats.gcChats.toList();
    var chatList = chats.userChats.toList();

    makeActive(ChatModel? c) => {chats.active = c};

    showGCSubMenu(String id) => {chats.showSubMenu(true, id)};
    showUserSubMenu(String id) => {chats.showSubMenu(false, id)};
    return Column(children: [
      Container(
          height: 209,
          margin: const EdgeInsets.all(1),
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(3),
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
          child: Stack(children: [
            Container(
                padding: const EdgeInsets.only(bottom: 40),
                child: ListView.builder(
                    itemCount: gcList.length,
                    itemBuilder: (context, index) => _ChatHeadingW(
                        gcList[index], makeActive, showGCSubMenu))),
            Positioned(
                bottom: 5,
                right: 5,
                child: Material(
                    color: selectedBackgroundColor.withOpacity(0),
                    child: IconButton(
                        splashRadius: 15,
                        iconSize: 15,
                        hoverColor: selectedBackgroundColor,
                        tooltip: "Add GC",
                        onPressed: () => createGC(),
                        icon: Icon(color: darkTextColor, Icons.add))))
          ])),
      Expanded(
          child: Container(
        margin: const EdgeInsets.all(1),
        decoration: BoxDecoration(
          borderRadius: BorderRadius.circular(3),
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
        child: Stack(children: [
          Container(
              padding: const EdgeInsets.only(bottom: 40),
              child: ListView.builder(
                  itemCount: chatList.length,
                  itemBuilder: (context, index) => _ChatHeadingW(
                      chatList[index], makeActive, showUserSubMenu))),
          Positioned(
              bottom: 5,
              right: 5,
              child: Material(
                  color: selectedBackgroundColor.withOpacity(0),
                  child: IconButton(
                      hoverColor: selectedBackgroundColor,
                      splashRadius: 15,
                      iconSize: 15,
                      tooltip: "Load Invite",
                      onPressed: () => loadInvite(context),
                      icon: Icon(size: 15, color: darkTextColor, Icons.add)))),
          Positioned(
              bottom: 5,
              left: 30,
              child: Material(
                  color: selectedBackgroundColor.withOpacity(0),
                  child: IconButton(
                      hoverColor: selectedBackgroundColor,
                      splashRadius: 15,
                      iconSize: 15,
                      tooltip: "List last received message time",
                      onPressed: () => gotoContactsLastMsgTimeScreen(context),
                      icon: Icon(
                          size: 15,
                          color: darkTextColor,
                          Icons.list_rounded)))),
          Positioned(
              bottom: 5,
              left: 5,
              child: Material(
                  color: selectedBackgroundColor.withOpacity(0),
                  child: IconButton(
                      hoverColor: selectedBackgroundColor,
                      splashRadius: 15,
                      iconSize: 15,
                      tooltip: "Generate Invite",
                      onPressed: () => genInvite(),
                      icon:
                          Icon(size: 15, color: darkTextColor, Icons.people))))
        ]),
      ))
    ]);
  }
}

class ChatDrawerMenu extends StatelessWidget {
  final FocusNode editLineFocusNode;
  const ChatDrawerMenu(this.editLineFocusNode, {Key? key}) : super(key: key);

  @override
  Widget build(BuildContext context) {
    return Consumer<ClientModel>(builder: (context, chats, child) {
      return Column(
          children: [Expanded(child: _ChatsList(chats, editLineFocusNode))]);
    });
  }
}
