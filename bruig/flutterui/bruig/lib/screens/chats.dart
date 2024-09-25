import 'dart:async';

import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/chats_list.dart';
import 'package:bruig/components/addressbook/addressbook.dart';
import 'package:bruig/components/text.dart';
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
        return const Txt.L("Bison Relay");
      }

      // Has active chat.
      ChatModel chat = activeChat.chat!;

      // On small screen, show only chat nick/title.
      bool isScreenSmall = checkIsScreenSmall(context);
      if (isScreenSmall) {
        return Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Txt.L(chat.nick),
              Container(
                  width: 40,
                  margin: const EdgeInsets.only(
                      top: 0, bottom: 0, left: 0, right: 5),
                  child: UserMenuAvatar(client, chat,
                      showChatSideMenuOnTap: true)),
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

      return Txt.L("Chat$suffix$profileSuffix");
    });
  }
}

class ChatsScreen extends StatefulWidget {
  static const routeName = '/chat';
  final ClientModel client;
  final AppNotifications ntfns;
  const ChatsScreen(this.client, this.ntfns, {super.key});

  static gotoChatScreenFor(BuildContext context, ChatModel chat) {
    ClientModel.of(context, listen: false).active = chat;
    Navigator.of(context).pushNamed(routeName);
  }

  @override
  State<ChatsScreen> createState() => _ChatsScreenState();
}

class _FundsNeededPage extends StatelessWidget {
  const _FundsNeededPage();

  @override
  Widget build(BuildContext context) {
    return Container(
        padding: const EdgeInsets.all(20),
        child: Center(
            child: Column(children: [
          const SizedBox(height: 34),
          const Txt.H("Fund Wallet and Channels"),
          const SizedBox(height: 34),
          const Text('''
Bison relay requires active LN channels with outbound capacity to pay to send messages to the server.
'''),
          const SizedBox(height: 34),
          Wrap(runSpacing: 10, children: [
            LoadingScreenButton(
              onPressed: () => Navigator.of(context, rootNavigator: true)
                  .pushNamed("/needsFunds"),
              text: "Add wallet funds",
            ),
            const SizedBox(height: 20, width: 34),
            LoadingScreenButton(
              onPressed: () => Navigator.of(context, rootNavigator: true)
                  .pushNamed(NeedsOutChannelScreen.routeName),
              text: "Create outbound channels",
            )
          ])
        ])));
  }
}

class _LoadingAddressBookPage extends StatelessWidget {
  const _LoadingAddressBookPage();

  @override
  Widget build(BuildContext context) {
    return Consumer<ThemeNotifier>(
        builder: (context, theme, child) => Container(
            padding: const EdgeInsets.all(20),
            child: Center(
                child: Column(children: [
              const SizedBox(height: 34),
              const Txt.H("Loading Address Book"),
              const SizedBox(height: 20),
              LoadingAnimationWidget.waveDots(
                color: theme.colors.onSurface,
                size: 50,
              ),
            ]))));
  }
}

class _InviteNeededPage extends StatefulWidget {
  const _InviteNeededPage();

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
    return Container(
        padding: const EdgeInsets.symmetric(horizontal: 40, vertical: 5),
        child: Center(
            child: Column(children: [
          const Txt.H("Initial Invitation"),
          const SizedBox(height: 34),
          const Text('''
Bison Relay does not rely on a central server for user accounts, so to chat with someone else you need to exchange an invitation with them. This is just a file that should be sent via some other secure transfer method.

After the invitation is accepted, you'll be able to chat with them, and if they know other people, they'll be able to connect you with them.
'''),
          const SizedBox(height: 34),
          Wrap(runSpacing: 10, children: [
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
        ])));
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
  bool metaLeft = false;
  bool metaRight = false;
  bool get anyMod =>
      ctrlLeft ||
      altLeft ||
      shiftLeft ||
      ctrlRight ||
      altRight ||
      shiftRight ||
      metaLeft ||
      metaRight;

  late final FocusNode inputFocusNode;

  Function? noModEnterKeyHandler;

  // If set, this is called when a paste event ("ctrl+v") is detected.
  Function? pasteEventHandler;

  CustomInputFocusNode() {
    inputFocusNode = FocusNode(onKeyEvent: (node, event) {
      if (event.logicalKey.keyId == LogicalKeyboardKey.controlLeft.keyId) {
        ctrlLeft = event is KeyDownEvent;
      } else if (event.logicalKey.keyId ==
          LogicalKeyboardKey.controlRight.keyId) {
        ctrlRight = event is KeyDownEvent;
      } else if (event.logicalKey.keyId == LogicalKeyboardKey.altLeft.keyId) {
        altLeft = event is KeyDownEvent;
      } else if (event.logicalKey.keyId == LogicalKeyboardKey.altRight.keyId) {
        altRight = event is KeyDownEvent;
      } else if (event.logicalKey.keyId == LogicalKeyboardKey.shiftLeft.keyId) {
        shiftLeft = event is KeyDownEvent;
      } else if (event.logicalKey.keyId == LogicalKeyboardKey.metaLeft.keyId) {
        metaLeft = event is KeyDownEvent;
      } else if (event.logicalKey.keyId == LogicalKeyboardKey.metaRight.keyId) {
        metaRight = event is KeyDownEvent;
      } else if (event.logicalKey.keyId ==
          LogicalKeyboardKey.shiftRight.keyId) {
        shiftRight = event is KeyDownEvent;
      } else if (event.logicalKey.keyId == LogicalKeyboardKey.enter.keyId) {
        // When a special handler is set, call it to bypass standard processing
        // of the key and return the 'handled' result.
        if (noModEnterKeyHandler != null && !anyMod) {
          noModEnterKeyHandler!();
          return KeyEventResult.handled;
        }
      } else if ((ctrlLeft || ctrlRight || metaLeft || metaRight) &&
          event.logicalKey.keyId == LogicalKeyboardKey.keyV.keyId &&
          event is KeyDownEvent) {
        if (pasteEventHandler != null) {
          pasteEventHandler!();
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
  final CustomInputFocusNode inputFocusNode = CustomInputFocusNode();
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

    if (!client.hasChats && !client.loadingAddressBook) {
      if (!hasLNBalance) {
        // Only show f user never had any contacts.
        return const _FundsNeededPage();
      }
      return const _InviteNeededPage();
    }
    if (client.loadingAddressBook) {
      return const _LoadingAddressBookPage();
    }

    bool isScreenSmall = checkIsScreenSmall(context);
    return !isScreenSmall
        ? Row(children: [
            ActiveChatsListMenu(client, inputFocusNode),
            Expanded(
                child: Container(
              margin: const EdgeInsets.all(1),
              child: ActiveChat(client, inputFocusNode),
            )),
          ])
        : Consumer<ActiveChatModel>(
            builder: (context, activeChat, child) => activeChat.empty
                ? ActiveChatsListMenu(client, inputFocusNode)
                : ActiveChat(client, inputFocusNode));
  }
}
