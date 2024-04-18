import 'dart:io';

import 'package:shared_preferences/shared_preferences.dart';
import 'package:flutter/foundation.dart';

class StorageManager {
  static const String goProfilerEnabledKey = "goProfilerEnabled";
  static const String ntfnFgSvcKey = "foregroundService";
  static const String notificationsKey = "notifications";

  static Future<void> saveData(String key, dynamic value) async {
    final prefs = await SharedPreferences.getInstance();
    if (value is int) {
      prefs.setInt(key, value);
    } else if (value is String) {
      prefs.setString(key, value);
    } else if (value is bool) {
      prefs.setBool(key, value);
    } else {
      debugPrint("Invalid Type");
    }
  }

  static Future<dynamic> readData(String key) async {
    final prefs = await SharedPreferences.getInstance();
    dynamic obj = prefs.get(key);
    return obj;
  }

  static Future<bool> deleteData(String key) async {
    final prefs = await SharedPreferences.getInstance();
    return prefs.remove(key);
  }

  static Future<void> setupDefaults() async {
    if (Platform.isAndroid) {
      if ((await StorageManager.readData(StorageManager.ntfnFgSvcKey)
              as bool?) ==
          null) {
        await StorageManager.saveData(StorageManager.ntfnFgSvcKey, true);
      }
    }

    if ((await StorageManager.readData(StorageManager.notificationsKey)
            as bool?) ==
        null) {
      await StorageManager.saveData(StorageManager.notificationsKey, true);
    }
  }
}
