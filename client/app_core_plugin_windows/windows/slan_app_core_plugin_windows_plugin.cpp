#include "include/slan_app_core_plugin_windows/slan_app_core_plugin_windows_plugin.h"

#include <flutter/method_channel.h>
#include <flutter/plugin_registrar_windows.h>
#include <flutter/standard_method_codec.h>

#include <memory>
#include <sstream>

namespace slan_app_core_plugin_windows {

namespace {

flutter::EncodableMap UnsupportedActionResult(
    const std::string& action,
    const std::string& detail) {
  flutter::EncodableMap payload;
  payload[flutter::EncodableValue("action")] = flutter::EncodableValue(action);
  payload[flutter::EncodableValue("accepted")] = flutter::EncodableValue(false);
  payload[flutter::EncodableValue("phase")] =
      flutter::EncodableValue("failed");
  payload[flutter::EncodableValue("source")] =
      flutter::EncodableValue("windows-plugin");
  payload[flutter::EncodableValue("detail")] = flutter::EncodableValue(detail);
  payload[flutter::EncodableValue("connectionStatus")] =
      flutter::EncodableValue("unsupported");
  payload[flutter::EncodableValue("hasConfiguration")] =
      flutter::EncodableValue(false);
  payload[flutter::EncodableValue("runtimeState")] =
      flutter::EncodableValue("unsupported");
  payload[flutter::EncodableValue("backendState")] =
      flutter::EncodableValue("not-implemented");
  payload[flutter::EncodableValue("runtimeLastError")] =
      flutter::EncodableValue(detail);
  return payload;
}

}  // namespace

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
  const auto method_name = method_call.method_name();
  if (method_name == "tunnelRuntimeView") {
    result->Success(flutter::EncodableValue());
    return;
  }

  std::ostringstream message;
  message
      << "Windows control-plane flows are available, but the local tunnel backend "
      << "is not implemented yet for " << method_name
      << ". Wintun/driver integration is still pending.";
  result->Success(flutter::EncodableValue(
      UnsupportedActionResult(method_name, message.str())));
}

}  // namespace slan_app_core_plugin_windows
