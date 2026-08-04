#include "include/client_core_plugin/client_core_plugin_c_api.h"

#include <flutter/plugin_registrar_windows.h>

#include "client_core_plugin_internal.h"

void ClientCorePluginRegisterWithRegistrar(
    FlutterDesktopPluginRegistrarRef registrar) {
  client_core_plugin::ClientCorePlugin::RegisterWithRegistrar(
      flutter::PluginRegistrarManager::GetInstance()
          ->GetRegistrar<flutter::PluginRegistrarWindows>(registrar));
}
