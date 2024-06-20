import 'dart:io';

import 'package:flutter/material.dart';
import './storage_manager.dart';

enum TextSize {
  small, // label.medium (12pt).
  system, // Default text size when no size is specified.
  medium, // body.medium / label.large. (14 pt).
  large, // body.large / title.medium (16 pt).
  huge, // title.large (22 pt).
}

enum TextColor {
  onPrimary,
  onSecondary,
  onTertiary,
  onError,

  onPrimaryContainer,
  onSecondaryContainer,
  onTertiaryContainer,
  onErrorContainer,

  onSurface, // Default text color.
  onSurfaceVariant,

  onInverseSurface,
  inversePrimary, // Used as text when the surface is inverseSurface.

  onPrimaryFixed,
  onPrimaryFixedVariant,
  onSecondaryFixed,
  onSecondaryFixedVariant,
  onTertiaryFixed,
  onTertiaryFixedVariant,
}

// SurfaceColor are colors used on surfaces/containers/background.
enum SurfaceColor {
  // Main component action colors.
  primary,
  secondary,
  tertiary,
  error,

  // Main container background colors.
  primaryContainer,
  secondaryContainer,
  tertiaryContainer,
  errorContainer,

  // Secondary surface background colors.
  surface, // Default background color.
  surfaceContainerLowest,
  surfaceContainerLow,
  surfaceContainer,
  surfaceContainerHigh,
  surfaceContainerHighest,

  // Surface variants.
  surfaceBright,
  surfaceDim,

  // Inverse colors.
  inverseSurface,
  inversePrimary,

  // These are rarely used.
  primaryFixed,
  primaryFixedDim,
  secondaryFixed,
  secondaryFixedDim,
  tertiaryFixed,
  tertiaryFixedDim,
}

class CustomColors {
  final Color sidebarDivider;

  CustomColors({this.sidebarDivider = Colors.black});
}

// Map between a background color token and its corresponding text color ("on"
// color).
final Map<SurfaceColor, TextColor> textColorForSurfaceColor = {
  SurfaceColor.primary: TextColor.onPrimary,
  SurfaceColor.secondary: TextColor.onSecondary,
  SurfaceColor.tertiary: TextColor.onTertiary,
  SurfaceColor.error: TextColor.onError,
  SurfaceColor.primaryContainer: TextColor.onPrimaryContainer,
  SurfaceColor.secondaryContainer: TextColor.onSecondaryContainer,
  SurfaceColor.tertiaryContainer: TextColor.onTertiaryContainer,
  SurfaceColor.errorContainer: TextColor.onErrorContainer,
  SurfaceColor.surface: TextColor.onSurface,
  SurfaceColor.surfaceContainerLowest: TextColor.onSurface,
  SurfaceColor.surfaceContainerLow: TextColor.onSurface,
  SurfaceColor.surfaceContainer: TextColor.onSurface,
  SurfaceColor.surfaceContainerHigh: TextColor.onSurface,
  SurfaceColor.surfaceContainerHighest: TextColor.onSurface,
  SurfaceColor.surfaceBright: TextColor.onSurface,
  SurfaceColor.surfaceDim: TextColor.onSurface,
  SurfaceColor.inverseSurface: TextColor.onInverseSurface,
  SurfaceColor.inversePrimary: TextColor.onSurface,
  SurfaceColor.primaryFixed: TextColor.onPrimaryFixed,
  SurfaceColor.primaryFixedDim: TextColor.onPrimaryFixed,
  SurfaceColor.secondaryFixed: TextColor.onSecondaryFixed,
  SurfaceColor.secondaryFixedDim: TextColor.onSecondaryFixed,
  SurfaceColor.tertiaryFixed: TextColor.onTertiaryFixed,
  SurfaceColor.tertiaryFixedDim: TextColor.onTertiaryFixed,
};

class AppFontSize {
  final String descr;
  final double scale;

  AppFontSize({required this.descr, required this.scale});
}

// Available global text rescaling factors.
final Map<String, AppFontSize> appFontSizes = {
  "system":
      AppFontSize(descr: "System default", scale: -1), // OS-level scaling.
  "xsmall": AppFontSize(descr: "Extra Small", scale: 0.65),
  "small": AppFontSize(descr: "Small", scale: 0.85),
  "medium": AppFontSize(descr: "Medium", scale: 1.15),
  "large": AppFontSize(descr: "Large", scale: 1.25),
  "xlarge": AppFontSize(descr: "Extra Large", scale: 1.5),
};

String appFontSizeKeyForScale(double scale) {
  var key = "system";
  appFontSizes.forEach((k, v) {
    if (v.scale == scale) {
      key = k;
    }
  });
  return key;
}

String _emojifont = Platform.isWindows ? "notoemoji_win" : "notoemoji_unix";

final TextTheme _interTextTheme = TextTheme(
  displayLarge: TextStyle(
      debugLabel: 'interdisplayLarge',
      fontFamily: 'Inter',
      decoration: TextDecoration.none,
      color: Colors.white70,
      fontFamilyFallback: [_emojifont]),
  displayMedium: TextStyle(
      debugLabel: 'interdisplayMedium',
      fontFamily: 'Inter',
      decoration: TextDecoration.none,
      color: Colors.white70,
      fontFamilyFallback: [_emojifont]),
  displaySmall: TextStyle(
      debugLabel: 'interdisplaySmall',
      fontFamily: 'Inter',
      decoration: TextDecoration.none,
      color: Colors.white70,
      fontFamilyFallback: [_emojifont]),
  headlineLarge: TextStyle(
      debugLabel: 'interheadlineLarge',
      fontFamily: 'Inter',
      decoration: TextDecoration.none,
      color: Colors.white70,
      fontFamilyFallback: [_emojifont]),
  headlineMedium: TextStyle(
      debugLabel: 'interheadlineMedium',
      fontFamily: 'Inter',
      decoration: TextDecoration.none,
      color: Colors.white70,
      fontFamilyFallback: [_emojifont]),
  headlineSmall: TextStyle(
      debugLabel: 'interheadlineSmall',
      fontFamily: 'Inter',
      decoration: TextDecoration.none,
      color: Colors.white70,
      fontFamilyFallback: [_emojifont]),
  titleLarge: TextStyle(
      debugLabel: 'intertitleLarge',
      fontFamily: 'Inter',
      decoration: TextDecoration.none,
      color: Colors.white70,
      fontFamilyFallback: [_emojifont]),
  titleMedium: TextStyle(
      debugLabel: 'intertitleMedium',
      fontFamily: 'Inter',
      decoration: TextDecoration.none,
      color: Colors.white70,
      fontFamilyFallback: [_emojifont]),
  titleSmall: TextStyle(
      debugLabel: 'intertitleSmall',
      fontFamily: 'Inter',
      decoration: TextDecoration.none,
      color: Colors.white70,
      fontFamilyFallback: [_emojifont]),
  bodyLarge: TextStyle(
      debugLabel: 'interbodyLarge',
      fontFamily: 'Inter',
      decoration: TextDecoration.none,
      color: Colors.white70,
      fontFamilyFallback: [_emojifont]),
  bodyMedium: TextStyle(
      debugLabel: 'interbodyMedium',
      fontFamily: 'Inter',
      decoration: TextDecoration.none,
      color: Colors.white70,
      fontFamilyFallback: [_emojifont]),
  bodySmall: TextStyle(
      debugLabel: 'interbodySmall',
      fontFamily: 'Inter',
      decoration: TextDecoration.none,
      color: Colors.white70,
      fontFamilyFallback: [_emojifont]),
  labelLarge: TextStyle(
      debugLabel: 'interlabelLarge',
      fontFamily: 'Inter',
      decoration: TextDecoration.none,
      color: Colors.white70,
      fontFamilyFallback: [_emojifont]),
  labelMedium: TextStyle(
      debugLabel: 'interlabelMedium',
      fontFamily: 'Inter',
      decoration: TextDecoration.none,
      color: Colors.white70,
      fontFamilyFallback: [_emojifont]),
  labelSmall: TextStyle(
      debugLabel: 'interlabelSmall',
      fontFamily: 'Inter',
      decoration: TextDecoration.none,
      color: Colors.white70,
      fontFamilyFallback: [_emojifont]),
);

final TextTheme _interBlackTextTheme = TextTheme(
  displayLarge: TextStyle(
      debugLabel: 'interdisplayLarge',
      fontFamily: 'Inter',
      decoration: TextDecoration.none,
      color: Colors.black54,
      fontFamilyFallback: [_emojifont]),
  displayMedium: TextStyle(
      debugLabel: 'interdisplayMedium',
      fontFamily: 'Inter',
      decoration: TextDecoration.none,
      color: Colors.black54,
      fontFamilyFallback: [_emojifont]),
  displaySmall: TextStyle(
      debugLabel: 'interdisplaySmall',
      fontFamily: 'Inter',
      decoration: TextDecoration.none,
      color: Colors.black54,
      fontFamilyFallback: [_emojifont]),
  headlineLarge: TextStyle(
      debugLabel: 'interheadlineLarge',
      fontFamily: 'Inter',
      decoration: TextDecoration.none,
      color: Colors.black54,
      fontFamilyFallback: [_emojifont]),
  headlineMedium: TextStyle(
      debugLabel: 'interheadlineMedium',
      fontFamily: 'Inter',
      decoration: TextDecoration.none,
      color: Colors.black54,
      fontFamilyFallback: [_emojifont]),
  headlineSmall: TextStyle(
      debugLabel: 'interheadlineSmall',
      fontFamily: 'Inter',
      decoration: TextDecoration.none,
      color: Colors.black87,
      fontFamilyFallback: [_emojifont]),
  titleLarge: TextStyle(
      debugLabel: 'intertitleLarge',
      fontFamily: 'Inter',
      decoration: TextDecoration.none,
      color: Colors.black87,
      fontFamilyFallback: [_emojifont]),
  titleMedium: TextStyle(
      debugLabel: 'intertitleMedium',
      fontFamily: 'Inter',
      decoration: TextDecoration.none,
      color: Colors.black87,
      fontFamilyFallback: [_emojifont]),
  titleSmall: TextStyle(
      debugLabel: 'intertitleSmall',
      fontFamily: 'Inter',
      decoration: TextDecoration.none,
      color: Colors.black87,
      fontFamilyFallback: [_emojifont]),
  bodyLarge: TextStyle(
      debugLabel: 'interbodyLarge',
      fontFamily: 'Inter',
      decoration: TextDecoration.none,
      color: Colors.black87,
      fontFamilyFallback: [_emojifont]),
  bodyMedium: TextStyle(
      debugLabel: 'interbodyMedium',
      fontFamily: 'Inter',
      decoration: TextDecoration.none,
      color: Colors.black87,
      fontFamilyFallback: [_emojifont]),
  bodySmall: TextStyle(
      debugLabel: 'interbodySmall',
      fontFamily: 'Inter',
      decoration: TextDecoration.none,
      color: Colors.black87,
      fontFamilyFallback: [_emojifont]),
  labelLarge: TextStyle(
      debugLabel: 'interlabelLarge',
      fontFamily: 'Inter',
      decoration: TextDecoration.none,
      color: Colors.black87,
      fontFamilyFallback: [_emojifont]),
  labelMedium: TextStyle(
      debugLabel: 'interlabelMedium',
      fontFamily: 'Inter',
      decoration: TextDecoration.none,
      color: Colors.black87,
      fontFamilyFallback: [_emojifont]),
  labelSmall: TextStyle(
      debugLabel: 'interlabelSmall',
      fontFamily: 'Inter',
      decoration: TextDecoration.none,
      color: Colors.black87,
      fontFamilyFallback: [_emojifont]),
);

class AppTheme {
  final String key;
  final String descr;
  final ThemeData data;
  final CustomColors extraColors;

  AppTheme(
      {required this.key,
      required this.descr,
      required this.data,
      required this.extraColors});

  factory AppTheme.empty() => AppTheme(
      key: "", descr: "", data: ThemeData(), extraColors: CustomColors());
}

final appThemes = {
  "dark": AppTheme(
    key: "dark",
    descr: "Dark Theme",
    data: ThemeData(
      fontFamily: "Inter",
      fontFamilyFallback: [_emojifont],
      primarySwatch: Colors.blue,
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
      // bottomAppBarColor: const Color(0xFF0175CE),
      indicatorColor: const Color(0xFF5A5968),
      shadowColor: const Color(0xFFE44B00),
      dialogBackgroundColor: const Color(0xFF3A384B),
      iconTheme: const IconThemeData(color: Color(0xFF8E8D98)),
      primaryColorDark: Color(0xC0FCFCFC),
      primaryColorLight: Color(0xFF0E0D0D),
      // textTheme: const TextTheme(
      // headline5: TextStyle(
      //   color: Colors.white,
      //   fontSize: 46,
      //   fontWeight: FontWeight.w800,
      // ),
      // bodyText1: TextStyle(color: Colors.white),
      // bodyText2: TextStyle(
      //     color:
      //         Colors.black)))), // USE WITH RANDOM BACKGROUND FOR POSTS
    ),
    extraColors: CustomColors(),
  ),
  "light": AppTheme(
    key: "light",
    descr: "Light Theme",
    data: ThemeData(
      fontFamily: "Inter",
      fontFamilyFallback: [_emojifont],
      primarySwatch: Colors.blue,
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
      // bottomAppBarColor: const Color(0xFF0175CE),
      indicatorColor: const Color(0xFF5A5968),
      shadowColor: const Color(0xFFE44B00),
      dialogBackgroundColor: const Color(0xFFC6C5CF),
      primaryColorDark: Color(0xC0FCFCFC),
      primaryColorLight: Color(0xFF0E0D0D),
      iconTheme: const IconThemeData(color: Color.fromARGB(255, 101, 100, 110)),
      // textTheme: const TextTheme(
      //     headline5: TextStyle(
      //       color: Colors.black,
      //       fontSize: 46,
      //       fontWeight: FontWeight.w800,
      //     ),
      //     bodyText1: TextStyle(color: Colors.black),
      //     bodyText2: TextStyle(color: Colors.white)),
    ),
    extraColors: CustomColors(),
  ),
  "dark-m3": AppTheme(
    key: "dark-m3",
    descr: "Dark (Material 3)",
    data: ThemeData.from(
      // Base Material3 color scheme based on seed
      useMaterial3: true,
      textTheme: _interTextTheme,
      colorScheme: ColorScheme.fromSeed(
        seedColor: const Color(0xFF19172C),
        brightness: Brightness.dark,

        // Color scheme customizations
        onSurfaceVariant: Colors.grey[600],
        background: const Color(
            0xFF19172C), // Same as surface, will be removed in the future.
        surface: const Color(0xFF19172C),
        surfaceContainerLow: const Color(0xFF17152A),
        surfaceContainerLowest: const Color(0xFF161429),
      ),
    ).copyWith(
      // Bruig theme customizations.
      listTileTheme: ListTileThemeData(
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(3)),
        selectedTileColor: Colors.grey[850],
        // tileColor: Colors.amber,
        iconColor:
            const Color(0xFFe5e1e9), // Same color as onSurface by default.
        // iconColor: Colors.grey[600],
      ),
    ),
    extraColors: CustomColors(),
  ),
  "light-m3": AppTheme(
    key: "light-m3",
    descr: "Light (Material 3)",
    data: ThemeData.from(
      // Base Material3 color scheme based on seed
      useMaterial3: true,
      textTheme: _interBlackTextTheme,
      colorScheme: ColorScheme.fromSeed(
        seedColor: const Color(0xFF19172C),
        brightness: Brightness.light,
      ),
    ).copyWith(
      // Bruig theme customizations.
      listTileTheme: ListTileThemeData(
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(3)),
        selectedTileColor: Colors.grey[100],
      ),
    ),
    extraColors: CustomColors(),
  ),
  "system": AppTheme(
    key: "system",
    descr: "Use System Default",
    data: ThemeData(),
    extraColors: CustomColors(),
  ),
};

const _defaultThemeName = "light"; // This MUST exist in the map above.

class ThemeNotifier with ChangeNotifier {
  final double defaultFontSize = 1;

  late ThemeData _themeData = appThemes[_defaultThemeName]!.data;

  @Deprecated("Use theme, colors or extraColors instead")
  ThemeData getTheme() => _themeData;
  ThemeData get theme => _themeData;

  ColorScheme get colors => _themeData.colorScheme;

  late CustomColors _extraColors;
  CustomColors get extraColors => _extraColors;

  late String _themeMode = "";
  String getThemeMode() => _themeMode;

  late double _fontSize = defaultFontSize;
  double getFontCoef() => _fontSize;

  double get fontScale => _fontSize;
  set fontScale(double v) {
    _fontSize = v;
    StorageManager.saveData('fontScale', v.toString());
    notifyListeners();
  }

  @Deprecated("Use Txt.S instead")
  double getSmallFont(BuildContext context) {
    if (MediaQuery.of(context).size.width <= 500) {
      return MediaQuery.textScalerOf(context).scale(12);
    } else {
      return ((_fontSize * .15) + 0.85) * 12;
    }
  }

  @Deprecated("Use Text or Txt.M instead")
  double getMediumFont(BuildContext context) {
    if (MediaQuery.of(context).size.width <= 500) {
      return MediaQuery.textScalerOf(context).scale(15);
    } else {
      return ((_fontSize * .15) + 0.85) * 15;
    }
  }

  @Deprecated("Use Txt.L instead")
  double getLargeFont(BuildContext context) {
    if (MediaQuery.of(context).size.width <= 500) {
      return MediaQuery.textScalerOf(context).scale(20);
    } else {
      return ((_fontSize * .15) + 0.85) * 20;
    }
  }

  @Deprecated("Use Txt.H instead")
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
    StorageManager.readData('fontScale').then((value) {
      _fontSize = double.parse(value ?? "0");
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

    var theme = appThemes[themeName] ??
        appThemes[_defaultThemeName] ??
        AppTheme.empty();
    _themeData = theme.data;
    _themeMode = value;
    _extraColors = theme.extraColors;
    await StorageManager.saveData('themeMode', value);
    _clearTxtStyleCache();
    notifyListeners();
  }

  void setFontSize(double fs) async {
    _fontSize = fs;
    StorageManager.saveData('fontCoef', fs.toString());
    _clearTxtStyleCache();
    notifyListeners();
  }

  final Map<TextSize?, Map<TextColor?, TextStyle>> _txtStyleCache = {
    null: {},
    TextSize.small: {},
    TextSize.medium: {},
    TextSize.large: {},
    TextSize.huge: {}
  };

  void _clearTxtStyleCache() {
    _txtStyleCache.forEach((k, v) {
      v.clear();
    });
  }

  // surfaceColor returns the theme color for the given color token.
  Color surfaceColor(SurfaceColor color) {
    switch (color) {
      case SurfaceColor.primary:
        return colors.primary;
      case SurfaceColor.secondary:
        return colors.secondary;
      case SurfaceColor.tertiary:
        return colors.tertiary;
      case SurfaceColor.error:
        return colors.error;
      case SurfaceColor.primaryContainer:
        return colors.primaryContainer;
      case SurfaceColor.secondaryContainer:
        return colors.secondaryContainer;
      case SurfaceColor.tertiaryContainer:
        return colors.tertiaryContainer;
      case SurfaceColor.errorContainer:
        return colors.errorContainer;
      case SurfaceColor.surface:
        return colors.surface;
      case SurfaceColor.surfaceContainerLowest:
        return colors.surfaceContainerLowest;
      case SurfaceColor.surfaceContainerLow:
        return colors.surfaceContainerLow;
      case SurfaceColor.surfaceContainer:
        return colors.surfaceContainer;
      case SurfaceColor.surfaceContainerHigh:
        return colors.surfaceContainerHigh;
      case SurfaceColor.surfaceContainerHighest:
        return colors.surfaceContainerHighest;
      case SurfaceColor.surfaceBright:
        return colors.surfaceBright;
      case SurfaceColor.surfaceDim:
        return colors.surfaceDim;
      case SurfaceColor.inverseSurface:
        return colors.inverseSurface;
      case SurfaceColor.inversePrimary:
        return colors.inversePrimary;
      case SurfaceColor.primaryFixed:
        return colors.primaryFixed;
      case SurfaceColor.primaryFixedDim:
        return colors.primaryFixedDim;
      case SurfaceColor.secondaryFixed:
        return colors.secondaryFixed;
      case SurfaceColor.secondaryFixedDim:
        return colors.secondaryFixedDim;
      case SurfaceColor.tertiaryFixed:
        return colors.tertiaryFixed;
      case SurfaceColor.tertiaryFixedDim:
        return colors.tertiaryFixedDim;
    }
  }

  // textColor returns the theme color for the given text color token.
  Color textColor(TextColor color) {
    switch (color) {
      case TextColor.onPrimary:
        return colors.onPrimary;
      case TextColor.onSecondary:
        return colors.onSecondary;
      case TextColor.onTertiary:
        return colors.onTertiary;
      case TextColor.onError:
        return colors.onError;
      case TextColor.onPrimaryContainer:
        return colors.onPrimaryContainer;
      case TextColor.onSecondaryContainer:
        return colors.onSecondaryContainer;
      case TextColor.onTertiaryContainer:
        return colors.onTertiaryContainer;
      case TextColor.onErrorContainer:
        return colors.onErrorContainer;
      case TextColor.onSurface:
        return colors.onSurface;
      case TextColor.onSurfaceVariant:
        return colors.onSurfaceVariant;
      case TextColor.onInverseSurface:
        return colors.onInverseSurface;
      case TextColor.onPrimaryFixed:
        return colors.onPrimaryFixed;
      case TextColor.onPrimaryFixedVariant:
        return colors.onPrimaryFixedVariant;
      case TextColor.onSecondaryFixed:
        return colors.onSecondaryFixed;
      case TextColor.onSecondaryFixedVariant:
        return colors.onSecondaryFixedVariant;
      case TextColor.onTertiaryFixed:
        return colors.onTertiaryFixed;
      case TextColor.onTertiaryFixedVariant:
        return colors.onTertiaryFixedVariant;
      case TextColor.inversePrimary:
        return colors.inversePrimary;
    }
  }

  // colorOnSurface returns the color to use for text on a surface of the given
  // background color (the "onXXXX" color).
  Color colorOnSurface(SurfaceColor color) {
    var txtColor = textColorForSurfaceColor[color] ?? TextColor.onSurface;
    return textColor(txtColor);
  }

  // textStyleFor returns the cached text style for a text of the given size and
  // color.
  TextStyle? textStyleFor(
      BuildContext context, TextSize? size, TextColor? color) {
    // Null size and color means no style (i.e. inherited/default style).
    if (size == null && color == null) {
      return null;
    }

    // Already cached style.
    var cached = _txtStyleCache[size]?[color];
    if (cached != null) {
      return cached;
    }

    double? fontSize;
    switch (size) {
      case null:
        break;
      case TextSize.small:
        fontSize = 12;
        break;
      case TextSize.system:
        // Use system default size.
        break;
      case TextSize.medium:
        fontSize = 14;
        break;
      case TextSize.large:
        fontSize = 16;
        break;
      case TextSize.huge:
        fontSize = 22;
        break;
    }

    // Null font color means default/inherited color.
    var fontColor = color != null ? textColor(color) : null;

    // Cache to reuse.
    var ts = TextStyle(fontSize: fontSize, color: fontColor);
    _txtStyleCache[size]?[color] = ts;
    return ts;
  }
}
