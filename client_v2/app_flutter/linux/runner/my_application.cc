#include "my_application.h"

#include <flutter_linux/flutter_linux.h>
#include <cairo.h>
#ifdef GDK_WINDOWING_X11
#include <gdk/gdkx.h>
#endif
#include <netdb.h>
#include <sys/socket.h>
#include <unistd.h>

#include <cstring>
#include <string>

#include "flutter/generated_plugin_registrant.h"

struct _MyApplication {
  GtkApplication parent_instance;
  char** dart_entrypoint_arguments;
  GtkWindow* window;
  GtkStatusIcon* tray_icon;
  GtkWidget* tray_menu;
  GtkWidget* network_menu_item;
  gboolean tray_enabled;
  gboolean quit_from_tray;
};

G_DEFINE_TYPE(MyApplication, my_application, GTK_TYPE_APPLICATION)

struct TrayServiceState {
  bool reachable = false;
  bool signed_in = false;
  bool network_enabled = false;
  bool syncing = false;
  bool switch_enabled = false;
};

constexpr const char kTrayOpenTitle[] = "Open Client";
constexpr const char kTrayQuitTitle[] = "Quit";
constexpr const char kTrayTooltipUnavailable[] =
    "SLAN Client - Service unavailable";
constexpr const char kTrayTooltipEnabled[] =
    "SLAN Client - Network enabled";
constexpr const char kTrayTooltipDisabled[] =
    "SLAN Client - Network disabled";
constexpr const char kTrayTooltipSignedOut[] =
    "SLAN Client - Signed out";

static std::string service_host() {
  const gchar* env = g_getenv("SLAN_CLIENT_CORE_SERVICE_HOST");
  if (env != nullptr && std::strlen(env) > 0) {
    return env;
  }
  return "127.0.0.1:46392";
}

static bool split_host_port(const std::string& host_port, std::string* host, std::string* port) {
  const auto separator = host_port.rfind(':');
  if (separator == std::string::npos || separator == 0 || separator + 1 >= host_port.size()) {
    return false;
  }
  *host = host_port.substr(0, separator);
  *port = host_port.substr(separator + 1);
  return !host->empty() && !port->empty();
}

static bool send_service_command(const char* method, std::string* response) {
  std::string host;
  std::string port;
  if (!split_host_port(service_host(), &host, &port)) {
    return false;
  }
  addrinfo hints{};
  hints.ai_family = AF_UNSPEC;
  hints.ai_socktype = SOCK_STREAM;
  hints.ai_protocol = IPPROTO_TCP;
  addrinfo* resolved = nullptr;
  if (getaddrinfo(host.c_str(), port.c_str(), &hints, &resolved) != 0) {
    return false;
  }
  int fd = -1;
  for (addrinfo* current = resolved; current != nullptr; current = current->ai_next) {
    fd = socket(current->ai_family, current->ai_socktype, current->ai_protocol);
    if (fd < 0) {
      continue;
    }
    timeval timeout{};
    timeout.tv_sec = 2;
    setsockopt(fd, SOL_SOCKET, SO_SNDTIMEO, &timeout, sizeof(timeout));
    setsockopt(fd, SOL_SOCKET, SO_RCVTIMEO, &timeout, sizeof(timeout));
    if (connect(fd, current->ai_addr, current->ai_addrlen) == 0) {
      break;
    }
    close(fd);
    fd = -1;
  }
  freeaddrinfo(resolved);
  if (fd < 0) {
    return false;
  }
  const std::string request = std::string("{\"method\":\"") + method + "\",\"args\":{}}\n";
  send(fd, request.c_str(), request.size(), 0);
  shutdown(fd, SHUT_WR);
  char buffer[4096];
  while (true) {
    const ssize_t received = recv(fd, buffer, sizeof(buffer), 0);
    if (received <= 0) {
      break;
    }
    if (response != nullptr) {
      response->append(buffer, buffer + received);
    }
  }
  close(fd);
  return true;
}

static bool json_bool_field(const std::string& json, const char* field) {
  const std::string key = std::string("\"") + field + "\"";
  const auto key_pos = json.find(key);
  if (key_pos == std::string::npos) {
    return false;
  }
  const auto colon_pos = json.find(':', key_pos + key.size());
  if (colon_pos == std::string::npos) {
    return false;
  }
  const auto true_pos = json.find("true", colon_pos + 1);
  const auto false_pos = json.find("false", colon_pos + 1);
  return true_pos != std::string::npos && (false_pos == std::string::npos || true_pos < false_pos);
}

static TrayServiceState query_tray_service_state() {
  std::string response;
  TrayServiceState state{};
  state.reachable = send_service_command("localState", &response);
  if (!state.reachable) {
    return state;
  }
  state.signed_in = json_bool_field(response, "signedIn");
  state.network_enabled = json_bool_field(response, "networkEnabled");
  state.syncing = json_bool_field(response, "syncing");
  state.switch_enabled = json_bool_field(response, "switchEnabled");
  return state;
}

static void show_main_window(MyApplication* self) {
  if (self->window == nullptr) {
    return;
  }
  gtk_widget_show(GTK_WIDGET(self->window));
  gtk_window_present(self->window);
}

static void settings_menu_cb(GtkMenuItem* item, gpointer user_data) {
  (void)item;
  show_main_window(MY_APPLICATION(user_data));
}

static void refresh_tray_menu(MyApplication* self);
static bool tray_network_action_enabled(const TrayServiceState& state);
static const char* tray_tooltip(const TrayServiceState& state);

static void network_menu_cb(GtkMenuItem* item, gpointer user_data) {
  (void)item;
  MyApplication* self = MY_APPLICATION(user_data);
  const TrayServiceState state = query_tray_service_state();
  if (!tray_network_action_enabled(state)) {
    return;
  }
  send_service_command(state.network_enabled ? "localNetworkDeactivate" : "localNetworkActivate", nullptr);
  refresh_tray_menu(self);
}

static void quit_menu_cb(GtkMenuItem* item, gpointer user_data) {
  (void)item;
  MyApplication* self = MY_APPLICATION(user_data);
  self->quit_from_tray = TRUE;
  send_service_command("localNetworkShutdown", nullptr);
  g_application_quit(G_APPLICATION(self));
}

static gboolean window_delete_event_cb(GtkWidget* widget, GdkEvent* event, gpointer user_data) {
  (void)event;
  MyApplication* self = MY_APPLICATION(user_data);
  if (self->tray_enabled && !self->quit_from_tray) {
    gtk_widget_hide(widget);
    return TRUE;
  }
  return FALSE;
}

static GdkPixbuf* make_vl_tray_pixbuf(bool network_enabled, bool service_available);

static void refresh_tray_menu(MyApplication* self) {
  if (self->network_menu_item == nullptr) {
    return;
  }
  const TrayServiceState state = query_tray_service_state();
  const char* network_label = !state.signed_in
                                  ? "Sign in to enable network"
                                  : (state.network_enabled ? "Disable Network"
                                                           : "Enable Network");
  gtk_menu_item_set_label(GTK_MENU_ITEM(self->network_menu_item),
                          network_label);
  if (GTK_IS_CHECK_MENU_ITEM(self->network_menu_item)) {
    gtk_check_menu_item_set_active(
        GTK_CHECK_MENU_ITEM(self->network_menu_item), state.network_enabled);
  }
  gtk_widget_set_sensitive(self->network_menu_item,
                           tray_network_action_enabled(state));
  if (self->tray_icon != nullptr) {
    const char* tooltip = tray_tooltip(state);
    G_GNUC_BEGIN_IGNORE_DEPRECATIONS
    g_autoptr(GdkPixbuf) icon = make_vl_tray_pixbuf(state.network_enabled, state.reachable);
    gtk_status_icon_set_from_pixbuf(self->tray_icon, icon);
    gtk_status_icon_set_tooltip_text(self->tray_icon, tooltip);
    G_GNUC_END_IGNORE_DEPRECATIONS
  }
}

static bool tray_network_action_enabled(const TrayServiceState& state) {
  return state.signed_in && state.switch_enabled && !state.syncing;
}

static const char* tray_tooltip(const TrayServiceState& state) {
  if (!state.reachable) {
    return kTrayTooltipUnavailable;
  }
  if (state.network_enabled) {
    return kTrayTooltipEnabled;
  }
  if (state.signed_in) {
    return kTrayTooltipDisabled;
  }
  return kTrayTooltipSignedOut;
}

static void tray_popup_menu_cb(GtkStatusIcon* status_icon,
                               guint button,
                               guint activate_time,
                               gpointer user_data) {
  (void)status_icon;
  (void)button;
  (void)activate_time;
  MyApplication* self = MY_APPLICATION(user_data);
  refresh_tray_menu(self);
  gtk_menu_popup_at_pointer(GTK_MENU(self->tray_menu), nullptr);
}

static void tray_activate_cb(GtkStatusIcon* status_icon, gpointer user_data) {
  (void)status_icon;
  show_main_window(MY_APPLICATION(user_data));
}

static gboolean tray_enabled_from_env() {
  const gchar* value = g_getenv("SLAN_LINUX_TRAY_MODE");
  return value != nullptr && g_strcmp0(value, "enabled") == 0;
}

static GdkPixbuf* make_vl_tray_pixbuf(bool network_enabled, bool service_available) {
  constexpr int kSize = 32;
  cairo_surface_t* surface = cairo_image_surface_create(CAIRO_FORMAT_ARGB32, kSize, kSize);
  cairo_t* cr = cairo_create(surface);
  if (network_enabled) {
    cairo_set_source_rgb(cr, 0.04, 0.39, 0.95);
  } else if (service_available) {
    cairo_set_source_rgb(cr, 0.42, 0.47, 0.55);
  } else {
    cairo_set_source_rgb(cr, 0.82, 0.21, 0.19);
  }
  cairo_rectangle(cr, 0, 0, kSize, kSize);
  cairo_fill(cr);
  cairo_select_font_face(cr, "Sans", CAIRO_FONT_SLANT_NORMAL, CAIRO_FONT_WEIGHT_BOLD);
  cairo_set_font_size(cr, 16);
  cairo_text_extents_t extents{};
  cairo_text_extents(cr, "VL", &extents);
  cairo_set_source_rgb(cr, 1, 1, 1);
  cairo_move_to(cr, (kSize - extents.width) / 2 - extents.x_bearing, (kSize - extents.height) / 2 - extents.y_bearing);
  cairo_show_text(cr, "VL");
  cairo_destroy(cr);
  GdkPixbuf* pixbuf = gdk_pixbuf_get_from_surface(surface, 0, 0, kSize, kSize);
  cairo_surface_destroy(surface);
  return pixbuf;
}

static void install_tray(MyApplication* self) {
  if (!self->tray_enabled || self->tray_icon != nullptr) {
    return;
  }
  self->tray_menu = gtk_menu_new();
  GtkWidget* settings = gtk_menu_item_new_with_label(kTrayOpenTitle);
  self->network_menu_item = gtk_check_menu_item_new_with_label("Enable Network");
  GtkWidget* quit = gtk_menu_item_new_with_label(kTrayQuitTitle);
  gtk_menu_shell_append(GTK_MENU_SHELL(self->tray_menu), settings);
  gtk_menu_shell_append(GTK_MENU_SHELL(self->tray_menu), self->network_menu_item);
  gtk_menu_shell_append(GTK_MENU_SHELL(self->tray_menu), gtk_separator_menu_item_new());
  gtk_menu_shell_append(GTK_MENU_SHELL(self->tray_menu), quit);
  g_signal_connect(settings, "activate", G_CALLBACK(settings_menu_cb), self);
  g_signal_connect(self->network_menu_item, "activate", G_CALLBACK(network_menu_cb), self);
  g_signal_connect(quit, "activate", G_CALLBACK(quit_menu_cb), self);
  gtk_widget_show_all(self->tray_menu);
  refresh_tray_menu(self);

  G_GNUC_BEGIN_IGNORE_DEPRECATIONS
  self->tray_icon = gtk_status_icon_new_from_icon_name("network-offline");
  gtk_status_icon_set_title(self->tray_icon, "SLAN Client");
  gtk_status_icon_set_tooltip_text(self->tray_icon, "SLAN Client");
  gtk_status_icon_set_visible(self->tray_icon, TRUE);
  G_GNUC_END_IGNORE_DEPRECATIONS
  g_signal_connect(self->tray_icon, "popup-menu", G_CALLBACK(tray_popup_menu_cb), self);
  g_signal_connect(self->tray_icon, "activate", G_CALLBACK(tray_activate_cb), self);
}

// Called when first Flutter frame received.
static void first_frame_cb(MyApplication* self, FlView* view) {
  gtk_widget_show(gtk_widget_get_toplevel(GTK_WIDGET(view)));
}

// Implements GApplication::activate.
static void my_application_activate(GApplication* application) {
  MyApplication* self = MY_APPLICATION(application);
  GtkWindow* window =
      GTK_WINDOW(gtk_application_window_new(GTK_APPLICATION(application)));
  self->window = window;
  self->tray_enabled = tray_enabled_from_env();
  install_tray(self);
  g_signal_connect(window, "delete-event", G_CALLBACK(window_delete_event_cb), self);

  // Use a header bar when running in GNOME as this is the common style used
  // by applications and is the setup most users will be using (e.g. Ubuntu
  // desktop).
  // If running on X and not using GNOME then just use a traditional title bar
  // in case the window manager does more exotic layout, e.g. tiling.
  // If running on Wayland assume the header bar will work (may need changing
  // if future cases occur).
  gboolean use_header_bar = TRUE;
#ifdef GDK_WINDOWING_X11
  GdkScreen* screen = gtk_window_get_screen(window);
  if (GDK_IS_X11_SCREEN(screen)) {
    const gchar* wm_name = gdk_x11_screen_get_window_manager_name(screen);
    if (g_strcmp0(wm_name, "GNOME Shell") != 0) {
      use_header_bar = FALSE;
    }
  }
#endif
  if (use_header_bar) {
    GtkHeaderBar* header_bar = GTK_HEADER_BAR(gtk_header_bar_new());
    gtk_widget_show(GTK_WIDGET(header_bar));
    gtk_header_bar_set_title(header_bar, "SLAN Client");
    gtk_header_bar_set_show_close_button(header_bar, TRUE);
    gtk_window_set_titlebar(window, GTK_WIDGET(header_bar));
  } else {
    gtk_window_set_title(window, "SLAN Client");
  }

  gtk_window_set_default_size(window, 480, 190);
  gtk_window_set_resizable(window, FALSE);

  g_autoptr(FlDartProject) project = fl_dart_project_new();
  fl_dart_project_set_dart_entrypoint_arguments(
      project, self->dart_entrypoint_arguments);

  FlView* view = fl_view_new(project);
  GdkRGBA background_color;
  // Background defaults to black, override it here if necessary, e.g. #00000000
  // for transparent.
  gdk_rgba_parse(&background_color, "#000000");
  fl_view_set_background_color(view, &background_color);
  gtk_widget_show(GTK_WIDGET(view));
  gtk_container_add(GTK_CONTAINER(window), GTK_WIDGET(view));

  // Show the window when Flutter renders.
  // Requires the view to be realized so we can start rendering.
  g_signal_connect_swapped(view, "first-frame", G_CALLBACK(first_frame_cb),
                           self);
  gtk_widget_realize(GTK_WIDGET(view));

  fl_register_plugins(FL_PLUGIN_REGISTRY(view));

  gtk_widget_grab_focus(GTK_WIDGET(view));
}

// Implements GApplication::local_command_line.
static gboolean my_application_local_command_line(GApplication* application,
                                                  gchar*** arguments,
                                                  int* exit_status) {
  MyApplication* self = MY_APPLICATION(application);
  // Strip out the first argument as it is the binary name.
  self->dart_entrypoint_arguments = g_strdupv(*arguments + 1);

  g_autoptr(GError) error = nullptr;
  if (!g_application_register(application, nullptr, &error)) {
    g_warning("Failed to register: %s", error->message);
    *exit_status = 1;
    return TRUE;
  }

  g_application_activate(application);
  *exit_status = 0;

  return TRUE;
}

// Implements GApplication::startup.
static void my_application_startup(GApplication* application) {
  // MyApplication* self = MY_APPLICATION(object);

  // Perform any actions required at application startup.

  G_APPLICATION_CLASS(my_application_parent_class)->startup(application);
}

// Implements GApplication::shutdown.
static void my_application_shutdown(GApplication* application) {
  MyApplication* self = MY_APPLICATION(application);

  // Perform any actions required at application shutdown.
  G_GNUC_BEGIN_IGNORE_DEPRECATIONS
  if (self->tray_icon != nullptr) {
    gtk_status_icon_set_visible(self->tray_icon, FALSE);
    g_object_unref(self->tray_icon);
    self->tray_icon = nullptr;
  }
  G_GNUC_END_IGNORE_DEPRECATIONS

  G_APPLICATION_CLASS(my_application_parent_class)->shutdown(application);
}

// Implements GObject::dispose.
static void my_application_dispose(GObject* object) {
  MyApplication* self = MY_APPLICATION(object);
  g_clear_pointer(&self->dart_entrypoint_arguments, g_strfreev);
  G_OBJECT_CLASS(my_application_parent_class)->dispose(object);
}

static void my_application_class_init(MyApplicationClass* klass) {
  G_APPLICATION_CLASS(klass)->activate = my_application_activate;
  G_APPLICATION_CLASS(klass)->local_command_line =
      my_application_local_command_line;
  G_APPLICATION_CLASS(klass)->startup = my_application_startup;
  G_APPLICATION_CLASS(klass)->shutdown = my_application_shutdown;
  G_OBJECT_CLASS(klass)->dispose = my_application_dispose;
}

static void my_application_init(MyApplication* self) {}

MyApplication* my_application_new() {
  // Set the program name to the application ID, which helps various systems
  // like GTK and desktop environments map this running application to its
  // corresponding .desktop file. This ensures better integration by allowing
  // the application to be recognized beyond its binary name.
  g_set_prgname(APPLICATION_ID);

  return MY_APPLICATION(g_object_new(my_application_get_type(),
                                     "application-id", APPLICATION_ID, "flags",
                                     G_APPLICATION_NON_UNIQUE, nullptr));
}
