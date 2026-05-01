#ifndef WIN32_LEAN_AND_MEAN
#define WIN32_LEAN_AND_MEAN
#endif
#include <winsock2.h>
#include <ws2tcpip.h>
#include <shellapi.h>
#include <windows.h>

#include "client_core_plugin_internal.h"

#include <flutter/method_channel.h>
#include <flutter/plugin_registrar_windows.h>
#include <flutter/standard_method_codec.h>

#include <cstdio>
#include <memory>
#include <optional>
#include <sstream>
#include <string>
#include <variant>
#include <vector>

namespace client_core_plugin {
namespace {

constexpr char kChannelName[] = "dev.slan/client_core_v2";
constexpr char kDefaultServiceHost[] = "127.0.0.1:46392";

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

std::string JsonFromValue(const flutter::EncodableValue& value);

std::string JsonFromMap(const flutter::EncodableMap& map) {
  std::ostringstream json;
  json << "{";
  bool first = true;
  for (const auto& entry : map) {
    const auto* key = std::get_if<std::string>(&entry.first);
    if (key == nullptr) {
      continue;
    }
    if (!first) {
      json << ",";
    }
    first = false;
    json << "\"" << EscapeJsonString(*key) << "\":" << JsonFromValue(entry.second);
  }
  json << "}";
  return json.str();
}

std::string JsonFromList(const flutter::EncodableList& list) {
  std::ostringstream json;
  json << "[";
  for (size_t index = 0; index < list.size(); ++index) {
    if (index > 0) {
      json << ",";
    }
    json << JsonFromValue(list[index]);
  }
  json << "]";
  return json.str();
}

std::string JsonFromValue(const flutter::EncodableValue& value) {
  if (std::holds_alternative<std::monostate>(value)) {
    return "null";
  }
  if (const auto* string_value = std::get_if<std::string>(&value)) {
    return "\"" + EscapeJsonString(*string_value) + "\"";
  }
  if (const auto* bool_value = std::get_if<bool>(&value)) {
    return *bool_value ? "true" : "false";
  }
  if (const auto* int_value = std::get_if<int32_t>(&value)) {
    return std::to_string(*int_value);
  }
  if (const auto* long_value = std::get_if<int64_t>(&value)) {
    return std::to_string(*long_value);
  }
  if (const auto* double_value = std::get_if<double>(&value)) {
    std::ostringstream json;
    json << *double_value;
    return json.str();
  }
  if (const auto* list_value = std::get_if<flutter::EncodableList>(&value)) {
    return JsonFromList(*list_value);
  }
  if (const auto* map_value = std::get_if<flutter::EncodableMap>(&value)) {
    return JsonFromMap(*map_value);
  }
  return "null";
}

std::optional<std::string> ReadEnvironmentString(const wchar_t* name) {
  const DWORD size = GetEnvironmentVariableW(name, nullptr, 0);
  if (size == 0) {
    return std::nullopt;
  }
  std::wstring wide(size - 1, L'\0');
  if (GetEnvironmentVariableW(name, wide.data(), size) == 0) {
    return std::nullopt;
  }
  const int utf8_size = WideCharToMultiByte(
      CP_UTF8, 0, wide.c_str(), static_cast<int>(wide.size()), nullptr, 0, nullptr, nullptr);
  std::string utf8(utf8_size, '\0');
  WideCharToMultiByte(
      CP_UTF8, 0, wide.c_str(), static_cast<int>(wide.size()), utf8.data(), utf8_size, nullptr, nullptr);
  return utf8;
}

std::wstring Utf8ToWide(const std::string& value) {
  if (value.empty()) {
    return L"";
  }
  const int size = MultiByteToWideChar(
      CP_UTF8, 0, value.c_str(), static_cast<int>(value.size()), nullptr, 0);
  std::wstring wide(size, L'\0');
  MultiByteToWideChar(
      CP_UTF8, 0, value.c_str(), static_cast<int>(value.size()), wide.data(), size);
  return wide;
}

std::string ResolveServiceHost() {
  const auto env_host = ReadEnvironmentString(L"SLAN_CLIENT_CORE_SERVICE_HOST");
  return env_host.has_value() && !env_host->empty() ? *env_host : kDefaultServiceHost;
}

std::string ResolveWebConsoleUrl() {
  if (const auto value = ReadEnvironmentString(L"SLAN_WEB_CONSOLE_URL")) {
    if (!value->empty()) {
      return *value;
    }
  }
  if (const auto value = ReadEnvironmentString(L"SLAN_CONTROL_BASE_URL")) {
    if (!value->empty()) {
      if (value->find("127.0.0.1") != std::string::npos ||
          value->find("localhost") != std::string::npos) {
        return "http://127.0.0.1:24200";
      }
      if (value->find("slan.localhost") != std::string::npos) {
        return "https://web.slan.localhost:18443";
      }
    }
  }
  return "http://127.0.0.1:24200";
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

std::string AppendQueryParam(
    const std::string& url,
    const std::string& key,
    const std::string& value) {
  if (value.empty()) {
    return url;
  }
  const char separator = url.find('?') == std::string::npos ? '?' : '&';
  return url + separator + key + "=" + value;
}

void OpenWebConsole(const std::string& callback_id = "", const std::string& device_id = "") {
  std::string url = ResolveWebConsoleUrl();
  if (!callback_id.empty()) {
    url = AppendQueryParam(url, "auth", "login");
    url = AppendQueryParam(url, "callbackId", callback_id);
  }
  if (!device_id.empty()) {
    url = AppendQueryParam(url, "deviceId", device_id);
  }
  const std::wstring wide_url = Utf8ToWide(url);
  ShellExecuteW(nullptr, L"open", wide_url.c_str(), nullptr, nullptr, SW_SHOWNORMAL);
}

std::optional<std::wstring> ResolveBundledServicePath() {
  std::wstring module_path(MAX_PATH, L'\0');
  const DWORD size = GetModuleFileNameW(nullptr, module_path.data(), static_cast<DWORD>(module_path.size()));
  if (size == 0 || size >= module_path.size()) {
    return std::nullopt;
  }
  module_path.resize(size);
  const auto separator = module_path.find_last_of(L"\\/");
  if (separator == std::wstring::npos) {
    return std::nullopt;
  }
  return module_path.substr(0, separator + 1) + L"client-core-service.exe";
}

bool TryStartBundledService() {
  const auto service_path = ResolveBundledServicePath();
  if (!service_path.has_value()) {
    return false;
  }

  STARTUPINFOW startup_info{};
  startup_info.cb = sizeof(startup_info);
  startup_info.dwFlags = STARTF_USESHOWWINDOW;
  startup_info.wShowWindow = SW_HIDE;

  PROCESS_INFORMATION process_info{};
  std::wstring command_line = L"\"" + *service_path + L"\"";
  std::vector<wchar_t> mutable_command(command_line.begin(), command_line.end());
  mutable_command.push_back(L'\0');

  if (!CreateProcessW(
          nullptr,
          mutable_command.data(),
          nullptr,
          nullptr,
          FALSE,
          CREATE_NO_WINDOW,
          nullptr,
          nullptr,
          &startup_info,
          &process_info)) {
    return false;
  }

  CloseHandle(process_info.hThread);
  CloseHandle(process_info.hProcess);
  Sleep(300);
  return true;
}

std::string BuildRequest(
    const std::string& method,
    const flutter::EncodableValue* arguments) {
  std::ostringstream request;
  request << "{\"method\":\"" << EscapeJsonString(method) << "\",\"args\":";
  if (arguments == nullptr) {
    request << "{}";
  } else {
    request << JsonFromValue(*arguments);
  }
  request << "}\n";
  return request.str();
}

std::optional<std::string> ForwardToService(
    const std::string& method,
    const flutter::EncodableValue* arguments) {
  WSADATA winsock_data{};
  if (WSAStartup(MAKEWORD(2, 2), &winsock_data) != 0) {
    return std::nullopt;
  }

  const std::string service_host = ResolveServiceHost();
  const auto separator = service_host.rfind(':');
  if (separator == std::string::npos || separator == 0 || separator == service_host.size() - 1) {
    WSACleanup();
    return std::nullopt;
  }

  const std::string hostname = service_host.substr(0, separator);
  const std::string port = service_host.substr(separator + 1);

  addrinfo hints{};
  hints.ai_family = AF_UNSPEC;
  hints.ai_socktype = SOCK_STREAM;
  hints.ai_protocol = IPPROTO_TCP;

  addrinfo* resolved = nullptr;
  if (getaddrinfo(hostname.c_str(), port.c_str(), &hints, &resolved) != 0) {
    WSACleanup();
    return std::nullopt;
  }

  SOCKET socket = INVALID_SOCKET;
  for (addrinfo* current = resolved; current != nullptr; current = current->ai_next) {
    SOCKET candidate = ::socket(current->ai_family, current->ai_socktype, current->ai_protocol);
    if (candidate == INVALID_SOCKET) {
      continue;
    }
    if (connect(candidate, current->ai_addr, static_cast<int>(current->ai_addrlen)) == 0) {
      socket = candidate;
      break;
    }
    closesocket(candidate);
  }
  freeaddrinfo(resolved);

  if (socket == INVALID_SOCKET) {
    WSACleanup();
    return std::nullopt;
  }

  const std::string request = BuildRequest(method, arguments);
  const char* current = request.data();
  int remaining = static_cast<int>(request.size());
  while (remaining > 0) {
    const int sent = send(socket, current, remaining, 0);
    if (sent == SOCKET_ERROR) {
      closesocket(socket);
      WSACleanup();
      return std::nullopt;
    }
    current += sent;
    remaining -= sent;
  }

  std::string response;
  char byte = 0;
  while (recv(socket, &byte, 1, 0) > 0) {
    if (byte == '\n') {
      closesocket(socket);
      WSACleanup();
      return response;
    }
    response.push_back(byte);
  }

  closesocket(socket);
  WSACleanup();
  return std::nullopt;
}

std::optional<std::string> ForwardToServiceWithAutoStart(
    const std::string& method,
    const flutter::EncodableValue* arguments) {
  if (const auto response = ForwardToService(method, arguments)) {
    return response;
  }
  if (!TryStartBundledService()) {
    return std::nullopt;
  }
  for (int attempt = 0; attempt < 15; ++attempt) {
    if (const auto response = ForwardToService(method, arguments)) {
      return response;
    }
    Sleep(100);
  }
  return std::nullopt;
}

std::string ReadCommandType(const flutter::EncodableValue* arguments) {
  if (arguments == nullptr) {
    return "";
  }
  const auto* map = std::get_if<flutter::EncodableMap>(arguments);
  if (map == nullptr) {
    return "";
  }
  const auto entry = map->find(flutter::EncodableValue("type"));
  if (entry == map->end()) {
    return "";
  }
  const auto* value = std::get_if<std::string>(&entry->second);
  return value == nullptr ? "" : *value;
}

void PutOptionalString(
    flutter::EncodableMap* map,
    const char* key,
    const std::string& value) {
  if (value.empty()) {
    map->insert({flutter::EncodableValue(key), flutter::EncodableValue()});
  } else {
    map->insert({flutter::EncodableValue(key), flutter::EncodableValue(value)});
  }
}

}  // namespace

ClientCorePlugin::ClientCorePlugin() = default;

ClientCorePlugin::~ClientCorePlugin() = default;

void ClientCorePlugin::RegisterWithRegistrar(
    flutter::PluginRegistrarWindows* registrar) {
  auto channel = std::make_unique<flutter::MethodChannel<flutter::EncodableValue>>(
      registrar->messenger(), kChannelName,
      &flutter::StandardMethodCodec::GetInstance());

  auto plugin = std::make_unique<ClientCorePlugin>();
  channel->SetMethodCallHandler(
      [plugin_pointer = plugin.get()](
          const auto& call,
          auto result) {
        plugin_pointer->HandleMethodCall(call, std::move(result));
      });

  registrar->AddPlugin(std::move(plugin));
}

void ClientCorePlugin::HandleMethodCall(
    const flutter::MethodCall<flutter::EncodableValue>& method_call,
    std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>> result) {
  const auto& method = method_call.method_name();
  const auto command_type =
      method == "dispatch" ? ReadCommandType(method_call.arguments()) : "";
  if (const auto service_response = ForwardToServiceWithAutoStart(method, method_call.arguments())) {
    if (command_type == "openWebConsole") {
      OpenWebConsole();
    } else if (command_type == "loginWithBrowser") {
      OpenWebConsole(
          ExtractJsonStringField(*service_response, "authCallbackId"),
          ExtractJsonStringField(*service_response, "deviceId"));
    }
    result->Success(flutter::EncodableValue(*service_response));
    return;
  }
  if (method == "start") {
    notice_ = "windowsPluginReady";
    error_.clear();
    result->Success(flutter::EncodableValue(StateAsMap()));
    return;
  }
  if (method == "state" || method == "refresh") {
    result->Success(flutter::EncodableValue(StateAsMap()));
    return;
  }
  if (method == "dispatch") {
    auto state = Dispatch(method_call.arguments());
    if (command_type == "openWebConsole") {
      OpenWebConsole();
    } else if (command_type == "loginWithBrowser") {
      OpenWebConsole(auth_callback_id_, device_id_);
    }
    result->Success(flutter::EncodableValue(state));
    return;
  }
  if (method == "enqueueControlTask" || method == "enqueueDownstreamControlTask" ||
      method == "ingestDownstreamControlMessage") {
    syncing_ = false;
    switch_enabled_ = true;
    error_ = "local service unavailable";
    result->Success(flutter::EncodableValue(StateAsMap()));
    return;
  }
  if (method == "controlTransportStatus") {
    flutter::EncodableMap map;
    map.insert({flutter::EncodableValue("mqttCredentialReady"), flutter::EncodableValue(false)});
    map.insert({flutter::EncodableValue("controlSessionReady"), flutter::EncodableValue(false)});
    map.insert({flutter::EncodableValue("ready"), flutter::EncodableValue(false)});
    map.insert({
        flutter::EncodableValue("missing"),
        flutter::EncodableValue(flutter::EncodableList{
            flutter::EncodableValue("localService"),
        }),
    });
    result->Success(flutter::EncodableValue(map));
    return;
  }
  if (method == "controlTransportPlan") {
    flutter::EncodableMap map;
    map.insert({flutter::EncodableValue("heartbeatQos"), flutter::EncodableValue("qos0")});
    map.insert({flutter::EncodableValue("controlQos"), flutter::EncodableValue("qos2")});
    result->Success(flutter::EncodableValue(map));
    return;
  }
  if (method == "controlTransportCadence") {
    flutter::EncodableMap map;
    map.insert({flutter::EncodableValue("ackFlushIntervalMs"), flutter::EncodableValue(1000)});
    map.insert({flutter::EncodableValue("heartbeatIntervalMs"), flutter::EncodableValue(30000)});
    map.insert({flutter::EncodableValue("runtimeStateIntervalMs"), flutter::EncodableValue(10000)});
    result->Success(flutter::EncodableValue(map));
    return;
  }
  if (method == "controlTransportTickPlan") {
    flutter::EncodableMap outbox;
    outbox.insert({flutter::EncodableValue("includeHeartbeat"), flutter::EncodableValue(true)});
    outbox.insert({flutter::EncodableValue("includeRuntimeState"), flutter::EncodableValue(true)});
    outbox.insert({flutter::EncodableValue("includeControlAcks"), flutter::EncodableValue(true)});
    flutter::EncodableMap map;
    map.insert({flutter::EncodableValue("nowMs"), flutter::EncodableValue(static_cast<int64_t>(0))});
    map.insert({flutter::EncodableValue("outbox"), flutter::EncodableValue(outbox)});
    map.insert({flutter::EncodableValue("nextAckFlushDueMs"), flutter::EncodableValue(static_cast<int64_t>(0))});
    map.insert({flutter::EncodableValue("nextHeartbeatDueMs"), flutter::EncodableValue(static_cast<int64_t>(0))});
    map.insert({flutter::EncodableValue("nextRuntimeStateDueMs"), flutter::EncodableValue(static_cast<int64_t>(0))});
    result->Success(flutter::EncodableValue(map));
    return;
  }
  if (method == "controlTransportOutbox") {
    flutter::EncodableMap map;
    map.insert({flutter::EncodableValue("messages"), flutter::EncodableValue(flutter::EncodableList{})});
    result->Success(flutter::EncodableValue(map));
    return;
  }
  if (method == "pendingControlAcks") {
    result->Success(flutter::EncodableValue(flutter::EncodableList{}));
    return;
  }
  if (method == "markControlAcked") {
    flutter::EncodableMap map;
    map.insert({flutter::EncodableValue("acknowledged"), flutter::EncodableValue(false)});
    map.insert({flutter::EncodableValue("error"), flutter::EncodableValue("local service unavailable")});
    result->Success(flutter::EncodableValue(map));
    return;
  }
  if (method == "markTransportPublished") {
    flutter::EncodableMap map;
    map.insert({flutter::EncodableValue("published"), flutter::EncodableValue(false)});
    map.insert({flutter::EncodableValue("error"), flutter::EncodableValue("local service unavailable")});
    result->Success(flutter::EncodableValue(map));
    return;
  }
  if (method == "shutdownNetwork") {
    network_enabled_ = false;
    virtual_ip_.clear();
    notice_ = "networkShutdown";
    result->Success(flutter::EncodableValue(StateAsMap()));
    return;
  }
  result->NotImplemented();
}

flutter::EncodableMap ClientCorePlugin::Dispatch(
    const flutter::EncodableValue* arguments) {
  const std::string type = ReadCommandType(arguments);
  syncing_ = true;
  sync_reason_ = type;
  switch_enabled_ = false;
  error_.clear();
  notice_.clear();

  if (type == "loginWithBrowser") {
    auth_callback_id_ = device_id_.empty() ? "windows-plugin-login" : device_id_;
    notice_ = "loginBrowserRequested";
  } else if (type == "enableNetwork") {
    network_enabled_ = false;
    error_ = "local service unavailable: cannot enable network";
  } else if (type == "disableNetwork") {
    network_enabled_ = false;
    virtual_ip_.clear();
    notice_ = "networkDisableRequested";
  } else if (type == "logout") {
    signed_in_ = false;
    user_label_.clear();
    device_id_.clear();
    auth_callback_id_.clear();
    network_enabled_ = false;
    virtual_ip_.clear();
    notice_ = "signedOut";
  } else if (type == "refresh") {
    notice_ = "refreshed";
  } else if (type == "openWebConsole") {
    notice_ = "webConsoleRequested";
  } else {
    error_ = "unsupported command: " + type;
  }

  syncing_ = false;
  sync_reason_.clear();
  switch_enabled_ = true;
  return StateAsMap();
}

flutter::EncodableMap ClientCorePlugin::StateAsMap() const {
  flutter::EncodableMap map;
  map.insert({flutter::EncodableValue("signedIn"), flutter::EncodableValue(signed_in_)});
  PutOptionalString(&map, "userLabel", user_label_);
  PutOptionalString(&map, "deviceId", device_id_);
  PutOptionalString(&map, "authCallbackId", auth_callback_id_);
  PutOptionalString(&map, "virtualIp", virtual_ip_);
  map.insert({flutter::EncodableValue("networkEnabled"), flutter::EncodableValue(network_enabled_)});
  map.insert({flutter::EncodableValue("syncing"), flutter::EncodableValue(syncing_)});
  PutOptionalString(&map, "syncReason", sync_reason_);
  map.insert({flutter::EncodableValue("switchEnabled"), flutter::EncodableValue(switch_enabled_)});
  PutOptionalString(&map, "notice", notice_);
  PutOptionalString(&map, "error", error_);
  return map;
}

}  // namespace client_core_plugin
