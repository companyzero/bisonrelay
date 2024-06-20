import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/copyable.dart';
import 'package:bruig/components/info_grid.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/snackbar.dart';
import 'package:bruig/models/uistate.dart';
import 'package:bruig/screens/chats.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:bruig/theme_manager.dart';

class UserProfile extends StatefulWidget {
  static String routeName = "${ChatsScreen.routeName}/profile";
  final ClientModel client;
  const UserProfile(this.client, {Key? key}) : super(key: key);

  @override
  State<UserProfile> createState() => _UserProfileState();
}

String _shortLog(String s) => s.length < 16 ? s : s.substring(0, 16);

class _UserProfileState extends State<UserProfile> {
  ClientModel get client => widget.client;
  ChatModel chat = emptyChatModel;
  RatchetDebugInfo ratchetInfo = RatchetDebugInfo.empty();
  AddressBookEntry abEntry = AddressBookEntry.empty();
  bool isIgnored = false;
  bool loading = false;
  bool firstLoading = true;
  ScrollController gridScrollCtrl = ScrollController();

  void readProfile() async {
    var snackbar = SnackBarModel.of(context);

    if (chat == emptyChatModel) {
      setState(() {
        ratchetInfo = RatchetDebugInfo.empty();
        abEntry = AddressBookEntry.empty();
        isIgnored = false;
        loading = false;
      });
      return;
    }

    try {
      var newIgnored = await Golib.isIgnored(chat.id);
      var newAbEntry = await Golib.addressBookEntry(chat.id);
      var newRatchetInfo = await Golib.userRatchetInfo(chat.id);

      setState(() {
        isIgnored = newIgnored;
        ratchetInfo = newRatchetInfo;
        abEntry = newAbEntry;
      });
    } catch (exception) {
      snackbar.error("Unable to load profile: $exception");
    } finally {
      setState(() {
        firstLoading = false;
      });
    }
  }

  void ignore() async {
    var snackbar = SnackBarModel.of(context);
    try {
      setState(() {
        loading = true;
      });
      await Golib.ignoreUser(chat.id);
      setState(() {
        isIgnored = true;
      });
    } catch (exception) {
      snackbar.error("Unable to ignore user: $exception");
    } finally {
      setState(() {
        loading = false;
      });
    }
  }

  void unignore() async {
    var snackbar = SnackBarModel.of(context);
    try {
      setState(() {
        loading = true;
      });
      await Golib.unignoreUser(chat.id);
      setState(() {
        isIgnored = false;
      });
    } catch (exception) {
      snackbar.error("Unable to un-ignore user: $exception");
    } finally {
      setState(() {
        loading = false;
      });
    }
  }

  void confirmBlock() {
    showDialog(
        context: context,
        builder: (context) => AlertDialog(
                title: const Text("Confirm user block"),
                content: const Text("Block this user? This cannot be undone."),
                actions: [
                  TextButton(
                      child: const Text("Cancel"),
                      onPressed: () => Navigator.pop(context)),
                  TextButton(
                      child: const Text("Block"),
                      onPressed: () {
                        Navigator.pop(context);
                        block();
                      }),
                ]));
  }

  void block() async {
    var snackbar = SnackBarModel.of(context);
    try {
      setState(() {
        loading = true;
      });
      await Golib.blockUser(chat.id);
      widget.client.removeChat(chat);
    } catch (exception) {
      snackbar.error("Unable to block user: $exception");
      setState(() => loading = false);
    }
  }

  void hide() async {
    var snackbar = SnackBarModel.of(context);
    try {
      setState(() {
        loading = true;
      });
      widget.client.hideChat(chat);
      widget.client.active = null;
    } catch (exception) {
      snackbar.error("Unable to hide user: $exception");
    } finally {
      setState(() {
        loading = false;
      });
    }
  }

  void activeChatChanged() {
    setState(() {
      if (widget.client.active == null || widget.client.active!.isGC) {
        chat = emptyChatModel;
      } else {
        chat = widget.client.active!;
        readProfile();
      }
    });
  }

  @override
  void initState() {
    super.initState();
    client.activeChat.addListener(activeChatChanged);
    activeChatChanged();
  }

  @override
  void didUpdateWidget(UserProfile oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.client != widget.client) {
      oldWidget.client.activeChat.removeListener(activeChatChanged);
      client.activeChat.addListener(activeChatChanged);
    }
  }

  @override
  void dispose() {
    client.activeChat.removeListener(activeChatChanged);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    bool isScreenSmall = checkIsScreenSmall(context);
    return Container(
        padding: const EdgeInsets.only(left: 15, right: 15, top: 8, bottom: 12),
        child: Column(
          children: [
            Row(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                const Txt.L("User Profile - "),
                Txt.L(chat.nick),
              ],
            ),
            Copyable.txt(Txt.S(chat.id)),
            const SizedBox(height: 20),
            Wrap(spacing: 10, runSpacing: 10, children: [
              isIgnored
                  ? OutlinedButton(
                      onPressed: !loading ? unignore : null,
                      child: const Text("Un-ignore user"))
                  : OutlinedButton(
                      onPressed: !loading ? ignore : null,
                      child: const Text("Ignore user")),
              const SizedBox(height: 20),
              OutlinedButton(
                onPressed: !loading ? hide : null,
                child: const Text("Hide Chat"),
              ),
              const SizedBox(height: 20),
              CancelButton(
                onPressed: !loading ? confirmBlock : null,
                label: "Block User",
              ),
            ]),
            const SizedBox(height: 20),
            const Text("Ratchet Debug Info"),
            const SizedBox(height: 10),
            Expanded(
                child: SimpleInfoGridAdv(
                    textSize: TextSize.small,
                    colLabelSize: 160,
                    separatorWidth: 0,
                    items: [
                  ["First Created", abEntry.firstCreated.toIso8601String()],
                  [
                    "Last Completed KX",
                    abEntry.lastCompletedKx.toIso8601String()
                  ],
                  [
                    "Last Handshake Attempt",
                    abEntry.lastHandshakeAttempt.toIso8601String()
                  ],
                  ["Last Sent Time", ratchetInfo.lastEncTime.toIso8601String()],
                  [
                    "Last Received Time",
                    ratchetInfo.lastDecTime.toIso8601String()
                  ],
                  [
                    "Send RV",
                    Copyable(
                        "${ratchetInfo.sendRVPlain} (${_shortLog(ratchetInfo.sendRV)}..."),
                  ],
                  [
                    "Receive RV",
                    Copyable(
                        "${ratchetInfo.recvRVPlain} (${_shortLog(ratchetInfo.recvRV)}...)")
                  ],
                  [
                    "Drain RV",
                    Copyable(
                        "${ratchetInfo.drainRVPlain} (${_shortLog(ratchetInfo.drainRV)}...)")
                  ],
                  ["My Reset RV", Copyable(ratchetInfo.myResetRV)],
                  ["Their Reset RV", Copyable(ratchetInfo.theirResetRV)],
                  ["Saved Keys", "${ratchetInfo.nbSavedKeys}"],
                  ["Will Ratchet", "${ratchetInfo.willRatchet}"],
                ])),
            const SizedBox(height: 10),
            if (!isScreenSmall)
              ElevatedButton(
                  onPressed: () => client.ui.showProfile.val = false,
                  child: const Text("Done"))
          ],
        ));
  }
}
