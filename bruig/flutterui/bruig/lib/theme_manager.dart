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
String _emojifont = Platform.isWindows ? "notoemoji_win" : "notoemoji_unix";

class AppTheme {
  final String key;
  final String descr;
  final ThemeData data;

  AppTheme({required this.key, required this.descr, required this.data});
}

final appThemes = {
  "dark": AppTheme(
      key: "dark",
      descr: "Dark Theme",
      data: ThemeData(
          fontFamily: "Inter",
          fontFamilyFallback: [_emojifont],
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
          shadowColor: const Color(0xFFE44B00),
          dialogBackgroundColor: const Color(0xFF3A384B),
          iconTheme: const IconThemeData(color: Color(0xFF8E8D98)),
          primaryColorDark: Color(0xC0FCFCFC),
          primaryColorLight: Color(0xFF0E0D0D),
          textTheme: const TextTheme(
              headline5: TextStyle(
                color: Colors.white,
                fontSize: 46,
                fontWeight: FontWeight.w800,
              ),
              bodyText1: TextStyle(color: Colors.white),
              bodyText2: TextStyle(
                  color:
                      Colors.black)))), // USE WITH RANDOM BACKGROUND FOR POSTS

  "light": AppTheme(
      key: "light",
      descr: "Light Theme",
      data: ThemeData(
          fontFamily: "Inter",
          fontFamilyFallback: [_emojifont],
          //primarySwatch: Colors.blue,
          primaryColor: Colors.white,
          brightness: Brightness.light,
          backgroundColor: const Color(0xFFE8E7F0),
          highlightColor: const Color(0xFFDFDFE9),
          dividerColor: const Color(0xFF68656E),
          canvasColor: const Color(0xFFEDEBF8),
          cardColor: const Color(0xFFEDEBF8),
          errorColor: Colors.red,
          focusColor: const Color(0xFF06030A),
          hoverColor: const Color(0xFFDFDDEC),
          scaffoldBackgroundColor: const Color(0xFFE8E7F3),
          bottomAppBarColor: const Color(0xFF0175CE),
          indicatorColor: const Color(0xFF5A5968),
          shadowColor: const Color(0xFFE44B00),
          dialogBackgroundColor: const Color(0xFFC6C5CF),
          primaryColorDark: Color(0xC0FCFCFC),
          primaryColorLight: Color(0xFF0E0D0D),
          iconTheme:
              const IconThemeData(color: Color.fromARGB(255, 101, 100, 110)),
          textTheme: const TextTheme(
              headline5: TextStyle(
                color: Colors.black,
                fontSize: 46,
                fontWeight: FontWeight.w800,
              ),
              bodyText1: TextStyle(color: Colors.black),
              bodyText2: TextStyle(color: Colors.white)))),

  "system":
      AppTheme(key: "system", descr: "Use System Default", data: ThemeData()),
};

const _defaultThemeName = "light"; // This MUST exist in the map above.

class ThemeNotifier with ChangeNotifier {
  final double defaultFontSize = 1;

  late ThemeData _themeData = appThemes[_defaultThemeName]!.data;
  ThemeData getTheme() => _themeData;

  late String _themeMode = "";
  String getThemeMode() => _themeMode;

  late double _fontSize = defaultFontSize;
  double getFontCoef() => _fontSize;

  double getSmallFont(BuildContext context) {
    if (MediaQuery.of(context).size.width <= 500) {
      return MediaQuery.textScalerOf(context).scale(12);
    } else {
      return ((_fontSize * .15) + 0.85) * 12;
    }
  }

  double getMediumFont(BuildContext context) {
    if (MediaQuery.of(context).size.width <= 500) {
      return MediaQuery.textScalerOf(context).scale(15);
    } else {
      return ((_fontSize * .15) + 0.85) * 15;
    }
  }

  double getLargeFont(BuildContext context) {
    if (MediaQuery.of(context).size.width <= 500) {
      return MediaQuery.textScalerOf(context).scale(20);
    } else {
      return ((_fontSize * .15) + 0.85) * 20;
    }
  }

  double getHugeFont(BuildContext context) {
    if (MediaQuery.of(context).size.width <= 500) {
      return MediaQuery.textScalerOf(context).scale(30);
    } else {
      return ((_fontSize * .15) + 0.85) * 30;
    }
  }

  ThemeNotifier() {
    StorageManager.readData('themeMode').then((value) {
      switchTheme(value);
    });
    StorageManager.readData('fontCoef').then((value) {
      _fontSize = double.parse(value ?? "1");
      notifyListeners();
    });
  }

  void switchTheme(String value) async {
    // When using the special theme "system", determine the theme based on
    // the platform brightness.
    String themeName = value;
    if (value == "system" && (Platform.isIOS || Platform.isAndroid)) {
      var brightness =
          WidgetsBinding.instance.platformDispatcher.platformBrightness;
      if (brightness == Brightness.dark) {
        themeName = "dark";
      } else {
        themeName = "light";
      }
    } else if (value == "system") {
      themeName = _defaultThemeName;
    }

    var td = appThemes[themeName]?.data ?? appThemes[_defaultThemeName]!.data;
    _themeData = td;
    _themeMode = value;
    await StorageManager.saveData('themeMode', value);
    _clearTxtStyleCache();
    notifyListeners();
  }

  void setFontSize(double fs) async {
    _fontSize = fs;
    StorageManager.saveData('fontCoef', fs.toString());
    notifyListeners();
  }
}
