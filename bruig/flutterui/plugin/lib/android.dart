import 'definitions.dart';
import 'all_platforms.dart';
import 'mobile.dart';

class AndroidPlugin extends PluginPlatform
    with ChanneledPlatform, BaseChanneledCalls, BaseMobilePlatform {
  String get minorPlatform => "android";
}
