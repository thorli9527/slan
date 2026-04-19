#include "include/slan_app_core_plugin_windows/slan_app_core_plugin_windows_plugin.h"

#include <flutter/method_channel.h>
#include <flutter/plugin_registrar_windows.h>
#include <flutter/standard_method_codec.h>

#include <memory>
#include <sstream>

namespace slan_app_core_plugin_windows {

void SlanAppCorePluginWindowsPlugin::RegisterWithRegistrar(
    flutter::PluginRegistrarWindows* registrar) {
  auto channel = std::make_unique<flutter::MethodChannel<flutter::EncodableValue>>(
      registrar->messenger(),
      "slan/app_core",
      &flutter::StandardMethodCodec::GetInstance());

  auto plugin = std::make_unique<SlanAppCorePluginWindowsPlugin>();

  channel->SetMethodCallHandler(
      [plugin_pointer = plugin.get()](
          const auto& call, auto result) {
        plugin_pointer->HandleMethodCall(call, std::move(result));
      });

  registrar->AddPlugin(std::move(plugin));
}

SlanAppCorePluginWindowsPlugin::SlanAppCorePluginWindowsPlugin() {}

SlanAppCorePluginWindowsPlugin::~SlanAppCorePluginWindowsPlugin() {}

void SlanAppCorePluginWindowsPlugin::HandleMethodCall(
    const flutter::MethodCall<flutter::EncodableValue>& method_call,
    std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>> result) {
  std::ostringstream message;
  message << "Windows host integration is not implemented yet for "
          << method_call.method_name();
  result->Error(
      "app_core_unsupported_platform",
      message.str());
}

}  // namespace slan_app_core_plugin_windows
