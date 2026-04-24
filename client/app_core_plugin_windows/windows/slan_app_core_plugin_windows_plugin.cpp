#ifndef WIN32_LEAN_AND_MEAN
#define WIN32_LEAN_AND_MEAN
#endif
#include <winsock2.h>
#include <ws2tcpip.h>

#include "include/slan_app_core_plugin_windows/slan_app_core_plugin_windows_plugin.h"

#include <flutter/method_channel.h>
#include <flutter/plugin_registrar_windows.h>
#include <flutter/standard_method_codec.h>

#include <windows.h>

#include <cstdio>
#include <fstream>
#include <filesystem>
#include <memory>
#include <optional>
#include <sstream>
#include <string>
#include <vector>

namespace slan_app_core_plugin_windows {

namespace {

constexpr char kDefaultAppCoreServiceHost[] = "127.0.0.1:46391";
constexpr wchar_t kWindowsServiceName[] = L"SLANAppCoreService";

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

std::string JsonFromEncodableValue(const flutter::EncodableValue& value);

std::string JsonFromEncodableMap(const flutter::EncodableMap& map) {
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
    json << "\"" << EscapeJsonString(*key) << "\":"
         << JsonFromEncodableValue(entry.second);
  }
  json << "}";
  return json.str();
}

std::string JsonFromEncodableList(const flutter::EncodableList& list) {
  std::ostringstream json;
  json << "[";
  for (size_t index = 0; index < list.size(); ++index) {
    if (index > 0) {
      json << ",";
    }
    json << JsonFromEncodableValue(list[index]);
  }
  json << "]";
  return json.str();
}

std::string JsonFromEncodableValue(const flutter::EncodableValue& value) {
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
    return JsonFromEncodableList(*list_value);
  }
  if (const auto* map_value = std::get_if<flutter::EncodableMap>(&value)) {
    return JsonFromEncodableMap(*map_value);
  }
  return "null";
}

std::optional<std::wstring> GetEnvironmentPath(const wchar_t* name) {
  const DWORD size = GetEnvironmentVariableW(name, nullptr, 0);
  if (size == 0) {
    return std::nullopt;
  }
  std::wstring value(size - 1, L'\0');
  if (GetEnvironmentVariableW(name, value.data(), size) == 0) {
    return std::nullopt;
  }
  return value;
}

std::wstring Utf8ToWide(const std::string& value);
std::string WideToUtf8(const std::wstring& value);
std::string WideToUtf8(const wchar_t* value);

std::wstring ResolveHelperExecutablePath() {
  if (const auto env_path = GetEnvironmentPath(L"SLAN_APP_CORE_HELPER")) {
    return *env_path;
  }

  std::wstring executable_path(MAX_PATH, L'\0');
  DWORD length = GetModuleFileNameW(
      nullptr, executable_path.data(),
      static_cast<DWORD>(executable_path.size()));
  executable_path.resize(length);
  const auto separator = executable_path.find_last_of(L"\\/");
  const std::wstring directory =
      separator == std::wstring::npos ? L"." : executable_path.substr(0, separator);
  return directory + L"\\app-core-helper.exe";
}

std::wstring ResolveServiceExecutablePath() {
  if (const auto env_path = GetEnvironmentPath(L"SLAN_APP_CORE_SERVICE")) {
    return *env_path;
  }

  std::wstring executable_path(MAX_PATH, L'\0');
  DWORD length = GetModuleFileNameW(
      nullptr, executable_path.data(),
      static_cast<DWORD>(executable_path.size()));
  executable_path.resize(length);
  const auto separator = executable_path.find_last_of(L"\\/");
  const std::wstring directory =
      separator == std::wstring::npos ? L"." : executable_path.substr(0, separator);
  return directory + L"\\app-core-service.exe";
}

std::string ResolveServiceHost() {
  if (const auto env_host = GetEnvironmentPath(L"SLAN_APP_CORE_SERVICE_HOST")) {
    const auto utf8 = WideToUtf8(*env_host);
    if (!utf8.empty()) {
      return utf8;
    }
  }
  if (const auto env_host = GetEnvironmentPath(L"SLAN_APP_CORE_HELPER_HOST")) {
    const auto utf8 = WideToUtf8(*env_host);
    if (!utf8.empty()) {
      return utf8;
    }
  }
  return kDefaultAppCoreServiceHost;
}

std::string BuildHelperRequestJson(
    const std::string& method_name,
    const flutter::EncodableValue* arguments) {
  std::ostringstream request;
  request << "{\"method\":\"" << EscapeJsonString(method_name) << "\",\"args\":";
  if (arguments == nullptr) {
    request << "{}";
  } else {
    request << JsonFromEncodableValue(*arguments);
  }
  request << "}\n";
  return request.str();
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

std::string WideToUtf8(const std::wstring& value) {
  if (value.empty()) {
    return "";
  }
  const int size = WideCharToMultiByte(
      CP_UTF8, 0, value.c_str(), static_cast<int>(value.size()), nullptr, 0, nullptr, nullptr);
  std::string utf8(size, '\0');
  WideCharToMultiByte(
      CP_UTF8,
      0,
      value.c_str(),
      static_cast<int>(value.size()),
      utf8.data(),
      size,
      nullptr,
      nullptr);
  return utf8;
}

std::string WideToUtf8(const wchar_t* value) {
  if (value == nullptr) {
    return "";
  }
  return WideToUtf8(std::wstring(value));
}

bool TryStartWindowsService() {
  STARTUPINFOW startup_info{};
  startup_info.cb = sizeof(startup_info);
  startup_info.dwFlags = STARTF_USESHOWWINDOW;
  startup_info.wShowWindow = SW_HIDE;

  PROCESS_INFORMATION process_info{};
  std::wstring command_line =
      L"cmd.exe /C sc.exe start \"" + std::wstring(kWindowsServiceName) + L"\"";
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

  WaitForSingleObject(process_info.hProcess, 10'000);
  DWORD exit_code = 1;
  GetExitCodeProcess(process_info.hProcess, &exit_code);
  CloseHandle(process_info.hThread);
  CloseHandle(process_info.hProcess);
  return exit_code == 0 || exit_code == 1056;
}

bool WindowsServiceExists() {
  STARTUPINFOW startup_info{};
  startup_info.cb = sizeof(startup_info);
  startup_info.dwFlags = STARTF_USESHOWWINDOW;
  startup_info.wShowWindow = SW_HIDE;

  PROCESS_INFORMATION process_info{};
  std::wstring command_line =
      L"cmd.exe /C sc.exe query \"" + std::wstring(kWindowsServiceName) + L"\"";
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

  WaitForSingleObject(process_info.hProcess, 10'000);
  DWORD exit_code = 1;
  GetExitCodeProcess(process_info.hProcess, &exit_code);
  CloseHandle(process_info.hThread);
  CloseHandle(process_info.hProcess);
  return exit_code == 0;
}

std::wstring CurrentExecutableDirectory() {
  std::wstring executable_path(MAX_PATH, L'\0');
  DWORD length = GetModuleFileNameW(
      nullptr, executable_path.data(),
      static_cast<DWORD>(executable_path.size()));
  executable_path.resize(length);
  const auto separator = executable_path.find_last_of(L"\\/");
  if (separator == std::wstring::npos) {
    return L".";
  }
  return executable_path.substr(0, separator);
}

}  // namespace

class AppCoreServiceBridgeClient {
 public:
  AppCoreServiceBridgeClient() = default;
  ~AppCoreServiceBridgeClient() { Close(); }

  bool Invoke(
      const std::string& method_name,
      const flutter::EncodableValue* arguments,
      std::string* response,
      std::string* error_message) {
    const std::string request = BuildHelperRequestJson(method_name, arguments);
    for (int attempt = 0; attempt < 2; ++attempt) {
      if (!EnsureConnected(error_message)) {
        return false;
      }
      if (!WriteRequest(request, error_message)) {
        ResetSocket();
        continue;
      }
      if (ReadResponseLine(response, error_message)) {
        return true;
      }
      ResetSocket();
    }
    return false;
  }

 private:
  bool ConnectToResolvedServiceHost(std::string* error_message) {
    addrinfo hints{};
    hints.ai_family = AF_UNSPEC;
    hints.ai_socktype = SOCK_STREAM;
    hints.ai_protocol = IPPROTO_TCP;

    const std::string service_host = ResolveServiceHost();
    const auto separator = service_host.rfind(':');
    if (separator == std::string::npos || separator == 0 || separator == service_host.size() - 1) {
      if (error_message != nullptr) {
        *error_message = "Invalid app-core service host. Expected host:port.";
      }
      return false;
    }
    const std::string hostname = service_host.substr(0, separator);
    const std::string port = service_host.substr(separator + 1);

    addrinfo* resolved = nullptr;
    const int resolve_result =
        getaddrinfo(hostname.c_str(), port.c_str(), &hints, &resolved);
    if (resolve_result != 0) {
      if (error_message != nullptr) {
        *error_message = "Failed to resolve app-core service host.";
      }
      return false;
    }

    bool connected = false;
    for (int attempt = 0; attempt < 25 && !connected; ++attempt) {
      for (addrinfo* current = resolved; current != nullptr; current = current->ai_next) {
        SOCKET candidate =
            socket(current->ai_family, current->ai_socktype, current->ai_protocol);
        if (candidate == INVALID_SOCKET) {
          continue;
        }
        if (connect(candidate, current->ai_addr, static_cast<int>(current->ai_addrlen)) == 0) {
          socket_ = candidate;
          connected = true;
          break;
        }
        closesocket(candidate);
      }
      if (!connected) {
        Sleep(200);
      }
    }
    freeaddrinfo(resolved);

    if (!connected) {
      if (error_message != nullptr) {
        *error_message =
            "Failed to connect to app-core service. Ensure app-core-service.exe is available.";
      }
      return false;
    }
    return true;
  }

  bool EnsureConnected(std::string* error_message) {
    if (socket_ != INVALID_SOCKET) {
      return true;
    }
    if (!EnsureWinsock(error_message)) {
      return false;
    }

    if (ConnectToResolvedServiceHost(nullptr)) {
      return true;
    }

    if (!EnsureServiceStarted(error_message)) {
      return false;
    }

    return ConnectToResolvedServiceHost(error_message);
  }

  bool EnsureWinsock(std::string* error_message) {
    if (winsock_ready_) {
      return true;
    }
    WSADATA winsock_data{};
    if (WSAStartup(MAKEWORD(2, 2), &winsock_data) != 0) {
      if (error_message != nullptr) {
        *error_message = "Failed to initialize Winsock for app-core service bridge.";
      }
      return false;
    }
    winsock_ready_ = true;
    return true;
  }

  bool EnsureServiceStarted(std::string* error_message) {
    if (process_ != nullptr) {
      DWORD exit_code = STILL_ACTIVE;
      if (GetExitCodeProcess(process_, &exit_code) && exit_code == STILL_ACTIVE) {
        return true;
      }
      CloseHandle(process_);
      process_ = nullptr;
    }

    if (TryStartWindowsService()) {
      return true;
    }
    if (error_message != nullptr) {
      if (!WindowsServiceExists()) {
        *error_message =
            "SLAN AppCore Service is not installed. Please reinstall SLAN so the Windows service is registered.";
      } else {
        *error_message =
            "SLAN AppCore Service could not be started. Please reinstall SLAN or restart the machine to restore the Windows service.";
      }
    }
    return false;
  }

  bool WriteRequest(const std::string& request, std::string* error_message) {
    const char* current = request.data();
    int remaining = static_cast<int>(request.size());
    while (remaining > 0) {
      const int bytes_sent = send(socket_, current, remaining, 0);
      if (bytes_sent == SOCKET_ERROR) {
        if (error_message != nullptr) {
          *error_message = "Failed to write request to app-core service.";
        }
        return false;
      }
      current += bytes_sent;
      remaining -= bytes_sent;
    }
    return true;
  }

  bool ReadResponseLine(std::string* response, std::string* error_message) {
    response->clear();
    char byte = 0;
    while (true) {
      const int bytes_read = recv(socket_, &byte, 1, 0);
      if (bytes_read <= 0) {
        if (error_message != nullptr) {
          *error_message = "app-core service closed its TCP connection unexpectedly.";
        }
        return false;
      }
      if (byte == '\n') {
        return true;
      }
      response->push_back(byte);
    }
  }

  void ResetSocket() {
    if (socket_ != INVALID_SOCKET) {
      closesocket(socket_);
      socket_ = INVALID_SOCKET;
    }
  }

  void Close() {
    ResetSocket();
    if (process_ != nullptr) {
      TerminateProcess(process_, 0);
      CloseHandle(process_);
      process_ = nullptr;
    }
    if (winsock_ready_) {
      WSACleanup();
      winsock_ready_ = false;
    }
  }

  HANDLE process_ = nullptr;
  SOCKET socket_ = INVALID_SOCKET;
  bool winsock_ready_ = false;
};

void SlanAppCorePluginWindowsPlugin::RegisterWithRegistrar(
    flutter::PluginRegistrarWindows* registrar) {
  auto channel = std::make_unique<flutter::MethodChannel<flutter::EncodableValue>>(
      registrar->messenger(), "slan/app_core",
      &flutter::StandardMethodCodec::GetInstance());

  auto plugin = std::make_unique<SlanAppCorePluginWindowsPlugin>();

  channel->SetMethodCallHandler(
      [plugin_pointer = plugin.get()](const auto& call, auto result) {
        plugin_pointer->HandleMethodCall(call, std::move(result));
      });

  registrar->AddPlugin(std::move(plugin));
}

SlanAppCorePluginWindowsPlugin::SlanAppCorePluginWindowsPlugin() {}

SlanAppCorePluginWindowsPlugin::~SlanAppCorePluginWindowsPlugin() {}

std::optional<std::string> SlanAppCorePluginWindowsPlugin::ForwardToService(
    const std::string& method_name,
    const flutter::EncodableValue* arguments,
    std::string* error_message) {
  std::lock_guard<std::mutex> lock(service_mutex_);
  if (!service_) {
    service_ = std::make_unique<AppCoreServiceBridgeClient>();
  }
  std::string response;
  if (!service_->Invoke(method_name, arguments, &response, error_message)) {
    return std::nullopt;
  }
  return response;
}

void SlanAppCorePluginWindowsPlugin::HandleMethodCall(
    const flutter::MethodCall<flutter::EncodableValue>& method_call,
    std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>> result) {
  std::string error_message;
  const auto response = ForwardToService(
      method_call.method_name(), method_call.arguments(), &error_message);
  if (!response.has_value()) {
    result->Error("app_core_process_start_failed", error_message);
    return;
  }
  result->Success(flutter::EncodableValue(*response));
}

}  // namespace slan_app_core_plugin_windows
