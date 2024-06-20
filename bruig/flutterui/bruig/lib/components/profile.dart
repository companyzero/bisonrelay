import 'package:bruig/components/buttons.dart';
import 'package:bruig/components/copyable.dart';
import 'package:bruig/components/info_grid.dart';
import 'package:bruig/components/snackbars.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/screens/chats.dart';
import 'package:flutter/material.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:golib_plugin/golib_plugin.dart';
import 'package:tuple/tuple.dart';
import 'package:bruig/theme_manager.dart';
import 'package:provider/provider.dart';
import 'package:bruig/components/empty_widget.dart';

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
      showErrorSnackbar(context, "Unable to load profile: $exception");
    } finally {
      setState(() {
        firstLoading = false;
      });
    }
  }

  void ignore() async {
    try {
      setState(() {
        loading = true;
      });
      await Golib.ignoreUser(chat.id);
      setState(() {
        isIgnored = true;
      });
    } catch (exception) {
      showErrorSnackbar(context, "Unable to ignore user: $exception");
    } finally {
      setState(() {
        loading = false;
      });
    }
  }

  void unignore() async {
    try {
      setState(() {
        loading = true;
      });
      await Golib.unignoreUser(chat.id);
      setState(() {
        isIgnored = false;
      });
    } catch (exception) {
      showErrorSnackbar(context, "Unable to un-ignore user: $exception");
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
    try {
      setState(() {
        loading = true;
      });
      await Golib.blockUser(chat.id);
      widget.client.removeChat(chat);
    } catch (exception) {
      showErrorSnackbar(context, "Unable to block user: $exception");
      setState(() => loading = false);
    }
  }

  void hide() async {
    try {
      setState(() {
        loading = true;
      });
      widget.client.hideChat(chat);
      widget.client.active = null;
    } catch (exception) {
      showErrorSnackbar(context, "Unable to hide user: $exception");
    } finally {
      setState(() {
        loading = false;
      });
    }
  }

  void activeChatChanged() {
    setState(() {
      chat = widget.client.active ?? emptyChatModel;
      readProfile();
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
    var theme = Theme.of(context);
    var textColor = theme.focusColor;

    return Consumer<ThemeNotifier>(builder: (context, theme, _) {
      var txtTS = TextStyle(
          color: textColor,
          fontWeight: FontWeight.w100,
          fontSize: theme.getSmallFont(context));
      var headTS = TextStyle(
          color: textColor,
          fontWeight: FontWeight.w400,
          fontSize: theme.getSmallFont(context));

      bool isScreenSmall = MediaQuery.of(context).size.width <= 500;

      return Container(
          padding: const EdgeInsets.all(20),
          child: Column(
            children: [
              Row(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  Text("User Profile - ",
                      style: TextStyle(
                          color: textColor,
                          fontSize: theme.getMediumFont(context))),
                  Text(chat.nick,
                      style: TextStyle(
                          color: textColor,
                          fontSize: theme.getMediumFont(context),
                          fontWeight: FontWeight.bold)),
                ],
              ),
              Copyable(chat.id,
                  textStyle:
                      TextStyle(color: textColor, fontWeight: FontWeight.w100)),
              const SizedBox(height: 20),
              Wrap(spacing: 10, runSpacing: 10, children: [
                isIgnored
                    ? ElevatedButton(
                        onPressed: !loading ? unignore : null,
                        child: const Text("Un-ignore user"))
                    : ElevatedButton(
                        onPressed: !loading ? ignore : null,
                        child: const Text("Ignore user")),
                const SizedBox(height: 20),
                ElevatedButton(
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
              Text("Ratchet Debug Info",
                  style: TextStyle(
                      color: textColor,
                      fontSize: theme.getMediumFont(context))),
              const SizedBox(height: 10),
              Expanded(
                  child: SimpleInfoGrid(
                colLabelSize: 160,
                separatorWidth: 0,
                [
                  Tuple2(
                      Text("First Created", style: headTS),
                      Text(abEntry.firstCreated.toIso8601String(),
                          style: txtTS)),
                  Tuple2(
                      Text("Last Completed KX", style: headTS),
                      Text(abEntry.lastCompletedKx.toIso8601String(),
                          style: txtTS)),
                  Tuple2(
                      Text("Last Handshake Attempt", style: headTS),
                      Text(abEntry.lastHandshakeAttempt.toIso8601String(),
                          style: txtTS)),
                  Tuple2(
                      Text("Last Sent Time", style: headTS),
                      Text(ratchetInfo.lastEncTime.toIso8601String(),
                          style: txtTS)),
                  Tuple2(
                      Text("Last Received Time", style: headTS),
                      Text(ratchetInfo.lastDecTime.toIso8601String(),
                          style: txtTS)),
                  Tuple2(
                      Text("Send RV", style: headTS),
                      Copyable(
                          "${ratchetInfo.sendRVPlain} (${_shortLog(ratchetInfo.sendRV)}...)",
                          textStyle: txtTS)),
                  Tuple2(
                      Text("Receive RV", style: headTS),
                      Copyable(
                          "${ratchetInfo.recvRVPlain} (${_shortLog(ratchetInfo.recvRV)}...)",
                          textStyle: txtTS)),
                  Tuple2(
                      Text("Drain RV", style: headTS),
                      Copyable(
                          "${ratchetInfo.drainRVPlain} (${_shortLog(ratchetInfo.drainRV)}...)",
                          textStyle: txtTS)),
                  Tuple2(Text("My Reset RV", style: headTS),
                      Copyable(ratchetInfo.myResetRV, textStyle: txtTS)),
                  Tuple2(Text("Their Reset RV", style: headTS),
                      Copyable(ratchetInfo.theirResetRV, textStyle: txtTS)),
                  Tuple2(Text("Saved Keys", style: headTS),
                      Text(ratchetInfo.nbSavedKeys.toString(), style: txtTS)),
                  Tuple2(Text("Will Ratchet", style: headTS),
                      Text(ratchetInfo.willRatchet.toString(), style: txtTS)),
                ],
                controller: gridScrollCtrl,
              )),
              isScreenSmall
                  ? const Empty()
                  : ElevatedButton(
                      onPressed: () => client.ui.showProfile.val = false,
                      child: const Text("Done"))
            ],
          ));
    });
  }
}
