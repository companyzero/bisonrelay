import 'dart:async';
import 'dart:io';

import 'package:flutter_local_notifications/flutter_local_notifications.dart';
import 'package:golib_plugin/definitions.dart';
// import 'package:permission_handler/permission_handler.dart';
import 'package:bruig/storage_manager.dart';

int id = 0;

class Notification {
  final int id;
  String? title;
  String? body;
  NotificationDetails? notificationDetails;
  String? payload;

  Notification(
      this.id, this.title, this.body, this.notificationDetails, this.payload);
}

class NotificationService {
  static final NotificationService _notificationService =
      NotificationService._internal();

  factory NotificationService() {
    return _notificationService;
  }
  bool _notificationsGranted = true;

  final FlutterLocalNotificationsPlugin flutterLocalNotificationsPlugin =
      FlutterLocalNotificationsPlugin();

  /// Streams are created so that app can respond to notification-related events
  /// since the plugin is initialised in the `main` function
  final StreamController<ReceivedNotification>
      didReceiveLocalNotificationStream =
      StreamController<ReceivedNotification>.broadcast();

  final StreamController<String?> selectNotificationStream =
      StreamController<String?>.broadcast();

  final String portName = 'notification_send_port';

  String? selectedNotificationPayload;

  /// A notification action which triggers a url launch event
  final String urlLaunchActionId = 'id_1';

  /// A notification action which triggers a App navigation event
  final String navigationActionId = 'id_3';

  /// Defines a iOS/MacOS notification category for text input actions.
  final String darwinNotificationCategoryText = 'textCategory';

  /// Defines a iOS/MacOS notification category for plain actions.
  final String darwinNotificationCategoryPlain = 'plainCategory';

  final List<Notification> _notificationsToBeSent = [];

  late Timer notificationChecker = Timer(const Duration(seconds: 10), () async {
    if (_notificationsToBeSent.isNotEmpty) {
      var set = <String>{};
      List<Notification> uniquePayloads =
          _notificationsToBeSent.where((e) => set.add(e.payload!)).toList();
      List<Notification> consolidatedNtfns = [];
      for (var uniqueNtfn in uniquePayloads) {
        var groupCount = 0;
        for (var ntfn in _notificationsToBeSent) {
          if (ntfn.payload == uniqueNtfn.payload) {
            groupCount++;
          }
        }
        String groupName = "";
        if (uniqueNtfn.payload!.contains("chat")) {
          var nick = uniqueNtfn.payload!.split(":");
          if (nick.length > 1) {
            groupName = nick[1];
          }
        } else if (uniqueNtfn.payload!.contains("post")) {
        } else if (uniqueNtfn.payload!.contains("post comment")) {}

        uniqueNtfn.body =
            "You have $groupCount new message${groupCount <= 1 ? "" : "s"} ${uniqueNtfn.payload!.contains("gc") ? 'in' : 'from'} $groupName";

        consolidatedNtfns.add(uniqueNtfn);
      }

      for (var newNtfn in consolidatedNtfns) {
        var notification = newNtfn;
        _notificationService.flutterLocalNotificationsPlugin.show(
            notification.id,
            notification.title,
            notification.body,
            notification.notificationDetails,
            payload: notification.payload);
      }
      _notificationsToBeSent.clear();
    }
  });

  @pragma('vm:entry-point')
  void notificationTapBackground(NotificationResponse notificationResponse) {
    // ignore: avoid_print
    print('notification(${notificationResponse.id}) action tapped: '
        '${notificationResponse.actionId} with'
        ' payload: ${notificationResponse.payload}');
    if (notificationResponse.input?.isNotEmpty ?? false) {
      // ignore: avoid_print
      print(
          'notification action tapped with input: ${notificationResponse.input}');
    }
  }

  Future<bool> allowNotifications() async {
    // Android notifications are done through the native plugin.
    if (Platform.isAndroid) return false;
    if (!_notificationsGranted) return false;
    bool notificationsEnabled = false;
    await StorageManager.readData(StorageManager.notificationsKey)
        .then((value) {
      if (value != null) {
        notificationsEnabled = value;
      }
    });
    return notificationsEnabled;
  }

  Future<void> init() async {
    /*
    const AndroidInitializationSettings initializationSettingsAndroid =
        AndroidInitializationSettings('app_icon');
    */

    final List<DarwinNotificationCategory> darwinNotificationCategories =
        <DarwinNotificationCategory>[
      DarwinNotificationCategory(
        darwinNotificationCategoryText,
        actions: <DarwinNotificationAction>[
          DarwinNotificationAction.text(
            'text_1',
            'Action 1',
            buttonTitle: 'Send',
            placeholder: 'Placeholder',
          ),
        ],
      ),
      DarwinNotificationCategory(
        darwinNotificationCategoryPlain,
        actions: <DarwinNotificationAction>[
          DarwinNotificationAction.plain('id_1', 'Action 1'),
          DarwinNotificationAction.plain(
            'id_2',
            'Action 2 (destructive)',
            options: <DarwinNotificationActionOption>{
              DarwinNotificationActionOption.destructive,
            },
          ),
          DarwinNotificationAction.plain(
            navigationActionId,
            'Action 3 (foreground)',
            options: <DarwinNotificationActionOption>{
              DarwinNotificationActionOption.foreground,
            },
          ),
          DarwinNotificationAction.plain(
            'id_4',
            'Action 4 (auth required)',
            options: <DarwinNotificationActionOption>{
              DarwinNotificationActionOption.authenticationRequired,
            },
          ),
        ],
        options: <DarwinNotificationCategoryOption>{
          DarwinNotificationCategoryOption.hiddenPreviewShowTitle,
        },
      )
    ];

    /// Note: permissions aren't requested here just to demonstrate that can be
    /// done later
    final DarwinInitializationSettings initializationSettingsDarwin =
        DarwinInitializationSettings(
      requestAlertPermission: false,
      requestBadgePermission: false,
      requestSoundPermission: false,
      onDidReceiveLocalNotification:
          (int id, String? title, String? body, String? payload) async {
        didReceiveLocalNotificationStream.add(
          ReceivedNotification(
            id: id,
            title: title,
            body: body,
            payload: payload,
          ),
        );
      },
      notificationCategories: darwinNotificationCategories,
    );
    final LinuxInitializationSettings initializationSettingsLinux =
        LinuxInitializationSettings(
      defaultActionName: 'Open notification',
      defaultIcon: AssetsLinuxIcon('assets/icons/app_icon.png'),
    );
    final InitializationSettings initializationSettings =
        InitializationSettings(
      // android: initializationSettingsAndroid,
      iOS: initializationSettingsDarwin,
      macOS: initializationSettingsDarwin,
      linux: initializationSettingsLinux,
    );
    await flutterLocalNotificationsPlugin.initialize(
      initializationSettings,
      onDidReceiveNotificationResponse:
          (NotificationResponse notificationResponse) {
        switch (notificationResponse.notificationResponseType) {
          case NotificationResponseType.selectedNotification:
            selectNotificationStream.add(notificationResponse.payload);
            break;
          case NotificationResponseType.selectedNotificationAction:
            if (notificationResponse.actionId == navigationActionId) {
              selectNotificationStream.add(notificationResponse.payload);
            }
            break;
        }
      },
      onDidReceiveBackgroundNotificationResponse:
          (Platform.isAndroid) ? null : notificationTapBackground,
    );
    /*
    if (Platform.isAndroid) {
      await _notificationService.isAndroidPermissionGranted();
    }
    */
    _notificationService.requestPermissions();
  }

  // This runs as a timer to catch multiple messages coming in at once to avoid,
  // spamming the user with tons of messages.  This is especially important on
  // startup when messages are received from when BR was not in use.  When
  // a new message is received and notification created, the timer is reset.
  void _startNotificationTimer() {
    notificationChecker = Timer(const Duration(seconds: 10), () async {
      if (_notificationsToBeSent.isNotEmpty) {
        var set = Set<String>();
        List<Notification> uniquePayloads =
            _notificationsToBeSent.where((e) => set.add(e.payload!)).toList();
        List<Notification> consolidatedNtfns = [];
        for (var uniqueNtfn in uniquePayloads) {
          var groupCount = 0;
          for (var ntfn in _notificationsToBeSent) {
            if (ntfn.payload == uniqueNtfn.payload) {
              groupCount++;
            }
          }
          String groupName = "";
          if (uniqueNtfn.payload!.contains("chat")) {
            var nick = uniqueNtfn.payload!.split(":");
            if (nick.length > 1) {
              groupName = nick[1];
            }
          } else if (uniqueNtfn.payload!.contains("post")) {
          } else if (uniqueNtfn.payload!.contains("post comment")) {}

          uniqueNtfn.body = groupCount > 1
              ? "You have $groupCount new messages ${uniqueNtfn.payload!.contains("gc") ? 'in' : 'from'} $groupName"
              : uniqueNtfn.body;

          consolidatedNtfns.add(uniqueNtfn);
        }

        for (var newNtfn in consolidatedNtfns) {
          var notification = newNtfn;
          _notificationService.flutterLocalNotificationsPlugin.show(
              notification.id,
              notification.title,
              notification.body,
              notification.notificationDetails,
              payload: notification.payload);
        }
        _notificationsToBeSent.clear();
      }
    });
  }

  /*
  Future<bool> isAndroidPermissionGranted() async {
    if (Platform.isAndroid) {
      bool granted = await flutterLocalNotificationsPlugin
              .resolvePlatformSpecificImplementation<
                  AndroidFlutterLocalNotificationsPlugin>()
              ?.areNotificationsEnabled() ??
          false;
      if (!granted) {
        granted = await Permission.notification.request().isGranted;
      }
      _notificationsGranted = granted;
      return granted;
    }
    return true;
  }
  */

  Future<void> requestPermissions() async {
    if (Platform.isIOS || Platform.isMacOS) {
      await flutterLocalNotificationsPlugin
          .resolvePlatformSpecificImplementation<
              IOSFlutterLocalNotificationsPlugin>()
          ?.requestPermissions(
            alert: true,
            badge: true,
            sound: true,
          );
      await flutterLocalNotificationsPlugin
          .resolvePlatformSpecificImplementation<
              MacOSFlutterLocalNotificationsPlugin>()
          ?.requestPermissions(
            alert: true,
            badge: true,
            sound: true,
          );
    } /* else if (Platform.isAndroid) {
      final AndroidFlutterLocalNotificationsPlugin? androidImplementation =
          flutterLocalNotificationsPlugin.resolvePlatformSpecificImplementation<
              AndroidFlutterLocalNotificationsPlugin>();

      final bool? grantedNotificationPermission =
          await androidImplementation?.requestNotificationsPermission();

      _notificationsGranted = grantedNotificationPermission ?? false;
    } */
  }

  // showChatNotification displays basic GC and PM notifications
  Future<void> showChatNotification(
      String message, String sender, bool isGC, String gcName) async {
    // If notifications aren't enabled, just skip
    if (!await allowNotifications()) return;
    /*
    const AndroidNotificationDetails androidNotificationDetails =
        AndroidNotificationDetails(
            'BR Chat Notifications', 'Chat Notifications',
            channelDescription:
                'Alerts for received messages from users or group chats',
            importance: Importance.max,
            priority: Priority.high,
            ticker: 'ticker');
    */
    const LinuxNotificationDetails linuxNotificationDetails =
        LinuxNotificationDetails(
            actions: []); //LinuxNotificationAction(key: "key", label: "label")]);
    const NotificationDetails notificationDetails = NotificationDetails(
        /*android: androidNotificationDetails,*/ linux:
            linuxNotificationDetails);
    var ntfnTitle = sender;
    if (isGC) {
      ntfnTitle = "$sender ($gcName)";
    }
    var body = message;

    // Reset notification check timer
    notificationChecker.cancel;
    _startNotificationTimer();
    _notificationsToBeSent.add(Notification(
        id++,
        ntfnTitle,
        body,
        notificationDetails,
        isGC ? "chat gc nick:$gcName" : "chat nick:$sender"));
  }

  // showPostNotification is used when a new post is received from a subscribed
  // user.
  Future<void> showPostNotification(PostSummary post) async {
    // If notifications aren't enabled, just skip
    if (!await allowNotifications()) return;
    /*
    const AndroidNotificationDetails androidNotificationDetails =
        AndroidNotificationDetails('BR Post Notifcations', 'Post Notifications',
            channelDescription: 'Alerts for newly received posts in Feed',
            importance: Importance.max,
            priority: Priority.high,
            ticker: 'ticker');
    */
    const NotificationDetails notificationDetails =
        NotificationDetails(/*android: androidNotificationDetails*/);
    var ntfnTitle = "New Post by ${post.authorNick}";
    await _notificationService.flutterLocalNotificationsPlugin.show(
        id++, ntfnTitle, post.title, notificationDetails,
        payload: "post:${post.authorID}:${post.id}");
  }

  // showPostCommentNotification is used when a new comment on an already
  // received post is made.
  Future<void> showPostCommentNotification(
      PostSummary post, String sender, String comment) async {
    // If notifications aren't enabled, just skip
    if (!await allowNotifications()) return;
    /*
    const AndroidNotificationDetails androidNotificationDetails =
        AndroidNotificationDetails(
            'BR Comment Notifications', 'Comment Notifications',
            channelDescription: 'Alerts for new comments on posts in Feed',
            importance: Importance.max,
            priority: Priority.high,
            ticker: 'ticker');
    */
    const NotificationDetails notificationDetails =
        NotificationDetails(/*android: androidNotificationDetails*/);
    var ntfnTitle =
        "New Comment from $sender on '${post.title.substring(0, post.title.length < 20 ? post.title.length - 1 : 20)}...'";
    await _notificationService.flutterLocalNotificationsPlugin.show(
        id++, ntfnTitle, comment, notificationDetails,
        payload: "postcomment:${post.authorID}:${post.id}");
  }

  NotificationService._internal();
}

class ReceivedNotification {
  ReceivedNotification({
    required this.id,
    required this.title,
    required this.body,
    required this.payload,
  });

  final int id;
  final String? title;
  final String? body;
  final String? payload;
}
