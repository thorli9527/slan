#ifndef WIN32_LEAN_AND_MEAN
#define WIN32_LEAN_AND_MEAN
#endif
#include <winsock2.h>
#include <ws2tcpip.h>
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
constexpr wchar_t kWindowsServiceName[] = L"SLANClientV2Service";

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

std::string UrlScheme(const std::string& url) {
  const auto pos = url.find("://");
  if (pos == std::string::npos || pos == 0) {
    return "http";
  }
  return url.substr(0, pos);
}

std::string UrlHost(const std::string& url) {
  const auto scheme_pos = url.find("://");
  const auto start = scheme_pos == std::string::npos ? 0 : scheme_pos + 3;
  if (start >= url.size()) {
    return "";
  }
  if (url[start] == '[') {
    const auto end = url.find(']', start + 1);
    if (end == std::string::npos || end <= start + 1) {
      return "";
    }
    return url.substr(start + 1, end - start - 1);
  }
  const auto end = url.find_first_of(":/?", start);
  if (end == std::string::npos) {
    return url.substr(start);
  }
  return url.substr(start, end - start);
}

bool UrlHasPort(const std::string& url, const std::string& port_suffix) {
  return url.find(port_suffix) != std::string::npos;
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
  STARTUPINFOW task_startup_info{};
  task_startup_info.cb = sizeof(task_startup_info);
  task_startup_info.dwFlags = STARTF_USESHOWWINDOW;
  task_startup_info.wShowWindow = SW_HIDE;

  PROCESS_INFORMATION task_process_info{};
  std::wstring task_command_line =
      std::wstring(L"sc.exe start ") + kWindowsServiceName;
  std::vector<wchar_t> mutable_task_command(
      task_command_line.begin(), task_command_line.end());
  mutable_task_command.push_back(L'\0');
  if (CreateProcessW(
          nullptr,
          mutable_task_command.data(),
          nullptr,
          nullptr,
          FALSE,
          CREATE_NO_WINDOW,
          nullptr,
          nullptr,
          &task_startup_info,
          &task_process_info)) {
    WaitForSingleObject(task_process_info.hProcess, 5000);
    DWORD exit_code = 1;
    GetExitCodeProcess(task_process_info.hProcess, &exit_code);
    CloseHandle(task_process_info.hThread);
    CloseHandle(task_process_info.hProcess);
    if (exit_code == 0) {
      Sleep(500);
      return true;
    }
  }

  if (reinterpret_cast<intptr_t>(
          ShellExecuteW(nullptr, L"runas", L"sc.exe",
                        (std::wstring(L"start ") + kWindowsServiceName).c_str(),
                        nullptr, SW_HIDE)) > 32) {
    Sleep(1000);
    return true;
  }

  const auto service_path = ResolveBundledServicePath();
  if (!service_path.has_value()) {
    return false;
  }

  if (reinterpret_cast<intptr_t>(
          ShellExecuteW(nullptr, L"runas", service_path->c_str(), nullptr, nullptr, SW_HIDE)) <=
      32) {
    return false;
  }
  Sleep(1000);
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
  for (int attempt = 0; attempt < 60; ++attempt) {
    if (const auto response = ForwardToService(method, arguments)) {
      return response;
    }
    Sleep(500);
  }
  return std::nullopt;
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
  if (const auto service_response = ForwardToServiceWithAutoStart(method, method_call.arguments())) {
    result->Success(flutter::EncodableValue(*service_response));
    return;
  }
  result->NotImplemented();
}

}  // namespace client_core_plugin
