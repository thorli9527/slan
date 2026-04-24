#include "include/slan_app_core_plugin_linux/slan_app_core_plugin_linux_plugin.h"

#include <flutter_linux/flutter_linux.h>
#include <gio/gio.h>
#include <gtk/gtk.h>

#include <arpa/inet.h>
#include <netdb.h>
#include <sys/socket.h>
#include <unistd.h>

#include <cerrno>
#include <cstdio>
#include <cstring>
#include <sstream>
#include <string>
#include <vector>

namespace {

constexpr char kDefaultHelperHost[] = "127.0.0.1:46321";

struct HelperEndpoint {
  std::string address;
  std::string source;
  bool allow_start;
};

std::string EscapeJsonString(const std::string& value) {
  std::ostringstream escaped;
  for (const unsigned char ch : value) {
    switch (ch) {
      case '\\':
        escaped << "\\\\";
        break;
      case '"':
        escaped << "\\\"";
        break;
      case '\b':
        escaped << "\\b";
        break;
      case '\f':
        escaped << "\\f";
        break;
      case '\n':
        escaped << "\\n";
        break;
      case '\r':
        escaped << "\\r";
        break;
      case '\t':
        escaped << "\\t";
        break;
      default:
        if (ch < 0x20) {
          char buffer[7];
          std::snprintf(buffer, sizeof(buffer), "\\u%04x", ch);
          escaped << buffer;
        } else {
          escaped << static_cast<char>(ch);
        }
        break;
    }
  }
  return escaped.str();
}

std::string JsonFromFlValue(FlValue* value);

std::string JsonFromFlList(FlValue* value) {
  std::ostringstream json;
  json << "[";
  const size_t length = fl_value_get_length(value);
  for (size_t index = 0; index < length; ++index) {
    if (index > 0) {
      json << ",";
    }
    json << JsonFromFlValue(fl_value_get_list_value(value, index));
  }
  json << "]";
  return json.str();
}

std::string JsonFromFlMap(FlValue* value) {
  std::ostringstream json;
  json << "{";
  const size_t length = fl_value_get_length(value);
  bool first = true;
  for (size_t index = 0; index < length; ++index) {
    FlValue* key = fl_value_get_map_key(value, index);
    if (fl_value_get_type(key) != FL_VALUE_TYPE_STRING) {
      continue;
    }
    if (!first) {
      json << ",";
    }
    first = false;
    json << "\"" << EscapeJsonString(fl_value_get_string(key)) << "\":"
         << JsonFromFlValue(fl_value_get_map_value(value, index));
  }
  json << "}";
  return json.str();
}

std::string JsonFromFlValue(FlValue* value) {
  if (value == nullptr) {
    return "null";
  }
  switch (fl_value_get_type(value)) {
    case FL_VALUE_TYPE_NULL:
      return "null";
    case FL_VALUE_TYPE_BOOL:
      return fl_value_get_bool(value) ? "true" : "false";
    case FL_VALUE_TYPE_INT:
      return std::to_string(fl_value_get_int(value));
    case FL_VALUE_TYPE_FLOAT: {
      std::ostringstream json;
      json << fl_value_get_float(value);
      return json.str();
    }
    case FL_VALUE_TYPE_STRING:
      return "\"" + EscapeJsonString(fl_value_get_string(value)) + "\"";
    case FL_VALUE_TYPE_LIST:
      return JsonFromFlList(value);
    case FL_VALUE_TYPE_MAP:
      return JsonFromFlMap(value);
    default:
      return "null";
  }
}

std::string BuildRequestJson(const char* method, FlValue* arguments) {
  std::ostringstream request;
  request << "{\"method\":\"" << EscapeJsonString(method == nullptr ? "" : method)
          << "\",\"args\":";
  request << (arguments == nullptr ? "{}" : JsonFromFlValue(arguments));
  request << "}\n";
  return request.str();
}

HelperEndpoint ResolveHelperEndpoint() {
  const char* service_host = g_getenv("SLAN_APP_CORE_SERVICE_HOST");
  if (service_host != nullptr && std::strlen(service_host) > 0) {
    return {service_host, "SLAN_APP_CORE_SERVICE_HOST", false};
  }
  const char* helper_host = g_getenv("SLAN_APP_CORE_HELPER_HOST");
  if (helper_host != nullptr && std::strlen(helper_host) > 0) {
    return {helper_host, "SLAN_APP_CORE_HELPER_HOST", true};
  }
  return {kDefaultHelperHost, "default", true};
}

std::string ResolveHelperHost() {
  return ResolveHelperEndpoint().address;
}

std::string CurrentExecutableDirectory() {
  char buffer[4096];
  const ssize_t length = readlink("/proc/self/exe", buffer, sizeof(buffer) - 1);
  if (length <= 0) {
    return ".";
  }
  buffer[length] = '\0';
  std::string path(buffer);
  const auto separator = path.find_last_of('/');
  return separator == std::string::npos ? "." : path.substr(0, separator);
}

std::string ResolveHelperExecutablePath() {
  const char* configured = g_getenv("SLAN_APP_CORE_HELPER");
  if (configured != nullptr && std::strlen(configured) > 0) {
    return configured;
  }
  return CurrentExecutableDirectory() + "/app-core-helper";
}

bool SplitHostPort(const std::string& host_port, std::string* host, std::string* port) {
  std::string value = host_port;
  constexpr char kTcpPrefix[] = "tcp://";
  if (value.rfind(kTcpPrefix, 0) == 0) {
    value = value.substr(std::strlen(kTcpPrefix));
  }
  const auto separator = value.rfind(':');
  if (separator == std::string::npos || separator == 0 || separator == value.size() - 1) {
    return false;
  }
  *host = value.substr(0, separator);
  *port = value.substr(separator + 1);
  return true;
}

std::string HelperEndpointSummary() {
  const HelperEndpoint endpoint = ResolveHelperEndpoint();
  return "host=" + endpoint.address + ", source=" + endpoint.source +
         ", helper=" + ResolveHelperExecutablePath();
}

}  // namespace

struct _SlanAppCorePluginLinuxPlugin {
  GObject parent_instance;
  GMutex mutex;
  GSubprocess* helper_process;
  int socket_fd;
};

G_DEFINE_TYPE(
    SlanAppCorePluginLinuxPlugin,
    slan_app_core_plugin_linux_plugin,
    g_object_get_type())

static void slan_app_core_plugin_linux_plugin_reset_socket(
    SlanAppCorePluginLinuxPlugin* self) {
  if (self->socket_fd >= 0) {
    close(self->socket_fd);
    self->socket_fd = -1;
  }
}

static bool slan_app_core_plugin_linux_plugin_connect(
    SlanAppCorePluginLinuxPlugin* self,
    std::string* error_message) {
  if (self->socket_fd >= 0) {
    return true;
  }

  std::string host;
  std::string port;
  if (!SplitHostPort(ResolveHelperHost(), &host, &port)) {
    *error_message = "Invalid app-core helper host. Expected host:port. " +
                     HelperEndpointSummary();
    return false;
  }

  addrinfo hints {};
  hints.ai_family = AF_UNSPEC;
  hints.ai_socktype = SOCK_STREAM;
  addrinfo* resolved = nullptr;
  const int resolve_result = getaddrinfo(host.c_str(), port.c_str(), &hints, &resolved);
  if (resolve_result != 0) {
    *error_message = "Failed to resolve app-core helper host '" + host +
                     "': " + gai_strerror(resolve_result) + ". " +
                     HelperEndpointSummary();
    return false;
  }

  for (addrinfo* current = resolved; current != nullptr; current = current->ai_next) {
    const int candidate = socket(current->ai_family, current->ai_socktype, current->ai_protocol);
    if (candidate < 0) {
      continue;
    }
    if (connect(candidate, current->ai_addr, current->ai_addrlen) == 0) {
      self->socket_fd = candidate;
      freeaddrinfo(resolved);
      return true;
    }
    close(candidate);
  }

  freeaddrinfo(resolved);
  *error_message = "Failed to connect to app-core helper at " + host + ":" +
                   port + ". " + HelperEndpointSummary();
  return false;
}

static void slan_app_core_plugin_linux_plugin_clear_helper(
    SlanAppCorePluginLinuxPlugin* self) {
  if (self->helper_process != nullptr) {
    g_clear_object(&self->helper_process);
  }
}

static bool slan_app_core_plugin_linux_plugin_helper_exited(
    SlanAppCorePluginLinuxPlugin* self,
    std::string* exit_summary) {
  if (self->helper_process == nullptr ||
      !g_subprocess_get_if_exited(self->helper_process)) {
    return false;
  }
  *exit_summary = "app-core helper exited with status " +
                  std::to_string(g_subprocess_get_exit_status(
                      self->helper_process));
  return true;
}

static bool slan_app_core_plugin_linux_plugin_start_helper(
    SlanAppCorePluginLinuxPlugin* self,
    std::string* error_message) {
  std::string exit_summary;
  if (slan_app_core_plugin_linux_plugin_helper_exited(self, &exit_summary)) {
    slan_app_core_plugin_linux_plugin_clear_helper(self);
  } else if (self->helper_process != nullptr) {
    return true;
  }

  const std::string helper_path = ResolveHelperExecutablePath();
  const HelperEndpoint endpoint = ResolveHelperEndpoint();
  if (!endpoint.allow_start) {
    *error_message =
        "app-core service host is external; Linux plugin will not start a "
        "local helper for it. " +
        HelperEndpointSummary();
    return false;
  }
  if (!g_file_test(helper_path.c_str(), G_FILE_TEST_EXISTS)) {
    *error_message = "app-core helper executable was not found. " +
                     HelperEndpointSummary();
    return false;
  }
  if (!g_file_test(helper_path.c_str(), G_FILE_TEST_IS_EXECUTABLE)) {
    *error_message = "app-core helper is not executable. " +
                     HelperEndpointSummary();
    return false;
  }

  std::string host;
  std::string port;
  if (!SplitHostPort(ResolveHelperHost(), &host, &port)) {
    *error_message = "Invalid app-core helper host. Expected host:port. " +
                     HelperEndpointSummary();
    return false;
  }
  const std::string listen_address = host + ":" + port;

  g_autoptr(GError) error = nullptr;
  self->helper_process = g_subprocess_new(
      G_SUBPROCESS_FLAGS_NONE,
      &error,
      helper_path.c_str(),
      "--tcp-host",
      listen_address.c_str(),
      nullptr);
  if (self->helper_process == nullptr) {
    *error_message = "Failed to start app-core helper at " + helper_path + ": " +
                     (error == nullptr ? "unknown error" : error->message) +
                     ". " + HelperEndpointSummary();
    return false;
  }
  return true;
}

static bool slan_app_core_plugin_linux_plugin_ensure_connected(
    SlanAppCorePluginLinuxPlugin* self,
    std::string* error_message) {
  if (slan_app_core_plugin_linux_plugin_connect(self, error_message)) {
    return true;
  }
  if (!slan_app_core_plugin_linux_plugin_start_helper(self, error_message)) {
    return false;
  }
  for (int attempt = 0; attempt < 25; ++attempt) {
    if (slan_app_core_plugin_linux_plugin_connect(self, error_message)) {
      return true;
    }
    std::string exit_summary;
    if (slan_app_core_plugin_linux_plugin_helper_exited(self, &exit_summary)) {
      *error_message = exit_summary + ". " + HelperEndpointSummary();
      slan_app_core_plugin_linux_plugin_clear_helper(self);
      return false;
    }
    usleep(200 * 1000);
  }
  return false;
}

static bool slan_app_core_plugin_linux_plugin_write_all(
    SlanAppCorePluginLinuxPlugin* self,
    const std::string& request,
    std::string* error_message) {
  const char* current = request.data();
  size_t remaining = request.size();
  while (remaining > 0) {
    const ssize_t written = send(self->socket_fd, current, remaining, 0);
    if (written <= 0) {
      *error_message = "Failed to write request to app-core helper. " +
                       HelperEndpointSummary();
      return false;
    }
    current += written;
    remaining -= static_cast<size_t>(written);
  }
  return true;
}

static bool slan_app_core_plugin_linux_plugin_read_line(
    SlanAppCorePluginLinuxPlugin* self,
    std::string* response,
    std::string* error_message) {
  response->clear();
  char byte = 0;
  while (true) {
    const ssize_t count = recv(self->socket_fd, &byte, 1, 0);
    if (count <= 0) {
      *error_message =
          "app-core helper closed its TCP connection unexpectedly. " +
          HelperEndpointSummary();
      return false;
    }
    if (byte == '\n') {
      return true;
    }
    response->push_back(byte);
  }
}

static bool slan_app_core_plugin_linux_plugin_forward(
    SlanAppCorePluginLinuxPlugin* self,
    const char* method,
    FlValue* arguments,
    std::string* response,
    std::string* error_message) {
  const std::string request = BuildRequestJson(method, arguments);
  g_mutex_lock(&self->mutex);
  for (int attempt = 0; attempt < 2; ++attempt) {
    if (!slan_app_core_plugin_linux_plugin_ensure_connected(self, error_message)) {
      g_mutex_unlock(&self->mutex);
      return false;
    }
    if (!slan_app_core_plugin_linux_plugin_write_all(self, request, error_message)) {
      slan_app_core_plugin_linux_plugin_reset_socket(self);
      continue;
    }
    if (slan_app_core_plugin_linux_plugin_read_line(self, response, error_message)) {
      g_mutex_unlock(&self->mutex);
      return true;
    }
    slan_app_core_plugin_linux_plugin_reset_socket(self);
  }
  g_mutex_unlock(&self->mutex);
  return false;
}

static void slan_app_core_plugin_linux_plugin_handle_method_call(
    SlanAppCorePluginLinuxPlugin* self,
    FlMethodCall* method_call) {
  const gchar* method = fl_method_call_get_name(method_call);
  FlValue* arguments = fl_method_call_get_args(method_call);
  std::string response_body;
  std::string error_message;

  if (!slan_app_core_plugin_linux_plugin_forward(
          self, method, arguments, &response_body, &error_message)) {
    g_autoptr(FlMethodResponse) response = FL_METHOD_RESPONSE(
        fl_method_error_response_new(
            "app_core_process_start_failed",
            error_message.c_str(),
            nullptr));
    fl_method_call_respond(method_call, response, nullptr);
    return;
  }

  g_autoptr(FlValue) result = fl_value_new_string(response_body.c_str());
  g_autoptr(FlMethodResponse) response =
      FL_METHOD_RESPONSE(fl_method_success_response_new(result));
  fl_method_call_respond(method_call, response, nullptr);
}

static void slan_app_core_plugin_linux_plugin_dispose(GObject* object) {
  SlanAppCorePluginLinuxPlugin* self =
      SLAN_APP_CORE_PLUGIN_LINUX_PLUGIN(object);
  slan_app_core_plugin_linux_plugin_reset_socket(self);
  if (self->helper_process != nullptr) {
    g_subprocess_force_exit(self->helper_process);
    slan_app_core_plugin_linux_plugin_clear_helper(self);
  }
  G_OBJECT_CLASS(slan_app_core_plugin_linux_plugin_parent_class)->dispose(object);
}

static void slan_app_core_plugin_linux_plugin_finalize(GObject* object) {
  SlanAppCorePluginLinuxPlugin* self =
      SLAN_APP_CORE_PLUGIN_LINUX_PLUGIN(object);
  g_mutex_clear(&self->mutex);
  G_OBJECT_CLASS(slan_app_core_plugin_linux_plugin_parent_class)->finalize(object);
}

static void slan_app_core_plugin_linux_plugin_class_init(
    SlanAppCorePluginLinuxPluginClass* klass) {
  GObjectClass* object_class = G_OBJECT_CLASS(klass);
  object_class->dispose = slan_app_core_plugin_linux_plugin_dispose;
  object_class->finalize = slan_app_core_plugin_linux_plugin_finalize;
}

static void slan_app_core_plugin_linux_plugin_init(
    SlanAppCorePluginLinuxPlugin* self) {
  g_mutex_init(&self->mutex);
  self->helper_process = nullptr;
  self->socket_fd = -1;
}

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
