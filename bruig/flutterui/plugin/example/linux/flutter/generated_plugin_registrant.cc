//
//  Generated file. Do not edit.
//

// clang-format off

#include "generated_plugin_registrant.h"

#include <golib_plugin/golib_plugin.h>

void fl_register_plugins(FlPluginRegistry* registry) {
  g_autoptr(FlPluginRegistrar) golib_plugin_registrar =
      fl_plugin_registry_get_registrar_for_plugin(registry, "GolibPlugin");
  golib_plugin_register_with_registrar(golib_plugin_registrar);
}
