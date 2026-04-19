#ifndef FLUTTER_PLUGIN_SLAN_APP_CORE_PLUGIN_WINDOWS_PLUGIN_H_
#define FLUTTER_PLUGIN_SLAN_APP_CORE_PLUGIN_WINDOWS_PLUGIN_H_

#include <flutter/method_channel.h>
#include <flutter/plugin_registrar_windows.h>

#include <memory>

namespace slan_app_core_plugin_windows {

class SlanAppCorePluginWindowsPlugin : public flutter::Plugin {
 public:
  static void RegisterWithRegistrar(flutter::PluginRegistrarWindows* registrar);

  SlanAppCorePluginWindowsPlugin();
  ~SlanAppCorePluginWindowsPlugin() override;

  SlanAppCorePluginWindowsPlugin(const SlanAppCorePluginWindowsPlugin&) = delete;
  SlanAppCorePluginWindowsPlugin& operator=(
      const SlanAppCorePluginWindowsPlugin&) = delete;

 private:
  void HandleMethodCall(
      const flutter::MethodCall<flutter::EncodableValue>& method_call,
      std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>> result);
};

}  // namespace slan_app_core_plugin_windows

#endif  // FLUTTER_PLUGIN_SLAN_APP_CORE_PLUGIN_WINDOWS_PLUGIN_H_
