#ifndef FLUTTER_PLUGIN_CLIENT_CORE_PLUGIN_INTERNAL_H_
#define FLUTTER_PLUGIN_CLIENT_CORE_PLUGIN_INTERNAL_H_

#include <flutter/method_channel.h>
#include <flutter/plugin_registrar_windows.h>
#include <flutter/standard_method_codec.h>

#include <memory>
#include <string>

namespace client_core_plugin {

class ClientCorePlugin : public flutter::Plugin {
 public:
  static void RegisterWithRegistrar(flutter::PluginRegistrarWindows* registrar);

  ClientCorePlugin();
  ~ClientCorePlugin() override;

  ClientCorePlugin(const ClientCorePlugin&) = delete;
  ClientCorePlugin& operator=(const ClientCorePlugin&) = delete;

 private:
  void HandleMethodCall(
      const flutter::MethodCall<flutter::EncodableValue>& method_call,
      std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>> result);
};

}  // namespace client_core_plugin

#endif  // FLUTTER_PLUGIN_CLIENT_CORE_PLUGIN_INTERNAL_H_
