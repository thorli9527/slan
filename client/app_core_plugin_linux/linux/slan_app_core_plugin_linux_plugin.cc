#include "include/slan_app_core_plugin_linux/slan_app_core_plugin_linux_plugin.h"

#include <flutter_linux/flutter_linux.h>
#include <gtk/gtk.h>

#include <cstring>

struct _SlanAppCorePluginLinuxPlugin {
  GObject parent_instance;
};

G_DEFINE_TYPE(
    SlanAppCorePluginLinuxPlugin,
    slan_app_core_plugin_linux_plugin,
    g_object_get_type())

static void slan_app_core_plugin_linux_plugin_handle_method_call(
    SlanAppCorePluginLinuxPlugin* self,
    FlMethodCall* method_call) {
  const gchar* method = fl_method_call_get_name(method_call);
  g_autofree gchar* message = g_strdup_printf(
      "Linux host integration is not implemented yet for %s",
      method);
  g_autoptr(FlMethodResponse) response = FL_METHOD_RESPONSE(
      fl_method_error_response_new(
          "app_core_unsupported_platform",
          message,
          nullptr));

  fl_method_call_respond(method_call, response, nullptr);
}

static void slan_app_core_plugin_linux_plugin_dispose(GObject* object) {
  G_OBJECT_CLASS(slan_app_core_plugin_linux_plugin_parent_class)->dispose(object);
}

static void slan_app_core_plugin_linux_plugin_class_init(
    SlanAppCorePluginLinuxPluginClass* klass) {
  G_OBJECT_CLASS(klass)->dispose = slan_app_core_plugin_linux_plugin_dispose;
}

static void slan_app_core_plugin_linux_plugin_init(
    SlanAppCorePluginLinuxPlugin* self) {}

static void method_call_cb(FlMethodChannel* channel,
                           FlMethodCall* method_call,
                           gpointer user_data) {
  SlanAppCorePluginLinuxPlugin* plugin =
      SLAN_APP_CORE_PLUGIN_LINUX_PLUGIN(user_data);
  slan_app_core_plugin_linux_plugin_handle_method_call(plugin, method_call);
}

void slan_app_core_plugin_linux_plugin_register_with_registrar(
    FlPluginRegistrar* registrar) {
  SlanAppCorePluginLinuxPlugin* plugin = SLAN_APP_CORE_PLUGIN_LINUX_PLUGIN(
      g_object_new(slan_app_core_plugin_linux_plugin_get_type(), nullptr));

  g_autoptr(FlStandardMethodCodec) codec = fl_standard_method_codec_new();
  g_autoptr(FlMethodChannel) channel = fl_method_channel_new(
      fl_plugin_registrar_get_messenger(registrar),
      "slan/app_core",
      FL_METHOD_CODEC(codec));
  fl_method_channel_set_method_call_handler(
      channel,
      method_call_cb,
      g_object_ref(plugin),
      g_object_unref);

  g_object_unref(plugin);
}
