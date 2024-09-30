import 'dart:io';

import 'package:bruig/util.dart';
import 'package:flutter/material.dart';
import 'package:flutter_markdown/flutter_markdown.dart';
import 'package:provider/provider.dart';
import './storage_manager.dart';

enum TextSize {
  small, // label.medium (12pt).
  system, // Default text size when no size is specified.
  medium, // body.medium / label.large. (14 pt).
  large, // body.large / title.medium (16 pt).
  huge, // title.large (22 pt).
}

// fontSize returns the font size to use given the text size token. When null is
// passed to size, a null font size ("default" size) is returned.
double? fontSize(TextSize? size) {
  switch (size) {
    case TextSize.small:
      return 12;
    case TextSize.system:
      return null;
    case TextSize.medium:
      return 14;
    case TextSize.large:
      return 16;
    case TextSize.huge:
      return 22;
    case null:
      return null;
  }
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

  error, // Used when only the text is displayed as error.
  successOnSurface, // Used for displaying a successful message on top of a surface
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
  final Color successOnSurface;
  final Color selectedItemOnSurfaceListView;

  const CustomColors({
    this.sidebarDivider = Colors.black,
    this.successOnSurface = const Color(0xFF2D882D),
    this.selectedItemOnSurfaceListView = Colors.amber,
  });
}

class CustomTextStyles {
  // Used on the small "gc" indicator on the list of chats.
  final TextStyle chatListGcIndicator;

  // Used on the nick initial on CircleAvatar (when the avatar color is
  // light or dark).
  final TextStyle lightAvatarInitial;
  final TextStyle darkAvatarInitial;

  final TextStyle monospaced;

  const CustomTextStyles({
    this.chatListGcIndicator = const TextStyle(),
    this.lightAvatarInitial =
        const TextStyle(fontSize: 16, color: Color(0xFF0E0D0D)),
    this.darkAvatarInitial =
        const TextStyle(fontSize: 16, color: Color(0xC0FCFCFC)),
    this.monospaced = const TextStyle(fontFamily: "RobotoMono"),
  });
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
  final CustomTextStyles extraTextStyles;

  // Decoration used to fade the background in StartupScreen components.
  final BoxDecoration? startupScreenBoxDecoration;

  AppTheme({
    required this.key,
    required this.descr,
    required this.data,
    required this.extraColors,
    required this.extraTextStyles,
    this.startupScreenBoxDecoration,
  });

  factory AppTheme.empty() => AppTheme(
      key: "",
      descr: "",
      data: ThemeData(),
      extraColors: const CustomColors(),
      extraTextStyles: const CustomTextStyles());
}

final appThemes = {
  "dark": AppTheme(
      key: "dark",
      descr: "Dark Theme",
      data: ThemeData.from(
        // Base Material3 color scheme based on seed
        useMaterial3: true,
        textTheme: _interTextTheme,
        colorScheme: ColorScheme.fromSeed(
          seedColor: const Color(0xFF19172C),
          brightness: Brightness.dark,

          // Color scheme customizations
          onSurfaceVariant: Colors.grey[600],
          // ignore: deprecated_member_use
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
          iconColor: const Color(0xFFe5e1e9), // onSurface
        ),

        hintColor: const Color(0xFF47464f), // onSurfaceVariant
        appBarTheme: const AppBarTheme(
          backgroundColor: Color(0xFF19172C), // suface color.
          scrolledUnderElevation:
              0, // Disable the scroll shadow effect on appbar
        ),

        disabledColor: Colors.grey[850],
      ),
      extraColors: const CustomColors(),
      extraTextStyles: const CustomTextStyles(
        chatListGcIndicator: TextStyle(
          fontStyle: FontStyle.italic,
          color: Color(0xFF47464f), // onSurfaceVariant
        ),
      ),
      startupScreenBoxDecoration: BoxDecoration(
          gradient: LinearGradient(
        begin: Alignment.bottomLeft,
        end: Alignment.topRight,
        colors: [
          const Color(0xFF19172C), // surface
          const Color(0xFF19172C).withOpacity(0.85),
          const Color(0xFF19172C).withOpacity(0.34),
        ],
        stops: const [0, 0.37, 1],
      ))),
  "light": AppTheme(
      key: "light",
      descr: "Light Theme",
      data: ThemeData.from(
        // Base Material3 color scheme based on seed
        useMaterial3: true,
        textTheme: _interBlackTextTheme,
        colorScheme: ColorScheme.fromSeed(
          seedColor: const Color(0xFFE8E7F3),
          brightness: Brightness.light,

          // Color scheme customizations
          onSurfaceVariant: Colors.grey[600],
          // ignore: deprecated_member_use
          background: const Color(
              0xFFE8E7F0), // Same as surface, will be removed in the future.
          surface: const Color(0xFFE8E7F3),
          surfaceContainerLow: const Color(0xFFE6E5F2),
          surfaceContainerLowest: const Color(0xFFE2E1ED),
        ),
      ).copyWith(
        // Bruig theme customizations.
        listTileTheme: ListTileThemeData(
          shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(3)),
          selectedTileColor: Colors.grey[100],
          iconColor: const Color(0xFF45464F), // onSurface
        ),

        hintColor: const Color(0xFF45464F), // onSurfaceVariant
        appBarTheme: const AppBarTheme(
          backgroundColor: Color(0xFFE8E7F3), // suface color.
          scrolledUnderElevation:
              0, // Disable the scroll shadow effect on appbar
        ),

        disabledColor: Colors.grey[850],
      ),
      extraColors: const CustomColors(
        sidebarDivider: Colors.white,
        selectedItemOnSurfaceListView: Color(0xFFFF6F00),
      ),
      extraTextStyles: const CustomTextStyles(
        chatListGcIndicator: TextStyle(
          fontStyle: FontStyle.italic,
          color: Color(0xFF45464F), // onSurfaceVariant
        ),
      ),
      startupScreenBoxDecoration: BoxDecoration(
          gradient: LinearGradient(
        begin: Alignment.bottomLeft,
        end: Alignment.topRight,
        colors: [
          const Color(0xFFE8E7F3), // surface
          const Color(0xFFE8E7F3).withOpacity(0.95),
          const Color(0xFFE8E7F3).withOpacity(0.95),
        ],
        stops: const [0, 0.5, 1],
      ))),
  "system": AppTheme(
    key: "system",
    descr: "Use System Default",
    data: ThemeData(),
    extraColors: const CustomColors(),
    extraTextStyles: const CustomTextStyles(),
  ),
};

const _defaultThemeName = "dark"; // This MUST exist in the map above.
const double _defaultFontScale = 1;

class ThemeNotifier with ChangeNotifier {
  static ThemeNotifier of(BuildContext context, {bool listen = true}) =>
      Provider.of<ThemeNotifier>(context, listen: listen);

  late ThemeData _themeData = appThemes[_defaultThemeName]!.data;
  late AppTheme _fullTheme = appThemes[_defaultThemeName]!;

  ThemeData get theme => _themeData;
  AppTheme get fullTheme => _fullTheme;
  Brightness get brightness => theme.brightness;
  ColorScheme get colors => _themeData.colorScheme;

  CustomColors _extraColors = const CustomColors();
  CustomColors get extraColors => _extraColors;

  CustomTextStyles _extraTextStyles = const CustomTextStyles();
  CustomTextStyles get extraTextStyles => _extraTextStyles;

  late String _themeMode = "";
  String getThemeMode() => _themeMode;

  late double _fontScale = _defaultFontScale;
  double get fontScale => _fontScale;

  bool _themeLoaded = false;
  bool get themeLoaded => _themeLoaded;

  ThemeNotifier({doLoad = true}) {
    if (doLoad) _loadThemeFromConfig();
  }

  // newNotifierWhenLoaded returns a new ThemeNotifier only after it has finished
  // loading the theme data.
  static Future<ThemeNotifier> newNotifierWhenLoaded() async {
    var theme = ThemeNotifier(doLoad: false);
    try {
      await theme._loadThemeFromConfig();
    } catch (exception) {
      debugPrint("Error while loading theme: $exception");

      // Continue to return default theme.
    }
    return theme;
  }

  Future<void> _loadThemeFromConfig() async {
    var fontScaleCfg =
        await StorageManager.readData(StorageManager.fontScaleKey);
    _fontScale = double.parse(fontScaleCfg ?? "0");
    var themeModeCfg =
        await StorageManager.readData(StorageManager.themeModeKey);
    switchTheme(themeModeCfg ?? _defaultThemeName);
  }

  void switchTheme(String value) async {
    // When using the special theme "system", determine the theme based on
    // the platform brightness.
    String themeName = value;
    if (value == "system" && (Platform.isIOS || Platform.isAndroid)) {
      var brightness =
          WidgetsBinding.instance.platformDispatcher.platformBrightness;
      if (brightness == Brightness.dark) {
        themeName = "dark-m3";
      } else {
        themeName = "light-m3";
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
    _extraTextStyles = theme.extraTextStyles;
    _fullTheme = theme;
    await StorageManager.saveData(StorageManager.themeModeKey, value);
    _clearTxtStyleCache();
    _rebuildMarkdownStyleSheet();
    _themeLoaded = true;
    notifyListeners();
  }

  void setFontSize(double fs) async {
    _fontScale = fs;
    StorageManager.saveData(StorageManager.fontScaleKey, fs.toString());
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
    _nickTextStyles.clear();
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
      case TextColor.error:
        return colors.error;
      case TextColor.successOnSurface:
        return extraColors.successOnSurface;
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

    // Null font color means default/inherited color.
    var fontColor = color != null ? textColor(color) : null;

    // Cache to reuse.
    var ts = TextStyle(fontSize: fontSize(size), color: fontColor);
    _txtStyleCache[size]?[color] = ts;
    return ts;
  }

  final Map<String, TextStyle> _nickTextStyles = {};

  // textStyleForNick returns the text style to use for the given remote user
  // nick (nick will be the same color as the avatar color).
  TextStyle textStyleForNick(String nick) {
    var res = _nickTextStyles[nick];
    if (res != null) {
      return res;
    }

    var color = colorFromNick(nick, brightness);
    res = TextStyle(color: color);
    _nickTextStyles[nick] = res;
    return res;
  }

  MarkdownStyleSheet _mdStyleSheet = MarkdownStyleSheet();
  MarkdownStyleSheet get mdStyleSheet => _mdStyleSheet;
  void _rebuildMarkdownStyleSheet() {
    _mdStyleSheet = MarkdownStyleSheet(
      code: extraTextStyles.monospaced,
    );
    _mdStyleSheet.styles["pre"] = _mdStyleSheet.code;
  }
}
