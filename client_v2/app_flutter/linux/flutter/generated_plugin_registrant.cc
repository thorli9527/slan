//
//  Generated file. Do not edit.
//

// clang-format off

#include "generated_plugin_registrant.h"

#include <client_core_plugin/client_core_plugin.h>

void fl_register_plugins(FlPluginRegistry* registry) {
  g_autoptr(FlPluginRegistrar) client_core_plugin_registrar =
      fl_plugin_registry_get_registrar_for_plugin(registry, "ClientCorePlugin");
  client_core_plugin_register_with_registrar(client_core_plugin_registrar);
}
