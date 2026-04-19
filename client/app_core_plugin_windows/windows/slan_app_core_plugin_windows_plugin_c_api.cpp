#include "include/slan_app_core_plugin_windows/slan_app_core_plugin_windows_plugin_c_api.h"

#include <flutter/plugin_registrar_windows.h>

#include "include/slan_app_core_plugin_windows/slan_app_core_plugin_windows_plugin.h"

void SlanAppCorePluginWindowsPluginCApiRegisterWithRegistrar(
    FlutterDesktopPluginRegistrarRef registrar) {
  slan_app_core_plugin_windows::SlanAppCorePluginWindowsPlugin::RegisterWithRegistrar(
      flutter::PluginRegistrarManager::GetInstance()
          ->GetRegistrar<flutter::PluginRegistrarWindows>(registrar));
}
