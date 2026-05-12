#include "include/client_core_plugin/client_core_plugin.h"

#include <flutter_linux/flutter_linux.h>
#include <gio/gio.h>
#include <glib.h>

#include <algorithm>
#include <cctype>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <sstream>
#include <string>

#define CLIENT_CORE_PLUGIN(obj) \
  (G_TYPE_CHECK_INSTANCE_CAST((obj), client_core_plugin_get_type(), ClientCorePlugin))

struct _ClientCorePlugin {
  GObject parent_instance;
};

G_DEFINE_TYPE(ClientCorePlugin, client_core_plugin, g_object_get_type())

namespace {

constexpr char kChannelName[] = "dev.slan/client_core_v2";
constexpr char kDefaultServiceHost[] = "127.0.0.1:46392";

std::string EnvString(const char* name) {
  const char* value = std::getenv(name);
  return value == nullptr ? std::string() : std::string(value);
}

std::string ResolveServiceHost() {
  const auto value = EnvString("SLAN_CLIENT_CORE_SERVICE_HOST");
  return value.empty() ? kDefaultServiceHost : value;
}

std::string ResolveWebConsoleUrl() {
  const auto explicit_url = EnvString("SLAN_WEB_CONSOLE_URL");
  if (!explicit_url.empty()) {
    return explicit_url;
  }
  const auto control_url = EnvString("SLAN_CONTROL_BASE_URL");
  if (control_url.find("api.dev.staticlss.com") != std::string::npos) {
    return "http://web.dev.staticlss.com";
  }
  return "http://web.dev.staticlss.com";
}

bool IsUsableClientDeviceId(const std::string& device_id) {
  if (device_id.empty()) {
    return false;
  }
  std::string lower;
  lower.reserve(device_id.size());
  for (const unsigned char ch : device_id) {
    lower.push_back(static_cast<char>(std::tolower(ch)));
  }
  return lower != "authcallbackid" && lower != "windows-plugin-login" &&
         lower != "macos-plugin-login" && lower != "linux-plugin-login" &&
         lower.rfind("cb-", 0) != 0;
}

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

std::string JsonFromValue(FlValue* value);

std::string JsonFromList(FlValue* value) {
  std::ostringstream json;
  json << "[";
  const size_t length = fl_value_get_length(value);
  for (size_t index = 0; index < length; ++index) {
    if (index > 0) {
      json << ",";
    }
    json << JsonFromValue(fl_value_get_list_value(value, index));
  }
  json << "]";
  return json.str();
}

std::string JsonFromMap(FlValue* value) {
  std::ostringstream json;
  json << "{";
  bool first = true;
  const size_t length = fl_value_get_length(value);
  for (size_t index = 0; index < length; ++index) {
    FlValue* key_value = fl_value_get_map_key(value, index);
    if (fl_value_get_type(key_value) != FL_VALUE_TYPE_STRING) {
      continue;
    }
    FlValue* map_value = fl_value_get_map_value(value, index);
    if (!first) {
      json << ",";
    }
    first = false;
    json << "\"" << EscapeJsonString(fl_value_get_string(key_value)) << "\":"
         << JsonFromValue(map_value);
  }
  json << "}";
  return json.str();
}

std::string JsonFromValue(FlValue* value) {
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
      return JsonFromList(value);
    case FL_VALUE_TYPE_MAP:
      return JsonFromMap(value);
    default:
      return "null";
  }
}

std::string BuildRequest(const std::string& method, FlValue* arguments) {
  std::ostringstream request;
  request << "{\"method\":\"" << EscapeJsonString(method) << "\",\"args\":";
  request << (arguments == nullptr ? "{}" : JsonFromValue(arguments));
  request << "}\n";
  return request.str();
}

std::string ExtractJsonStringField(const std::string& json, const std::string& field_name) {
  const std::string marker = "\"" + field_name + "\":";
  const auto marker_pos = json.find(marker);
  if (marker_pos == std::string::npos) {
    return "";
  }
  auto value_pos = json.find('"', marker_pos + marker.size());
  if (value_pos == std::string::npos) {
    return "";
  }
  ++value_pos;
  std::string value;
  bool escaping = false;
  for (size_t index = value_pos; index < json.size(); ++index) {
    const char ch = json[index];
    if (escaping) {
      value.push_back(ch);
      escaping = false;
      continue;
    }
    if (ch == '\\') {
      escaping = true;
      continue;
    }
    if (ch == '"') {
      break;
    }
    value.push_back(ch);
  }
  return value;
}

std::string AppendQueryParam(const std::string& url, const std::string& key,
                             const std::string& value) {
  if (value.empty()) {
    return url;
  }
  const char separator = url.find('?') == std::string::npos ? '?' : '&';
  return url + separator + key + "=" + value;
}

void OpenUrl(const std::string& url) {
  GError* error = nullptr;
  if (!g_app_info_launch_default_for_uri(url.c_str(), nullptr, &error) && error != nullptr) {
    g_error_free(error);
  }
}

void OpenWebConsole(const std::string& callback_id = "",
                    const std::string& device_id = "",
                    const std::string& console_login_key = "") {
  std::string url = ResolveWebConsoleUrl();
  if (!callback_id.empty()) {
    url = AppendQueryParam(url, "auth", "login");
    url = AppendQueryParam(url, "callbackId", callback_id);
  }
  if (!console_login_key.empty()) {
    url = AppendQueryParam(url, "consoleLoginKey", console_login_key);
  }
  if (IsUsableClientDeviceId(device_id)) {
    url = AppendQueryParam(url, "deviceId", device_id);
  }
  url = AppendQueryParam(url, "clientPlatform", "linux");
  url = AppendQueryParam(url, "clientName", "SLAN Client V2");
  OpenUrl(url);
}

bool TryStartBundledService() {
  const auto service_path = EnvString("SLAN_CLIENT_CORE_SERVICE_BIN");
  if (!service_path.empty()) {
    gchar* argv[] = {const_cast<gchar*>(service_path.c_str()), nullptr};
    GError* error = nullptr;
    const bool ok = g_spawn_async(nullptr, argv, nullptr, G_SPAWN_SEARCH_PATH,
                                  nullptr, nullptr, nullptr, &error);
    if (error != nullptr) {
      g_error_free(error);
    }
    if (ok) {
      g_usleep(300000);
      return true;
    }
  }
  return false;
}

std::string ReadLine(GSocketConnection* connection) {
  GInputStream* input = g_io_stream_get_input_stream(G_IO_STREAM(connection));
  std::string response;
  char byte = 0;
  GError* error = nullptr;
  while (g_input_stream_read(input, &byte, 1, nullptr, &error) == 1) {
    if (byte == '\n') {
      break;
    }
    response.push_back(byte);
  }
  if (error != nullptr) {
    g_error_free(error);
  }
  return response;
}

std::string ForwardToService(const std::string& method, FlValue* arguments) {
  std::string service_host = ResolveServiceHost();
  const auto separator = service_host.rfind(':');
  if (separator == std::string::npos || separator == 0 ||
      separator == service_host.size() - 1) {
    return "";
  }
  const std::string host = service_host.substr(0, separator);
  const std::string port = service_host.substr(separator + 1);
  int port_value = 0;
  try {
    port_value = std::stoi(port);
  } catch (...) {
    return "";
  }
  if (port_value <= 0 || port_value > 65535) {
    return "";
  }

  GError* error = nullptr;
  GSocketClient* client = g_socket_client_new();
  g_socket_client_set_timeout(client, 2);
  GSocketConnection* connection = g_socket_client_connect_to_host(
      client, host.c_str(), static_cast<guint16>(port_value), nullptr, &error);
  g_object_unref(client);
  if (connection == nullptr) {
    if (error != nullptr) {
      g_error_free(error);
    }
    return "";
  }

  const std::string request = BuildRequest(method, arguments);
  GOutputStream* output = g_io_stream_get_output_stream(G_IO_STREAM(connection));
  g_output_stream_write_all(output, request.data(), request.size(), nullptr, nullptr, &error);
  if (error != nullptr) {
    g_error_free(error);
    g_object_unref(connection);
    return "";
  }
  g_output_stream_flush(output, nullptr, nullptr);
  auto response = ReadLine(connection);
  g_object_unref(connection);
  return response;
}

std::string ForwardToServiceWithAutoStart(const std::string& method, FlValue* arguments) {
  if (auto response = ForwardToService(method, arguments); !response.empty()) {
    return response;
  }
  if (!TryStartBundledService()) {
    return "";
  }
  for (int attempt = 0; attempt < 15; ++attempt) {
    if (auto response = ForwardToService(method, arguments); !response.empty()) {
      return response;
    }
    g_usleep(100000);
  }
  return "";
}

std::string ReadCommandType(FlValue* arguments) {
  if (arguments == nullptr || fl_value_get_type(arguments) != FL_VALUE_TYPE_MAP) {
    return "";
  }
  FlValue* value = fl_value_lookup_string(arguments, "type");
  if (value == nullptr || fl_value_get_type(value) != FL_VALUE_TYPE_STRING) {
    return "";
  }
  return fl_value_get_string(value);
}

void OpenAuthenticatedWebConsole() {
  if (auto response = ForwardToServiceWithAutoStart("consoleLoginKey", nullptr);
      !response.empty()) {
    OpenWebConsole("", ExtractJsonStringField(response, "deviceId"),
                   ExtractJsonStringField(response, "loginKey"));
    return;
  }
  OpenWebConsole();
}

void client_core_plugin_handle_method_call(ClientCorePlugin* self,
                                           FlMethodCall* method_call) {
  const gchar* method = fl_method_call_get_name(method_call);
  FlValue* args = fl_method_call_get_args(method_call);
  const auto command_type =
      std::strcmp(method, "dispatch") == 0 ? ReadCommandType(args) : "";
  const auto response = ForwardToServiceWithAutoStart(method, args);
  if (!response.empty()) {
    if (command_type == "openWebConsole") {
      OpenAuthenticatedWebConsole();
    } else if (command_type == "loginWithBrowser") {
      OpenWebConsole(ExtractJsonStringField(response, "authCallbackId"),
                     ExtractJsonStringField(response, "deviceId"));
    }
    g_autoptr(FlValue) result = fl_value_new_string(response.c_str());
    fl_method_call_respond_success(method_call, result, nullptr);
    return;
  }
  fl_method_call_respond_not_implemented(method_call, nullptr);
}

}  // namespace

static void client_core_plugin_dispose(GObject* object) {
  G_OBJECT_CLASS(client_core_plugin_parent_class)->dispose(object);
}

static void client_core_plugin_class_init(ClientCorePluginClass* klass) {
  G_OBJECT_CLASS(klass)->dispose = client_core_plugin_dispose;
}

static void client_core_plugin_init(ClientCorePlugin* self) {}

static void method_call_cb(FlMethodChannel* channel, FlMethodCall* method_call,
                           gpointer user_data) {
  ClientCorePlugin* plugin = CLIENT_CORE_PLUGIN(user_data);
  client_core_plugin_handle_method_call(plugin, method_call);
}

void client_core_plugin_register_with_registrar(FlPluginRegistrar* registrar) {
  ClientCorePlugin* plugin = CLIENT_CORE_PLUGIN(
      g_object_new(client_core_plugin_get_type(), nullptr));

  g_autoptr(FlStandardMethodCodec) codec = fl_standard_method_codec_new();
  g_autoptr(FlMethodChannel) channel = fl_method_channel_new(
      fl_plugin_registrar_get_messenger(registrar), kChannelName,
      FL_METHOD_CODEC(codec));
  fl_method_channel_set_method_call_handler(channel, method_call_cb,
                                            g_object_ref(plugin),
                                            g_object_unref);
  g_object_unref(plugin);
}
