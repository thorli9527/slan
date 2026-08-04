#ifndef FLUTTER_PLUGIN_CLIENT_CORE_PLUGIN_H_
#define FLUTTER_PLUGIN_CLIENT_CORE_PLUGIN_H_

#include <flutter_linux/flutter_linux.h>

G_BEGIN_DECLS

#ifdef FLUTTER_PLUGIN_IMPL
#define FLUTTER_PLUGIN_EXPORT __attribute__((visibility("default")))
#else
#define FLUTTER_PLUGIN_EXPORT
#endif

typedef struct _ClientCorePlugin ClientCorePlugin;
typedef struct {
  GObjectClass parent_class;
} ClientCorePluginClass;

FLUTTER_PLUGIN_EXPORT GType client_core_plugin_get_type();

FLUTTER_PLUGIN_EXPORT void client_core_plugin_register_with_registrar(
    FlPluginRegistrar* registrar);

G_END_DECLS

#endif
