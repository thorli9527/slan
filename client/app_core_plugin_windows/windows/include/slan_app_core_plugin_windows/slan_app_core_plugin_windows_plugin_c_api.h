#ifndef FLUTTER_PLUGIN_SLAN_APP_CORE_PLUGIN_WINDOWS_PLUGIN_C_API_H_
#define FLUTTER_PLUGIN_SLAN_APP_CORE_PLUGIN_WINDOWS_PLUGIN_C_API_H_

#include <flutter/plugin_registrar_windows.h>

#ifdef SLAN_APP_CORE_PLUGIN_WINDOWS_PLUGIN_IMPL
#define SLAN_APP_CORE_PLUGIN_WINDOWS_PLUGIN_EXPORT __declspec(dllexport)
#else
#define SLAN_APP_CORE_PLUGIN_WINDOWS_PLUGIN_EXPORT __declspec(dllimport)
#endif

#if defined(__cplusplus)
extern "C" {
#endif

SLAN_APP_CORE_PLUGIN_WINDOWS_PLUGIN_EXPORT void
SlanAppCorePluginWindowsPluginCApiRegisterWithRegistrar(
    FlutterDesktopPluginRegistrarRef registrar);

#if defined(__cplusplus)
}  // extern "C"
#endif

#endif  // FLUTTER_PLUGIN_SLAN_APP_CORE_PLUGIN_WINDOWS_PLUGIN_C_API_H_
