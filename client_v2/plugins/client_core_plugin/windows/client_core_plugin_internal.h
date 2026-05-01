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

  flutter::EncodableMap StateAsMap() const;
  flutter::EncodableMap Dispatch(const flutter::EncodableValue* arguments);

  bool signed_in_ = false;
  std::string user_label_;
  std::string device_id_;
  std::string auth_callback_id_;
  std::string virtual_ip_;
  bool network_enabled_ = false;
  bool syncing_ = false;
  std::string sync_reason_;
  bool switch_enabled_ = true;
  std::string notice_;
  std::string error_;
};

}  // namespace client_core_plugin

#endif  // FLUTTER_PLUGIN_CLIENT_CORE_PLUGIN_INTERNAL_H_
