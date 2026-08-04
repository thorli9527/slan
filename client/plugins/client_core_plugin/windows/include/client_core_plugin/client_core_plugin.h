#ifndef FLUTTER_PLUGIN_CLIENT_CORE_PLUGIN_H_
#define FLUTTER_PLUGIN_CLIENT_CORE_PLUGIN_H_

#include <flutter_plugin_registrar.h>

#ifdef CLIENT_CORE_PLUGIN_IMPL
#define CLIENT_CORE_PLUGIN_EXPORT __declspec(dllexport)
#else
#define CLIENT_CORE_PLUGIN_EXPORT __declspec(dllimport)
#endif

#if defined(__cplusplus)
extern "C" {
#endif

CLIENT_CORE_PLUGIN_EXPORT void ClientCorePluginRegisterWithRegistrar(
    FlutterDesktopPluginRegistrarRef registrar);

#if defined(__cplusplus)
}  // extern "C"
#endif

#endif  // FLUTTER_PLUGIN_CLIENT_CORE_PLUGIN_H_
