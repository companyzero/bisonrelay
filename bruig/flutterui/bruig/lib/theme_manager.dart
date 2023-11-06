import 'dart:io';

import 'package:flutter/material.dart';
import './storage_manager.dart';

//  key blue: 2970ff
//  key green: 2ed6a1
//  main dark blue: 091440
//  secondary blue: 70cbff
//  secondary green: 41bf53
//  secondary orange: ed6d47
//  fontFamily: "SourceCodePro"),
//  colorScheme: ColorScheme.fromSeed(seedColor: Color(0xff2970ff))),

class ThemeNotifier with ChangeNotifier {
  final double defaultFontSize = 1;
  static String emojifont =
      Platform.isWindows ? "notoemoji_win" : "notoemoji_unix";
  final darkTheme = ThemeData(
      fontFamily: "Inter",
      fontFamilyFallback: [emojifont],
      //primarySwatch: Colors.blue,
      primaryColor: Colors.black,
      brightness: Brightness.dark,
      backgroundColor: const Color(0xFF19172C),
      highlightColor: const Color(0xFF252438),
      dividerColor: const Color(0xFF8E8D98),
      canvasColor: const Color(0xFF05031A),
      cardColor: const Color(0xFF05031A),
      errorColor: Colors.red,
      focusColor: const Color(0xFFE4E3E6),
      hoverColor: const Color(0xFF121026),
      scaffoldBackgroundColor: const Color(0xFF19172C),
      bottomAppBarColor: const Color(0xFF0175CE),
      indicatorColor: const Color(0xFF5A5968),
      selectedRowColor: Colors.black38,
      shadowColor: const Color(0xFFE44B00),
      dialogBackgroundColor: const Color(0xFF3A384B),
      iconTheme: const IconThemeData(color: Color(0xFF8E8D98)),
      textTheme: const TextTheme(
          headline5: TextStyle(
            color: Colors.white,
            fontSize: 46,
            fontWeight: FontWeight.w800,
          ),
          bodyText1: TextStyle(color: Colors.white),
          bodyText2: TextStyle(
              color: Colors.black))); // USE WITH RANDOM BACKGROUND FOR POSTS

  final lightTheme = ThemeData(
      fontFamily: "Inter",
      fontFamilyFallback: [emojifont],
      //primarySwatch: Colors.blue,
      primaryColor: Colors.black,
      brightness: Brightness.dark,
      backgroundColor: const Color(0xFF19172C),
      highlightColor: const Color(0xFF252438),
      dividerColor: const Color(0xFF8E8D98),
      canvasColor: const Color(0xFF05031A),
      cardColor: const Color(0xFF05031A),
      errorColor: Colors.red,
      focusColor: const Color(0xFFE4E3E6),
      hoverColor: const Color(0xFF121026),
      scaffoldBackgroundColor: const Color(0xFF19172C),
      bottomAppBarColor: const Color(0xFF0175CE),
      indicatorColor: const Color(0xFF5A5968),
      selectedRowColor: Colors.black38,
      shadowColor: const Color(0xFFE44B00),
      dialogBackgroundColor: const Color(0xFF3A384B),
      iconTheme: const IconThemeData(color: Color(0xFF8E8D98)),
      textTheme: const TextTheme(
          headline5: TextStyle(
            color: Colors.white,
            fontSize: 46,
            fontWeight: FontWeight.w800,
          ),
          bodyText1: TextStyle(color: Colors.white),
          bodyText2: TextStyle(color: Colors.black)));

  late ThemeData _themeData = lightTheme;
  ThemeData getTheme() => _themeData;

  late double _fontSize = defaultFontSize;
  double getFontCoef() => _fontSize;

  double getSmallFont(BuildContext context) {
    var mediaQuery = MediaQuery.of(context);
    if (mediaQuery.size.width <= 500) {
      return mediaQuery.textScaleFactor * 13;
    } else {
      return _fontSize * 2 + 11;
    }
  }

  double getMediumFont(BuildContext context) {
    var mediaQuery = MediaQuery.of(context);
    if (mediaQuery.size.width <= 500) {
      return mediaQuery.textScaleFactor * 15;
    } else {
      return _fontSize * 2 + 13;
    }
  }

  double getLargeFont(BuildContext context) {
    var mediaQuery = MediaQuery.of(context);
    if (mediaQuery.size.width <= 500) {
      return mediaQuery.textScaleFactor * 20;
    } else {
      return _fontSize * 2 + 13;
    }
  }

  double getHugeFont(BuildContext context) {
    var mediaQuery = MediaQuery.of(context);
    if (mediaQuery.size.width <= 500) {
      return mediaQuery.textScaleFactor * 30;
    } else {
      return _fontSize * 2 + 26;
    }
  }

  ThemeNotifier() {
    StorageManager.readData('themeMode').then((value) {
      debugPrint('value read from storage: ${value.toString()}');
      var themeMode = value ?? 'light';
      if (themeMode == 'light') {
        _themeData = lightTheme;
      } else {
        debugPrint('setting dark theme');
        _themeData = darkTheme;
      }
      notifyListeners();
    });
    StorageManager.readData('fontCoef').then((value) {
      debugPrint('value read from storage: ${value.toString()}');
      _fontSize = double.parse(value ?? "1");
      notifyListeners();
    });
  }

  void setDarkMode() async {
    _themeData = darkTheme;
    StorageManager.saveData('themeMode', 'dark');
    notifyListeners();
  }

  void setLightMode() async {
    _themeData = lightTheme;
    StorageManager.saveData('themeMode', 'light');
    notifyListeners();
  }

  void setFontSize(double fs) async {
    _fontSize = fs;
    StorageManager.saveData('fontCoef', fs.toString());
    notifyListeners();
  }
}
