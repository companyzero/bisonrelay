import 'dart:async';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_local_notifications/flutter_local_notifications.dart';
import 'package:golib_plugin/definitions.dart';
import 'package:bruig/storage_manager.dart';
import 'package:golib_plugin/golib_plugin.dart';

class NotificationService {
  // Singleton constructor.
  NotificationService._internal() {
    _handleToEmit();
  }
  static final NotificationService _notificationService =
      NotificationService._internal();
  factory NotificationService() {
    return _notificationService;
  }

  final bool _notificationsGranted = true;

  final FlutterLocalNotificationsPlugin flutterLocalNotificationsPlugin =
      FlutterLocalNotificationsPlugin();

  // Tracks whether the app is in background and should emit UI notifications
  // (they are not emitted if app is focused).
  bool appInBackground = false;

  final StreamController<String?> selectNotificationStream =
      StreamController<String?>.broadcast();

  /// A notification action which triggers a App navigation event
  final String navigationActionId = 'id_3';

  /// Defines a iOS/MacOS notification category for text input actions.
  final String darwinNotificationCategoryText = 'textCategory';

  /// Defines a iOS/MacOS notification category for plain actions.
  final String darwinNotificationCategoryPlain = 'plainCategory';

  int _id = 1;

  void _showNotification(UINotification n) async {
    if (!appInBackground) return; // Skip if app is focused.
    if (!await allowNotifications()) return;

    const LinuxNotificationDetails linuxDetails = LinuxNotificationDetails(
        actions: []); //LinuxNotificationAction(key: "key", label: "label")]);
    const NotificationDetails details =
        NotificationDetails(linux: linuxDetails);

    String title = "Bison Relay";
    String body = n.text;

    // Payload is used when the notification is tapped to know which screen to
    // open.
    String payload = "";
    switch (n.type) {
      case UINtfnPM:
        payload = "chat:${n.from}";
        break;
      case UINtfnGCM:
      case UINtfnGCMMention:
        payload = "gc:${n.from}";
        break;
    }

    flutterLocalNotificationsPlugin.show(
      _id++,
      title,
      body,
      details,
      payload: payload,
    );
  }

  void testNotification() {
    flutterLocalNotificationsPlugin.show(
      _id++,
      "Test Notification",
      "This is a test notification",
      const NotificationDetails(),
      payload: "",
    );
  }

  void _handleToEmit() async {
    // Android notifications emission is handled by the native plugin.
    if (Platform.isAndroid) return;
    if (Platform.isWindows) return; // Unsupported.

    var stream = Golib.uiNotifications();
    await for (var n in stream) {
      _showNotification(n);
    }
  }

  // Update the running config (on Golib) based on the user-selected app settings.
  void updateUIConfig() async {
    UINotificationsConfig cfg;
    if (!await StorageManager.readBool(StorageManager.notificationsKey,
        defaultVal: true)) {
      cfg = UINotificationsConfig.disabled();
    } else {
      cfg = UINotificationsConfig(
          await StorageManager.readBool(StorageManager.ntfnPMs,
              defaultVal: true),
          await StorageManager.readBool(StorageManager.ntfnGCMs),
          await StorageManager.readBool(StorageManager.ntfnGCMentions,
              defaultVal: true));
    }
    await Golib.updateUINotificationsCfg(cfg);
  }

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
    // Android notifications are done through the native plugin.
    if (Platform.isAndroid) return;

    // Windows is not supported. See https://github.com/MaikuB/flutter_local_notifications/issues/746.
    if (Platform.isWindows) return;

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

    final DarwinInitializationSettings initializationSettingsDarwin =
        DarwinInitializationSettings(
      requestAlertPermission: true,
      requestBadgePermission: true,
      requestSoundPermission: true,
      onDidReceiveLocalNotification:
          (int id, String? title, String? body, String? payload) async {},
      notificationCategories: darwinNotificationCategories,
    );
    final LinuxInitializationSettings initializationSettingsLinux =
        LinuxInitializationSettings(
      defaultActionName: 'Open notification',
      defaultIcon: AssetsLinuxIcon('assets/icons/app_icon.png'),
    );
    final InitializationSettings initializationSettings =
        InitializationSettings(
      iOS: initializationSettingsDarwin,
      macOS: initializationSettingsDarwin,
      linux: initializationSettingsLinux,
    );
    var initRes = await flutterLocalNotificationsPlugin.initialize(
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
          /*(Platform.isAndroid) ? null :*/ notificationTapBackground,
    );
    if (initRes != null && !initRes) {
      debugPrint("Unable to initialize local notifications plugin");
    }
  }
}
