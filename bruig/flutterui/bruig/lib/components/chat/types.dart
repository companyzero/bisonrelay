import 'package:bruig/models/client.dart';

typedef SendMsg = void Function(String msg);
typedef MakeActiveCB = void Function(ChatModel? c);
typedef ShowSubMenuCB = void Function();
typedef OpenReplyDMCB = void Function(bool, String);
