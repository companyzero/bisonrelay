import 'package:bruig/models/client.dart';
import 'package:bruig/models/menus.dart';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:bruig/screens/feed/feed_posts.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:file_picker/file_picker.dart';

typedef MakeActiveCB = void Function(ChatModel? c);
typedef ShowSubMenuCB = void Function(List<ChatMenuItem>);

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
    var textColor = theme.dividerColor; //  UNREAD COUNT TEXT COLOR
    var hightLightTextColor = theme.focusColor; // NAME TEXT COLOR
    var selectedBackgroundColor = theme.highlightColor;
    var unreadMessageIconColor = theme.shadowColor;
    var darkTextColor = const Color(0xFF5A5968);

    Widget? trailing;
    if (chat.active) {
      textColor = hightLightTextColor;
    } else if (chat.unreadCount > 0) {
      trailing = Container(
          margin: const EdgeInsets.all(1),
          child: CircleAvatar(
              backgroundColor: unreadMessageIconColor, radius: 1.5));
    }

    List<ChatMenuItem> Function(ChatModel) popupMenuBuilder;

    if (!chat.isGC) {
      popupMenuBuilder = buildUserChatMenu;
    } else {
      popupMenuBuilder = buildGCMenu;
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
            widget.showSubMenu(popupMenuBuilder(chat));
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

class ChatsList extends StatelessWidget {
  final FocusNode editLineFocusNode;
  ChatsList(this.editLineFocusNode, {super.key});

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
      client.subGCMenu = [];
      client.subUserMenu = [];
      editLineFocusNode.requestFocus();
    }

    var theme = Theme.of(context);
    var sidebarBackground = theme.backgroundColor;
    var hoverColor = theme.hoverColor;
    var darkTextColor = const Color(0xFF5A5968);
    var selectedBackgroundColor = theme.highlightColor;

    return Consumer<ClientModel>(builder: (context, chats, child) {
      var list = chats.chats;
      var gcList = list.where((x) => x.isGC).toList();
      gcList.sort((a, b) => a.unreadCount.compareTo(b.unreadCount));
      var chatList = list.where((x) => !x.isGC).toList();
      chatList.sort((a, b) => a.unreadCount.compareTo(b.unreadCount));
      makeActive(ChatModel? c) =>
          {chats.active = c, chats.subGCMenu = [], chats.subUserMenu = []};
      showGCSubMenu(List<ChatMenuItem> sm) => {chats.subGCMenu = sm};
      showUserSubMenu(List<ChatMenuItem> sm) => {chats.subUserMenu = sm};
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
            child: chats.subGCMenu.isEmpty
                ? Stack(children: [
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
                  ])
                : Stack(children: [
                    ListView.builder(
                      itemCount: chats.subGCMenu.length,
                      itemBuilder: (context, index) => ListTile(
                        title: Text(chats.subGCMenu[index].label,
                            style: const TextStyle(fontSize: 11)),
                        onTap: () {
                          chats.subGCMenu[index].onSelected(context, chats);
                          closeMenus(chats);
                        },
                      ),
                    ),
                    Positioned(
                        top: 5,
                        right: 5,
                        child: Material(
                            color: selectedBackgroundColor.withOpacity(0),
                            child: IconButton(
                                splashRadius: 15,
                                hoverColor: selectedBackgroundColor,
                                iconSize: 15,
                                onPressed: () => closeMenus(chats),
                                icon: Icon(
                                    color: darkTextColor,
                                    Icons.close_outlined)))),
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
          child: chats.subUserMenu.isEmpty
              ? Stack(children: [
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
                              icon: Icon(
                                  size: 15, color: darkTextColor, Icons.add)))),
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
                              icon: Icon(
                                  size: 15,
                                  color: darkTextColor,
                                  Icons.people))))
                ])
              : Stack(alignment: Alignment.topRight, children: [
                  ListView.builder(
                    itemCount: chats.subUserMenu.length,
                    itemBuilder: (context, index) => ListTile(
                        title: Text(chats.subUserMenu[index].label,
                            style: const TextStyle(fontSize: 11)),
                        onTap: () {
                          chats.subUserMenu[index].onSelected(context, chats);
                          closeMenus(chats);
                        }),
                  ),
                  Positioned(
                      top: 5,
                      right: 5,
                      child: Material(
                          color: selectedBackgroundColor.withOpacity(0),
                          child: IconButton(
                              hoverColor: selectedBackgroundColor,
                              splashRadius: 15,
                              iconSize: 15,
                              onPressed: () => closeMenus(chats),
                              icon: Icon(
                                  color: darkTextColor,
                                  Icons.close_outlined)))),
                ]),
        ))
      ]);
    });
  }
}

class ChatDrawerMenu extends StatelessWidget {
  final FocusNode editLineFocusNode;
  const ChatDrawerMenu(this.editLineFocusNode, {Key? key}) : super(key: key);

  @override
  Widget build(BuildContext context) {
    return Column(children: [Expanded(child: ChatsList(editLineFocusNode))]);
  }
}
