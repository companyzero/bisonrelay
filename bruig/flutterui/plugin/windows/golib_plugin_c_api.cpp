#include "include/golib_plugin/golib_plugin_c_api.h"

#include <flutter/plugin_registrar_windows.h>

#include "golib_plugin.h"

void GolibPluginCApiRegisterWithRegistrar(
    FlutterDesktopPluginRegistrarRef registrar) {
  golib_plugin::GolibPlugin::RegisterWithRegistrar(
      flutter::PluginRegistrarManager::GetInstance()
          ->GetRegistrar<flutter::PluginRegistrarWindows>(registrar));
}
