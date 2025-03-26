import 'package:bruig/components/interactive_avatar.dart';
import 'package:bruig/components/text.dart';
import 'package:bruig/components/usersearch/user_search_model.dart';
import 'package:bruig/models/client.dart';
import 'package:bruig/models/emoji.dart';
import 'package:bruig/models/uistate.dart';
import 'package:bruig/screens/chats.dart';
import 'package:bruig/screens/ln/components.dart';
import 'package:bruig/theme_manager.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

class _SearchChatItemW extends StatefulWidget {
  final ClientModel client;
  final ChatModel chat;
  final ValueChanged<ChatModel>? onChatTapped;
  final UserSelectionModel? userSelModel;
  const _SearchChatItemW(
    this.client,
    this.chat, {
    this.onChatTapped,
    this.userSelModel,
  });

  @override
  State<_SearchChatItemW> createState() => __SearchChatItemWState();
}

class __SearchChatItemWState extends State<_SearchChatItemW> {
  ClientModel get client => widget.client;
  UserSelectionModel? get userSelModel => widget.userSelModel;
  ChatModel get chat => widget.chat;
  bool selected = false;

  void onTap() {
    if (widget.onChatTapped != null) {
      widget.onChatTapped!(chat);
    }
    if (widget.userSelModel != null) {
      setState(() => selected = widget.userSelModel!.toggle(chat));
    }
  }

  void selModelChanged() {
    var newSel = widget.userSelModel!.contains(chat);
    if (newSel != selected) {
      setState(() => selected = newSel);
    }
  }

  @override
  void initState() {
    super.initState();
    selected = userSelModel?.contains(chat) ?? false;
    userSelModel?.addListener(selModelChanged);
  }

  @override
  void didUpdateWidget(covariant _SearchChatItemW oldWidget) {
    super.didUpdateWidget(oldWidget);
    selected = userSelModel?.contains(chat) ?? false;
    if (userSelModel != oldWidget.userSelModel) {
      oldWidget.userSelModel?.removeListener(selModelChanged);
      userSelModel?.addListener(selModelChanged);
    }
  }

  @override
  void dispose() {
    userSelModel?.removeListener(selModelChanged);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return ListTile(
      leading: UserMenuAvatar(client, chat),
      selected: selected,
      onTap: onTap,
      enabled: true,
      title: Row(children: [
        Expanded(child: Txt(chat.nick, overflow: TextOverflow.ellipsis)),
        if (chat.isGC) ...[
          const SizedBox(width: 5),
          const Txt(
            "GC",
            color: TextColor.onSecondaryFixedVariant,
          )
        ],
      ]),
    );
  }
}

class ChatSearchInput extends StatefulWidget {
  final CustomInputFocusNode inputFocusNode;
  final bool onlyUsers;
  final ValueChanged<String> onChanged;
  final String? hintText;
  const ChatSearchInput(this.inputFocusNode, this.onlyUsers, this.onChanged,
      {this.hintText, super.key});

  @override
  State<ChatSearchInput> createState() => _ChatSearchInputState();
}

class _ChatSearchInputState extends State<ChatSearchInput> {
  final controller = TextEditingController();

  final FocusNode node = FocusNode();

  @override
  void initState() {
    super.initState();
  }

  @override
  void dispose() {
    super.dispose();
  }

  @override
  void didUpdateWidget(ChatSearchInput oldWidget) {
    super.didUpdateWidget(oldWidget);
    widget.inputFocusNode.inputFocusNode.requestFocus();
    if (oldWidget.onlyUsers != widget.onlyUsers ||
        oldWidget.hintText != widget.hintText) {
      controller.text = "";
    }
  }

  void handleKeyPress(KeyEvent event) {
    bool modPressed = HardwareKeyboard.instance.isShiftPressed ||
        HardwareKeyboard.instance.isControlPressed;
    widget.onChanged(controller.text);
    if (event.logicalKey.keyLabel == "Enter" && !modPressed) {
      controller.value = const TextEditingValue(
          text: "", selection: TextSelection.collapsed(offset: 0));
    }
  }

  @override
  Widget build(BuildContext context) {
    bool isScreenSmall = checkIsScreenSmall(context);
    return KeyboardListener(
        focusNode: node,
        onKeyEvent: handleKeyPress,
        child: Container(
            margin: const EdgeInsets.only(bottom: 5),
            child: Row(children: [
              Expanded(
                child: TextField(
                  autofocus: isScreenSmall ? false : true,
                  focusNode: widget.inputFocusNode.inputFocusNode,
                  controller: controller,
                  minLines: 1,
                  maxLines: null,
                  keyboardType: TextInputType.multiline,
                  decoration: InputDecoration(
                    isDense: true,
                    hintText: widget.hintText ??
                        'Search name of user ${widget.onlyUsers ? "" : "or group chat"}',
                    border: const OutlineInputBorder(
                        borderRadius: BorderRadius.all(Radius.circular(30)),
                        borderSide: BorderSide(width: 1)),
                  ),
                ),
              )
            ])));
  }
}

enum UserSearchPanelTargets {
  usersAndGCs,
  users,
  gcs,
}

class UserSearchPanel extends StatefulWidget {
  final ClientModel client;
  final UserSearchPanelTargets targets;
  final CustomInputFocusNode? inputFocusNode;
  final VoidCallback? onCancel;
  final VoidCallback? onConfirm;
  final ValueChanged<ChatModel>? onChatTapped;
  final String confirmLabel;
  final UserSelectionModel? userSelModel;
  final bool showButtonsRow;
  final String? searchInputHintText;
  final ValueChanged<String>? onSearchInputChanged;
  final List<ChatModel>? sourceChats;
  const UserSearchPanel(
    this.client, {
    super.key,
    this.targets = UserSearchPanelTargets.users,
    this.confirmLabel = "Confirm",
    this.inputFocusNode,
    this.userSelModel,
    this.showButtonsRow = true,
    this.searchInputHintText,
    this.onCancel,
    this.onConfirm,
    this.onChatTapped,
    this.onSearchInputChanged,
    this.sourceChats,
  });

  @override
  State<UserSearchPanel> createState() => _UserSearchPanelState();
}

class _UserSearchPanelState extends State<UserSearchPanel> {
  ClientModel get client => widget.client;
  List<ChatModel> chats = [];
  late CustomInputFocusNode inputFocusNode;
  String filterSearchString = "";
  List<ChatModel> filteredSearch = [];
  bool get allowGCs =>
      widget.targets == UserSearchPanelTargets.usersAndGCs ||
      widget.targets == UserSearchPanelTargets.gcs;
  bool get allowUsers =>
      widget.targets == UserSearchPanelTargets.usersAndGCs ||
      widget.targets == UserSearchPanelTargets.users;

  void onInputChanged(String value) {
    var newSearchResults = client.searchChats(value,
        ignoreGC: !allowGCs,
        ignoreUsers: !allowUsers,
        sourceChats: widget.sourceChats);
    setState(() {
      filterSearchString = value;
      filteredSearch = newSearchResults.toList();
    });
    if (widget.onSearchInputChanged != null) {
      widget.onSearchInputChanged!(value);
    }
  }

  void onCancel() {
    if (widget.onCancel != null) {
      widget.onCancel!();
    }
  }

  void recalcChatsList() {
    var newChats = widget.sourceChats ??
        client.hiddenChats.sorted + client.activeChats.sorted;
    if (!allowGCs) {
      newChats.removeWhere((c) => c.isGC);
    }
    if (!allowUsers) {
      newChats.removeWhere((c) => !c.isGC);
    }
    newChats
        .sort((a, b) => a.nick.toLowerCase().compareTo(b.nick.toLowerCase()));
    chats = newChats;
  }

  @override
  void initState() {
    super.initState();
    inputFocusNode =
        widget.inputFocusNode ?? CustomInputFocusNode(TypingEmojiSelModel());
    recalcChatsList();
  }

  @override
  void didUpdateWidget(UserSearchPanel oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.targets != widget.targets) {
      recalcChatsList();
      filterSearchString = "";
      filteredSearch = [];
    }
  }

  @override
  Widget build(BuildContext context) {
    String resultsHeaderTxt;
    if (chats.isEmpty) {
      resultsHeaderTxt = "No chats available";
    } else if (filterSearchString == "") {
      resultsHeaderTxt = "All chats";
    } else if (filteredSearch.isEmpty) {
      resultsHeaderTxt = "No matching chats";
    } else {
      resultsHeaderTxt = "Matching chats";
    }

    var resultsChat = filterSearchString != "" ? filteredSearch : chats;

    return Column(children: [
      ChatSearchInput(inputFocusNode, !allowGCs, onInputChanged,
          hintText: widget.searchInputHintText),
      const SizedBox(height: 10),
      if (widget.showButtonsRow)
        Row(mainAxisAlignment: MainAxisAlignment.spaceEvenly, children: [
          if (widget.confirmLabel != "")
            TextButton(
                onPressed: widget.onConfirm, child: Txt.L(widget.confirmLabel)),
          TextButton(onPressed: onCancel, child: const Txt.L("Cancel")),
          // const SizedBox(height: 10),
        ]),
      LNInfoSectionHeader(resultsHeaderTxt),
      const SizedBox(height: 10),
      Expanded(
          child: Material(
              clipBehavior: Clip.hardEdge,
              child: ListView.builder(
                  itemCount: resultsChat.length,
                  itemBuilder: (context, index) => _SearchChatItemW(
                        client,
                        resultsChat[index],
                        userSelModel: widget.userSelModel,
                        onChatTapped: widget.onChatTapped,
                      )))),
    ]);
  }
}
