#import "GolibPlugin.h"
#if __has_include(<golib_plugin/golib_plugin-Swift.h>)
#import <golib_plugin/golib_plugin-Swift.h>
#else
// Support project import fallback if the generated compatibility header
// is not copied when this plugin is created as a library.
// https://forums.swift.org/t/swift-static-libraries-dont-copy-generated-objective-c-header/19816
#import "golib_plugin-Swift.h"
#endif

@implementation GolibPlugin
+ (void)registerWithRegistrar:(NSObject<FlutterPluginRegistrar>*)registrar {
  [SwiftGolibPlugin registerWithRegistrar:registrar];
}
@end
