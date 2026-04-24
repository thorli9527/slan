#ifndef FLUTTER_PLUGIN_SLAN_APP_CORE_PLUGIN_WINDOWS_PLUGIN_H_
#define FLUTTER_PLUGIN_SLAN_APP_CORE_PLUGIN_WINDOWS_PLUGIN_H_

#include <flutter/method_channel.h>
#include <flutter/plugin_registrar_windows.h>

#include <memory>
#include <mutex>
#include <optional>
#include <string>

namespace slan_app_core_plugin_windows {

class AppCoreServiceBridgeClient;

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

  std::optional<std::string> ForwardToService(
      const std::string& method_name,
      const flutter::EncodableValue* arguments,
      std::string* error_message);

  std::mutex service_mutex_;
  std::unique_ptr<AppCoreServiceBridgeClient> service_;
};

}  // namespace slan_app_core_plugin_windows

#endif  // FLUTTER_PLUGIN_SLAN_APP_CORE_PLUGIN_WINDOWS_PLUGIN_H_
