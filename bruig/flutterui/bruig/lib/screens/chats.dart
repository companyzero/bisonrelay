import 'dart:async';

import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/chats_list.dart';
import 'package:bruig/components/addressbook/addressbook.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/notifications.dart';
import 'package:bruig/models/uistate.dart';
import 'package:bruig/screens/needs_out_channel.dart';
import 'package:bruig/theme_manager.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:provider/provider.dart';
import 'package:bruig/components/chat/active_chat.dart';
import 'package:loading_animation_widget/loading_animation_widget.dart';

import 'package:bruig/components/interactive_avatar.dart';

class ChatsScreenTitle extends StatelessWidget {
  const ChatsScreenTitle({super.key});

  @override
  Widget build(BuildContext context) {
    return Consumer4<ClientModel, ActiveChatModel, ShowProfileModel,
            ThemeNotifier>(
        builder: (context, client, activeChat, showProfile, theme, child) {
      var activeHeading = activeChat.chat;
      var showAddressBook = client.ui.showAddressBook.val;

      // No active chat or address book page is active.
      if (activeHeading == null || showAddressBook) {
        return Row(mainAxisAlignment: MainAxisAlignment.start, children: [
          Text("Bison Relay",
              textAlign: TextAlign.center,
              style: TextStyle(
                  fontSize: theme.getLargeFont(context),
                  color: Theme.of(context).focusColor))
        ]);
      }

      // Has active chat.
      ChatModel chat = activeChat.chat!;

      // On small screen, show only chat nick/title.
      bool isScreenSmall = MediaQuery.of(context).size.width <= 500;
      if (isScreenSmall) {
        return Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text(chat.nick,
                  textAlign: TextAlign.center,
                  style: TextStyle(
                      fontSize: theme.getLargeFont(context),
                      color: Theme.of(context).focusColor)),
              Container(
                  width: 40,
                  margin: const EdgeInsets.only(
                      top: 0, bottom: 0, left: 0, right: 5),
                  child: UserMenuAvatar(client, chat, onTap: () {
                    client.showSubMenu(chat.isGC, chat.id);
                  })),
            ]);
      }

      // Full chat path.
      bool profile = showProfile.val;
      var suffix = chat.nick != "" ? " / ${chat.nick}" : "";
      var profileSuffix = profile
          ? chat.isGC
              ? " / Manage Group Chat"
              : " / Profile"
          : "";

      return Text("Chat$suffix$profileSuffix",
          style: TextStyle(
              fontSize: theme.getLargeFont(context),
              color: Theme.of(context).focusColor));
    });
  }
}

class ChatsScreen extends StatefulWidget {
  static const routeName = '/chat';
  final ClientModel client;
  final AppNotifications ntfns;
  const ChatsScreen(this.client, this.ntfns, {Key? key}) : super(key: key);

  @override
  State<ChatsScreen> createState() => _ChatsScreenState();
}

class _FundsNeededPage extends StatelessWidget {
  const _FundsNeededPage({super.key});

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var backgroundColor = theme.backgroundColor;
    var textColor = theme.dividerColor;
    var secondaryTextColor = theme.focusColor;
    bool isScreenSmall = MediaQuery.of(context).size.width <= 500;

    return Consumer<ThemeNotifier>(
        builder: (context, theme, child) => Container(
            padding: const EdgeInsets.all(20),
            decoration: BoxDecoration(color: backgroundColor),
            child: Center(
                child: Column(
              children: [
                const SizedBox(height: 34),
                Text("Fund Wallet and Channels",
                    style: TextStyle(
                        color: textColor,
                        fontSize: theme.getHugeFont(context),
                        fontWeight: FontWeight.w200)),
                const SizedBox(height: 34),
                Text('''
Bison relay requires active LN channels with outbound capacity to pay to send messages to the server.
''',
                    style: TextStyle(
                        color: secondaryTextColor,
                        fontSize: theme.getMediumFont(context),
                        fontWeight: FontWeight.w300)),
                const SizedBox(height: 34),
                Center(
                  child: Flex(
                      direction:
                          isScreenSmall ? Axis.vertical : Axis.horizontal,
                      mainAxisAlignment: MainAxisAlignment.center,
                      children: [
                        LoadingScreenButton(
                          onPressed: () =>
                              Navigator.of(context, rootNavigator: true)
                                  .pushNamed("/needsFunds"),
                          text: "Add wallet funds",
                        ),
                        const SizedBox(height: 20, width: 34),
                        LoadingScreenButton(
                          onPressed: () =>
                              Navigator.of(context, rootNavigator: true)
                                  .pushNamed(NeedsOutChannelScreen.routeName),
                          text: "Create outbound channels",
                        )
                      ]),
                ),
              ],
            ))));
  }
}

class _LoadingAddressBookPage extends StatelessWidget {
  const _LoadingAddressBookPage({super.key});

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var backgroundColor = theme.backgroundColor;
    var textColor = theme.dividerColor;

    return Consumer<ThemeNotifier>(
        builder: (context, theme, child) => Container(
            padding: const EdgeInsets.all(20),
            decoration: BoxDecoration(color: backgroundColor),
            child: Center(
                child: Column(children: [
              const SizedBox(height: 34),
              Text("Loading Address Book",
                  style: TextStyle(
                      color: textColor,
                      fontSize: theme.getHugeFont(context),
                      fontWeight: FontWeight.w200)),
              const SizedBox(height: 20),
              LoadingAnimationWidget.waveDots(
                color: textColor,
                size: 50,
              ),
            ]))));
  }
}

class _InviteNeededPage extends StatefulWidget {
  _InviteNeededPage({super.key});

  @override
  State<_InviteNeededPage> createState() => _InviteNeededPageState();
}

class _InviteNeededPageState extends State<_InviteNeededPage> {
  Timer? _debounce;

  @override
  void dispose() {
    _debounce?.cancel();
    super.dispose();
  }

  void debouncedLoadInvite(BuildContext context) {
    if (_debounce?.isActive ?? false) _debounce!.cancel();
    _debounce = Timer(const Duration(milliseconds: 500), () async {
      loadInvite(context);
    });
  }

  @override
  Widget build(BuildContext context) {
    var theme = Theme.of(context);
    var backgroundColor = theme.backgroundColor;
    var textColor = theme.dividerColor;
    var secondaryTextColor = theme.focusColor;
    bool isScreenSmall = MediaQuery.of(context).size.width <= 500;
    return Consumer<ThemeNotifier>(
        builder: (context, theme, child) => Container(
            padding: const EdgeInsets.all(20),
            decoration: BoxDecoration(color: backgroundColor),
            child: Center(
                child: Column(
              children: [
                const SizedBox(height: 34),
                Text("Initial Invitation",
                    style: TextStyle(
                        color: textColor,
                        fontSize: theme.getHugeFont(context),
                        fontWeight: FontWeight.w200)),
                const SizedBox(height: 34),
                Text('''
Bison Relay does not rely on a central server for user accounts, so to chat with someone else you need to exchange an invitation with them. This is just a file that should be sent via some other secure transfer method.

After the invitation is accepted, you'll be able to chat with them, and if they know other people, they'll be able to connect you with them.
''',
                    style: TextStyle(
                        color: secondaryTextColor,
                        fontSize: theme.getMediumFont(context),
                        fontWeight: FontWeight.w300)),
                const SizedBox(height: 34),
                Center(
                  child: Flex(
                      direction:
                          isScreenSmall ? Axis.vertical : Axis.horizontal,
                      mainAxisAlignment: MainAxisAlignment.center,
                      children: [
                        LoadingScreenButton(
                          onPressed: () => debouncedLoadInvite(context),
                          text: "Load Invitation",
                        ),
                        const SizedBox(height: 20, width: 34),
                        LoadingScreenButton(
                          onPressed: () => generateInvite(context),
                          text: "Create Invitation",
                        )
                      ]),
                ),
              ],
            ))));
  }
}

// This class is a hack to pass a FocusNode down the component stack along with
// callbacks for the Input() class to know when to send vs when to add new lines
// to the input component. There should to be a better way to do this.
class CustomInputFocusNode {
  bool ctrlLeft = false;
  bool ctrlRight = false;
  bool shiftLeft = false;
  bool shiftRight = false;
  bool altLeft = false;
  bool altRight = false;
  bool get anyMod =>
      ctrlLeft || altLeft || shiftLeft || ctrlRight || altRight || shiftRight;

  late final FocusNode inputFocusNode;

  Function? noModEnterKeyHandler;

  CustomInputFocusNode() {
    inputFocusNode = FocusNode(onKeyEvent: (node, event) {
      if (event.logicalKey.keyId == LogicalKeyboardKey.controlLeft.keyId) {
        ctrlLeft = !ctrlLeft;
      } else if (event.logicalKey.keyId ==
          LogicalKeyboardKey.controlRight.keyId) {
        ctrlRight = !ctrlRight;
      } else if (event.logicalKey.keyId == LogicalKeyboardKey.altLeft.keyId) {
        altLeft = !altLeft;
      } else if (event.logicalKey.keyId == LogicalKeyboardKey.altRight.keyId) {
        altRight = !altRight;
      } else if (event.logicalKey.keyId == LogicalKeyboardKey.shiftLeft.keyId) {
        shiftLeft = !shiftLeft;
      } else if (event.logicalKey.keyId ==
          LogicalKeyboardKey.shiftRight.keyId) {
        shiftRight = !shiftRight;
      } else if (event.logicalKey.keyId == LogicalKeyboardKey.enter.keyId) {
        // When a special handler is set, call it to bypass standard processing
        // of the key and return the 'handled' result.
        if (noModEnterKeyHandler != null && !anyMod) {
          noModEnterKeyHandler!();
          return KeyEventResult.handled;
        }
      }

      return KeyEventResult.ignored;
    });
  }
}

class _ChatsScreenState extends State<ChatsScreen> {
  ClientModel get client => widget.client;
  AppNotifications get ntfns => widget.ntfns;
  CustomInputFocusNode inputFocusNode = CustomInputFocusNode();
  bool hasLNBalance = false;
  List<PostListItem> userPostList = [];
  Timer? checkLNTimer;

  // check if ln wallet has balance. busywait, needs to be changed into a ntf.
  void keepCheckingLNHasBalance() async {
    if (client.activeChats.isNotEmpty) {
      // Doesn't matter, we already have contacts, so won't show onboard pages.
      return;
    }

    check() async {
      var balances = await Golib.lnGetBalances();
      var newHasBalance = balances.channel.maxOutboundAmount > 0;
      if (!newHasBalance) return false;
      if (mounted) {
        setState(() {
          hasLNBalance = newHasBalance;
        });
      }
      return true;
    }

    if (await check()) return;

    checkLNTimer = Timer.periodic(const Duration(seconds: 1), (timer) async {
      if (await check()) timer.cancel();
    });
  }

  void showAddressBookChanged() => setState(() {});

  @override
  void initState() {
    super.initState();
    keepCheckingLNHasBalance();
    client.ui.showAddressBook.addListener(showAddressBookChanged);
  }

  @override
  void didUpdateWidget(ChatsScreen oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.client != client) {
      oldWidget.client.ui.showAddressBook
          .removeListener(showAddressBookChanged);
      client.ui.showAddressBook.addListener(showAddressBookChanged);
    }
  }

  @override
  void dispose() {
    checkLNTimer?.cancel();
    client.ui.showAddressBook.removeListener(showAddressBookChanged);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (client.ui.showAddressBook.val) {
      return AddressBook(client, inputFocusNode);
    }

    var theme = Theme.of(context);
    var backgroundColor = theme.backgroundColor;

    if (!client.hasChats && !client.loadingAddressBook) {
      if (!hasLNBalance) {
        // Only show f user never had any contacts.
        return const _FundsNeededPage();
      }
      return _InviteNeededPage();
    }
    if (client.loadingAddressBook) {
      return const _LoadingAddressBookPage();
    }

    bool isScreenSmall = MediaQuery.of(context).size.width <= 500;
    return !isScreenSmall
        ? Row(children: [
            SizedBox(
                width: 200,
                height: double.infinity,
                child: ActiveChatsListMenu(client, inputFocusNode)),
            Expanded(
                child: Container(
              margin: const EdgeInsets.all(1),
              decoration: BoxDecoration(
                color: backgroundColor,
                borderRadius: BorderRadius.circular(3),
              ),
              child: ActiveChat(client, inputFocusNode),
            )),
          ])
        : client.active == null
            ? ActiveChatsListMenu(client, inputFocusNode)
            : ActiveChat(client, inputFocusNode);
  }
}
