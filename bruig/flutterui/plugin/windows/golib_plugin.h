#ifndef FLUTTER_PLUGIN_GOLIB_PLUGIN_H_
#define FLUTTER_PLUGIN_GOLIB_PLUGIN_H_

#include <flutter/method_channel.h>
#include <flutter/plugin_registrar_windows.h>

#include <memory>

namespace golib_plugin {

class GolibPlugin : public flutter::Plugin {
 public:
  static void RegisterWithRegistrar(flutter::PluginRegistrarWindows *registrar);

  GolibPlugin();

  virtual ~GolibPlugin();

  // Disallow copy and assign.
  GolibPlugin(const GolibPlugin&) = delete;
  GolibPlugin& operator=(const GolibPlugin&) = delete;

 private:
  // Called when a method is called on this plugin's channel from Dart.
  void HandleMethodCall(
      const flutter::MethodCall<flutter::EncodableValue> &method_call,
      std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>> result);
};

}  // namespace golib_plugin

#endif  // FLUTTER_PLUGIN_GOLIB_PLUGIN_H_
