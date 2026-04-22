#include "include/slan_app_core_plugin_windows/slan_app_core_plugin_windows_plugin.h"

#include <flutter/method_channel.h>
#include <flutter/plugin_registrar_windows.h>
#include <flutter/standard_method_codec.h>

#include <windows.h>
#include <shellapi.h>

#include <cstdio>
#include <fstream>
#include <filesystem>
#include <memory>
#include <optional>
#include <sstream>
#include <string>

namespace slan_app_core_plugin_windows {

namespace {

constexpr wchar_t kDefaultWindowsTunnelInterfaceAlias[] = L"Loopback Pseudo-Interface 1";
constexpr wchar_t kPreferredWindowsTunnelInterfaceDescription[] =
    L"Microsoft KM-TEST Loopback Adapter";

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

std::optional<std::string> GetStringFromMap(
    const flutter::EncodableMap& map,
    const char* key) {
  const auto iterator = map.find(flutter::EncodableValue(key));
  if (iterator == map.end()) {
    return std::nullopt;
  }
  if (const auto* value = std::get_if<std::string>(&iterator->second)) {
    return *value;
  }
  return std::nullopt;
}

const flutter::EncodableMap* GetMapFromMap(
    const flutter::EncodableMap& map,
    const char* key) {
  const auto iterator = map.find(flutter::EncodableValue(key));
  if (iterator == map.end()) {
    return nullptr;
  }
  return std::get_if<flutter::EncodableMap>(&iterator->second);
}

const flutter::EncodableList* GetListFromMap(
    const flutter::EncodableMap& map,
    const char* key) {
  const auto iterator = map.find(flutter::EncodableValue(key));
  if (iterator == map.end()) {
    return nullptr;
  }
  return std::get_if<flutter::EncodableList>(&iterator->second);
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

std::string JsonEnvelope(
    bool ok,
    const std::string& result_json,
    const std::string& error_code = "",
    const std::string& error_message = "") {
  std::ostringstream json;
  json << "{\"ok\":" << (ok ? "true" : "false");
  if (ok) {
    json << ",\"result\":" << result_json;
  } else {
    json << ",\"errorCode\":\"" << EscapeJsonString(error_code) << "\"";
    json << ",\"errorMessage\":\"" << EscapeJsonString(error_message) << "\"";
    json << ",\"error\":\"" << EscapeJsonString(error_message) << "\"";
  }
  json << "}";
  return json.str();
}

std::string TunnelActionResultJson(
    const std::string& action,
    bool accepted,
    const std::string& phase,
    const std::string& detail,
    bool has_configuration,
    const std::string& peer_virtual_ip,
    bool running,
    const std::optional<std::string>& last_error) {
  std::ostringstream json;
  json << "{"
       << "\"accepted\":" << (accepted ? "true" : "false")
       << ",\"action\":\"" << EscapeJsonString(action) << "\""
       << ",\"backendState\":\""
       << EscapeJsonString(last_error.has_value() ? "failed" : (running ? "started" : "idle"))
       << "\""
       << ",\"configurationPeerVirtualIp\":\"" << EscapeJsonString(peer_virtual_ip) << "\""
       << ",\"connectionStatus\":\"" << EscapeJsonString(running ? "connected" : "disconnected")
       << "\""
       << ",\"detail\":\"" << EscapeJsonString(detail) << "\""
       << ",\"hasConfiguration\":" << (has_configuration ? "true" : "false")
       << ",\"phase\":\"" << EscapeJsonString(phase) << "\""
       << ",\"runtimeLastError\":";
  if (last_error.has_value()) {
    json << "\"" << EscapeJsonString(*last_error) << "\"";
  } else {
    json << "null";
  }
  json << ",\"runtimeState\":\"" << EscapeJsonString(running ? "configured" : "disconnected")
       << "\""
       << ",\"source\":\"windows-plugin\""
       << "}";
  return json.str();
}

std::optional<std::pair<std::string, int>> ParseAddressAndPrefix(
    const flutter::EncodableValue* configuration) {
  if (configuration == nullptr) {
    return std::nullopt;
  }
  const auto* map = std::get_if<flutter::EncodableMap>(configuration);
  if (map == nullptr) {
    return std::nullopt;
  }
  const auto* interface_map = GetMapFromMap(*map, "wireguardInterface");
  if (interface_map == nullptr) {
    return std::nullopt;
  }
  const auto* addresses = GetListFromMap(*interface_map, "addresses");
  if (addresses == nullptr || addresses->empty()) {
    return std::nullopt;
  }
  const auto* address = std::get_if<std::string>(&addresses->front());
  if (address == nullptr) {
    return std::nullopt;
  }
  const auto separator = address->find('/');
  if (separator == std::string::npos) {
    return std::nullopt;
  }
  return std::make_pair(
      address->substr(0, separator),
      std::stoi(address->substr(separator + 1)));
}

std::optional<std::string> ExtractPeerVirtualIp(
    const flutter::EncodableValue* configuration) {
  if (configuration == nullptr) {
    return std::nullopt;
  }
  const auto* map = std::get_if<flutter::EncodableMap>(configuration);
  if (map == nullptr) {
    return std::nullopt;
  }
  return GetStringFromMap(*map, "peerVirtualIp");
}

std::wstring CreateTempPowerShellScript(const std::wstring& script_body) {
  wchar_t temp_path[MAX_PATH];
  GetTempPathW(MAX_PATH, temp_path);
  wchar_t temp_file[MAX_PATH];
  GetTempFileNameW(temp_path, L"sln", 0, temp_file);
  std::filesystem::path script_path(temp_file);
  script_path.replace_extension(L".ps1");
  std::wofstream output(script_path);
  output << script_body;
  output.close();
  return script_path.wstring();
}

bool RunElevatedPowerShellScript(
    const std::wstring& script_body,
    std::string* error_message) {
  const std::wstring script_path = CreateTempPowerShellScript(script_body);
  SHELLEXECUTEINFOW execute_info{};
  execute_info.cbSize = sizeof(execute_info);
  execute_info.fMask = SEE_MASK_NOCLOSEPROCESS;
  execute_info.lpVerb = L"runas";
  execute_info.lpFile = L"powershell.exe";
  const std::wstring parameters =
      L"-NoProfile -ExecutionPolicy Bypass -File \"" + script_path + L"\"";
  execute_info.lpParameters = parameters.c_str();
  execute_info.nShow = SW_HIDE;
  if (!ShellExecuteExW(&execute_info)) {
    const DWORD error = GetLastError();
    std::filesystem::remove(script_path);
    if (error_message != nullptr) {
      if (error == ERROR_CANCELLED) {
        *error_message = "UAC prompt was cancelled.";
      } else {
        *error_message = "Failed to start elevated PowerShell.";
      }
    }
    return false;
  }
  WaitForSingleObject(execute_info.hProcess, INFINITE);
  DWORD exit_code = 1;
  GetExitCodeProcess(execute_info.hProcess, &exit_code);
  CloseHandle(execute_info.hProcess);
  std::filesystem::remove(script_path);
  if (exit_code == 0) {
    return true;
  }
  if (error_message != nullptr) {
    *error_message = "Elevated PowerShell command failed.";
  }
  return false;
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

std::string RunPowerShellCapture(const std::wstring& command_line) {
  const std::wstring script_path = CreateTempPowerShellScript(command_line);
  const std::wstring output_path =
      (std::filesystem::path(CurrentExecutableDirectory()) / L"slan_tunnel_state.json").wstring();
  STARTUPINFOW startup_info{};
  startup_info.cb = sizeof(startup_info);
  PROCESS_INFORMATION process_info{};
  std::wstring shell_command =
      L"powershell.exe -NoProfile -ExecutionPolicy Bypass -File \"" + script_path + L"\"";
  if (!CreateProcessW(
          nullptr,
          shell_command.data(),
          nullptr,
          nullptr,
          FALSE,
          CREATE_NO_WINDOW,
          nullptr,
          nullptr,
          &startup_info,
          &process_info)) {
    std::filesystem::remove(script_path);
    return "";
  }
  WaitForSingleObject(process_info.hProcess, INFINITE);
  CloseHandle(process_info.hThread);
  CloseHandle(process_info.hProcess);
  std::filesystem::remove(script_path);
  std::ifstream input(output_path);
  if (!input.is_open()) {
    return "";
  }
  std::stringstream buffer;
  buffer << input.rdbuf();
  input.close();
  std::filesystem::remove(output_path);
  return buffer.str();
}

std::string TrimAsciiWhitespace(const std::string& value) {
  const auto first = value.find_first_not_of(" \r\n\t");
  if (first == std::string::npos) {
    return "";
  }
  const auto last = value.find_last_not_of(" \r\n\t");
  return value.substr(first, last - first + 1);
}

struct TunnelInterfaceTarget {
  int interface_index = 1;
  std::wstring interface_alias = kDefaultWindowsTunnelInterfaceAlias;
  bool is_dedicated_adapter = false;
};

TunnelInterfaceTarget ResolveTunnelInterfaceTarget() {
  TunnelInterfaceTarget target;
  if (const auto configured_alias =
          GetEnvironmentPath(L"SLAN_WINDOWS_TUNNEL_INTERFACE_ALIAS")) {
    if (!configured_alias->empty()) {
      target.interface_alias = *configured_alias;
      return target;
    }
  }

  const std::wstring output_path =
      (std::filesystem::path(CurrentExecutableDirectory()) / L"slan_tunnel_state.json")
          .wstring();
  std::ostringstream route_command;
  route_command << "Set-Content -Path '" << WideToUtf8(output_path)
                << "' -Value ((route print | Out-String))\n";
  const std::string route_output =
      RunPowerShellCapture(Utf8ToWide(route_command.str()));
  std::istringstream route_lines(route_output);
  std::string route_line;
  while (std::getline(route_lines, route_line)) {
    if (route_line.find("Microsoft KM-TEST") == std::string::npos) {
      continue;
    }
    std::istringstream parser(route_line);
    int parsed_index = 0;
    if (parser >> parsed_index) {
      target.interface_index = parsed_index;
      target.is_dedicated_adapter = true;
      break;
    }
  }
  if (!target.is_dedicated_adapter) {
    return target;
  }

  std::ostringstream netsh_command;
  netsh_command << "Set-Content -Path '" << WideToUtf8(output_path)
                << "' -Value ((netsh interface ipv4 show interfaces | Out-String))\n";
  const std::string netsh_output =
      RunPowerShellCapture(Utf8ToWide(netsh_command.str()));
  std::istringstream netsh_lines(netsh_output);
  std::string netsh_line;
  while (std::getline(netsh_lines, netsh_line)) {
    std::istringstream parser(netsh_line);
    int parsed_index = 0;
    if (!(parser >> parsed_index) || parsed_index != target.interface_index) {
      continue;
    }
    std::string metric;
    std::string mtu;
    std::string state;
    if (!(parser >> metric >> mtu >> state)) {
      break;
    }
    std::string alias;
    std::getline(parser, alias);
    alias = TrimAsciiWhitespace(alias);
    if (!alias.empty()) {
      target.interface_alias = Utf8ToWide(alias);
    }
    break;
  }
  return target;
}

}  // namespace

class HelperBridgeClient {
 public:
  HelperBridgeClient() = default;
  ~HelperBridgeClient() { Close(); }

  bool Invoke(
      const std::string& method_name,
      const flutter::EncodableValue* arguments,
      std::string* response,
      std::string* error_message) {
    if (!EnsureStarted(error_message)) {
      return false;
    }

    const std::string request = BuildHelperRequestJson(method_name, arguments);
    DWORD bytes_written = 0;
    if (!WriteFile(
            stdin_write_, request.data(), static_cast<DWORD>(request.size()),
            &bytes_written, nullptr) ||
        bytes_written != request.size()) {
      if (error_message != nullptr) {
        *error_message = "Failed to write request to app-core helper.";
      }
      return false;
    }

    return ReadResponseLine(response, error_message);
  }

 private:
  bool EnsureStarted(std::string* error_message) {
    if (process_ != nullptr) {
      return true;
    }

    SECURITY_ATTRIBUTES security_attributes{};
    security_attributes.nLength = sizeof(SECURITY_ATTRIBUTES);
    security_attributes.bInheritHandle = TRUE;

    HANDLE stdout_read = nullptr;
    HANDLE stdout_write = nullptr;
    if (!CreatePipe(&stdout_read, &stdout_write, &security_attributes, 0)) {
      if (error_message != nullptr) {
        *error_message = "Failed to create stdout pipe for app-core helper.";
      }
      return false;
    }

    HANDLE stdin_read = nullptr;
    HANDLE stdin_write = nullptr;
    if (!CreatePipe(&stdin_read, &stdin_write, &security_attributes, 0)) {
      CloseHandle(stdout_read);
      CloseHandle(stdout_write);
      if (error_message != nullptr) {
        *error_message = "Failed to create stdin pipe for app-core helper.";
      }
      return false;
    }

    SetHandleInformation(stdout_read, HANDLE_FLAG_INHERIT, 0);
    SetHandleInformation(stdin_write, HANDLE_FLAG_INHERIT, 0);

    STARTUPINFOW startup_info{};
    startup_info.cb = sizeof(STARTUPINFOW);
    startup_info.dwFlags = STARTF_USESTDHANDLES;
    startup_info.hStdInput = stdin_read;
    startup_info.hStdOutput = stdout_write;
    startup_info.hStdError = GetStdHandle(STD_ERROR_HANDLE);

    PROCESS_INFORMATION process_info{};
    std::wstring command_line = L"\"" + ResolveHelperExecutablePath() + L"\"";
    BOOL started = CreateProcessW(
        nullptr, command_line.data(), nullptr, nullptr, TRUE, 0, nullptr,
        nullptr, &startup_info, &process_info);

    CloseHandle(stdin_read);
    CloseHandle(stdout_write);

    if (!started) {
      CloseHandle(stdout_read);
      CloseHandle(stdin_write);
      if (error_message != nullptr) {
        *error_message =
            "Failed to launch app-core helper. Set SLAN_APP_CORE_HELPER to the Rust helper executable path.";
      }
      return false;
    }

    process_ = process_info.hProcess;
    stdin_write_ = stdin_write;
    stdout_read_ = stdout_read;
    CloseHandle(process_info.hThread);
    return true;
  }

  bool ReadResponseLine(std::string* response, std::string* error_message) {
    response->clear();
    char byte = 0;
    DWORD bytes_read = 0;
    while (true) {
      if (!ReadFile(stdout_read_, &byte, 1, &bytes_read, nullptr) ||
          bytes_read == 0) {
        if (error_message != nullptr) {
          *error_message = "app-core helper closed stdout unexpectedly.";
        }
        return false;
      }
      if (byte == '\n') {
        return true;
      }
      response->push_back(byte);
    }
  }

  void Close() {
    if (stdin_write_ != nullptr) {
      CloseHandle(stdin_write_);
      stdin_write_ = nullptr;
    }
    if (stdout_read_ != nullptr) {
      CloseHandle(stdout_read_);
      stdout_read_ = nullptr;
    }
    if (process_ != nullptr) {
      TerminateProcess(process_, 0);
      CloseHandle(process_);
      process_ = nullptr;
    }
  }

  HANDLE process_ = nullptr;
  HANDLE stdin_write_ = nullptr;
  HANDLE stdout_read_ = nullptr;
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

std::optional<std::string> SlanAppCorePluginWindowsPlugin::ForwardToHelper(
    const std::string& method_name,
    const flutter::EncodableValue* arguments,
    std::string* error_message) {
  std::lock_guard<std::mutex> lock(helper_mutex_);
  if (!helper_) {
    helper_ = std::make_unique<HelperBridgeClient>();
  }
  std::string response;
  if (!helper_->Invoke(method_name, arguments, &response, error_message)) {
    return std::nullopt;
  }
  return response;
}

void SlanAppCorePluginWindowsPlugin::HandleMethodCall(
    const flutter::MethodCall<flutter::EncodableValue>& method_call,
    std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>> result) {
  if (HandleTunnelMethodCall(method_call, result)) {
    return;
  }
  std::string error_message;
  const auto response = ForwardToHelper(
      method_call.method_name(), method_call.arguments(), &error_message);
  if (!response.has_value()) {
    result->Error("app_core_process_start_failed", error_message);
    return;
  }
  result->Success(flutter::EncodableValue(*response));
}

bool SlanAppCorePluginWindowsPlugin::HandleTunnelMethodCall(
    const flutter::MethodCall<flutter::EncodableValue>& method_call,
    std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>>& result) {
  const std::string& method = method_call.method_name();
  if (method != "applyTunnelConfiguration" && method != "bringTunnelUp" &&
      method != "bringTunnelDown" && method != "removeTunnelPeer" &&
      method != "tunnelRuntimeView") {
    return false;
  }

  std::lock_guard<std::mutex> lock(tunnel_mutex_);
  if (method == "applyTunnelConfiguration") {
    const auto address_and_prefix = ParseAddressAndPrefix(method_call.arguments());
    tunnel_local_virtual_ip_ =
        address_and_prefix.has_value() ? std::optional<std::string>(address_and_prefix->first)
                                       : std::nullopt;
    tunnel_local_prefix_len_ =
        address_and_prefix.has_value() ? std::optional<int>(address_and_prefix->second)
                                       : std::nullopt;
    tunnel_peer_virtual_ip_ = ExtractPeerVirtualIp(method_call.arguments());
    tunnel_last_error_.reset();
    tunnel_running_ = false;
    const auto peer_virtual_ip = tunnel_peer_virtual_ip_.value_or("");
    result->Success(flutter::EncodableValue(JsonEnvelope(
        true,
        TunnelActionResultJson(
            "applyTunnelConfiguration",
            true,
            "configured",
            "Windows plugin accepted tunnel configuration.",
            true,
            peer_virtual_ip,
            false,
            tunnel_last_error_))));
    return true;
  }

  const TunnelInterfaceTarget interface_target = ResolveTunnelInterfaceTarget();
  const std::string interface_alias_utf8 = WideToUtf8(interface_target.interface_alias);
  const int interface_index = interface_target.interface_index;
  const std::string peer_virtual_ip = tunnel_peer_virtual_ip_.value_or("");

  if (!tunnel_local_virtual_ip_.has_value() || !tunnel_local_prefix_len_.has_value()) {
    if (method == "bringTunnelDown" || method == "removeTunnelPeer") {
      if (interface_target.is_dedicated_adapter) {
        std::ostringstream cleanup_script;
        cleanup_script << "$ErrorActionPreference='Stop'\n";
        cleanup_script
            << "Get-NetIPAddress -InterfaceIndex " << interface_index
            << " -AddressFamily IPv4 -ErrorAction SilentlyContinue | "
               "Remove-NetIPAddress -Confirm:$false -ErrorAction SilentlyContinue\n"
            << "Disable-NetAdapter -InterfaceIndex " << interface_index
            << " -Confirm:$false -ErrorAction SilentlyContinue | Out-Null\n";
        std::string ignored_error;
        RunElevatedPowerShellScript(Utf8ToWide(cleanup_script.str()), &ignored_error);
      }
      tunnel_running_ = false;
      tunnel_last_error_.reset();
      if (method == "removeTunnelPeer") {
        tunnel_peer_virtual_ip_.reset();
      }
      result->Success(flutter::EncodableValue(JsonEnvelope(
          true,
          TunnelActionResultJson(
              method,
              true,
              "verified",
              interface_target.is_dedicated_adapter
                  ? "Windows plugin disabled the SLAN adapter."
                  : "Windows plugin found no staged tunnel configuration.",
              false,
              "",
              false,
              tunnel_last_error_))));
      return true;
    }
    result->Success(flutter::EncodableValue(JsonEnvelope(
        true,
        TunnelActionResultJson(
            method,
            false,
            "failed",
            "Missing tunnel configuration.",
            false,
            "",
            false,
            std::optional<std::string>("missing tunnel configuration")))));
    return true;
  }

  if (method == "bringTunnelUp") {
    std::ostringstream script;
    script
        << "$ErrorActionPreference='Stop'\n"
        << "if (Get-NetAdapter -InterfaceIndex " << interface_index
        << " -ErrorAction SilentlyContinue) {\n";
    if (interface_target.is_dedicated_adapter) {
      script
          << "  Enable-NetAdapter -InterfaceIndex " << interface_index
          << " -Confirm:$false -ErrorAction SilentlyContinue | Out-Null\n"
          << "  Set-NetIPInterface -InterfaceIndex " << interface_index
          << " -Dhcp Disabled -ErrorAction SilentlyContinue | Out-Null\n";
    }
    script
        << "  Get-NetIPAddress -InterfaceIndex " << interface_index
        << " -AddressFamily IPv4 -ErrorAction SilentlyContinue | "
           "Remove-NetIPAddress -Confirm:$false -ErrorAction SilentlyContinue\n"
        << "  New-NetIPAddress -InterfaceIndex " << interface_index
        << " -IPAddress " << *tunnel_local_virtual_ip_
        << " -PrefixLength " << *tunnel_local_prefix_len_
        << " -AddressFamily IPv4 -Type Unicast | Out-Null\n"
        << "} else {\n"
        << "  throw 'Tunnel interface is unavailable.'\n"
        << "}\n";
    std::string error_message;
    if (!RunElevatedPowerShellScript(Utf8ToWide(script.str()), &error_message)) {
      tunnel_running_ = false;
      tunnel_last_error_ = error_message;
      result->Success(flutter::EncodableValue(JsonEnvelope(
          true,
          TunnelActionResultJson(
              method,
              false,
              "failed",
              "Windows plugin failed to assign the virtual IP: " + error_message,
              true,
              peer_virtual_ip,
              false,
              tunnel_last_error_))));
      return true;
    }
    tunnel_running_ = true;
    tunnel_last_error_.reset();
    result->Success(flutter::EncodableValue(JsonEnvelope(
        true,
        TunnelActionResultJson(
            method,
            true,
            "started",
            "Windows plugin assigned the virtual IP.",
            true,
            peer_virtual_ip,
            true,
            tunnel_last_error_))));
    return true;
  }

  if (method == "bringTunnelDown" || method == "removeTunnelPeer") {
    std::ostringstream script;
    script << "$ErrorActionPreference='Stop'\n"
           << "Get-NetIPAddress -InterfaceIndex " << interface_index
           << " -AddressFamily IPv4 -ErrorAction SilentlyContinue | "
              "Remove-NetIPAddress -Confirm:$false -ErrorAction SilentlyContinue\n";
    if (interface_target.is_dedicated_adapter) {
      script << "Disable-NetAdapter -InterfaceIndex " << interface_index
             << " -Confirm:$false -ErrorAction SilentlyContinue | Out-Null\n";
    }
    std::string ignored_error;
    RunElevatedPowerShellScript(Utf8ToWide(script.str()), &ignored_error);
    tunnel_running_ = false;
    tunnel_last_error_.reset();
    if (method == "removeTunnelPeer") {
      tunnel_local_virtual_ip_.reset();
      tunnel_local_prefix_len_.reset();
      tunnel_peer_virtual_ip_.reset();
    }
    result->Success(flutter::EncodableValue(JsonEnvelope(
        true,
        TunnelActionResultJson(
            method,
            true,
            "verified",
            method == "bringTunnelDown"
                ? (interface_target.is_dedicated_adapter
                       ? "Windows plugin removed the virtual IP and disabled the SLAN adapter."
                       : "Windows plugin removed the virtual IP.")
                : (interface_target.is_dedicated_adapter
                       ? "Windows plugin cleared the tunnel peer and disabled the SLAN adapter."
                       : "Windows plugin cleared the tunnel peer."),
            tunnel_local_virtual_ip_.has_value(),
            peer_virtual_ip,
            false,
            tunnel_last_error_))));
    return true;
  }

  if (method == "tunnelRuntimeView") {
    std::string detected_ip;
    if (tunnel_local_virtual_ip_.has_value()) {
      const std::wstring output_path =
          (std::filesystem::path(CurrentExecutableDirectory()) / L"slan_tunnel_state.json")
              .wstring();
      std::ostringstream command;
      command
          << "$value = Get-NetIPAddress -InterfaceIndex " << interface_index
          << " -AddressFamily IPv4 -ErrorAction SilentlyContinue | "
             "Where-Object {$_.IPAddress -eq '" << *tunnel_local_virtual_ip_
          << "'} | Select-Object -First 1 -ExpandProperty IPAddress; "
          << "Set-Content -Path '" << WideToUtf8(output_path) << "' -Value ($value | Out-String)\n";
      detected_ip = RunPowerShellCapture(Utf8ToWide(command.str()));
    }
    const bool ip_present =
        detected_ip.find(tunnel_local_virtual_ip_.value_or("")) !=
        std::string::npos;
    std::ostringstream runtime;
    runtime << "{"
            << "\"state\":\"" << EscapeJsonString(ip_present ? "configured" : "disconnected") << "\""
            << ",\"transport\":\"relay\""
            << ",\"debugEngineMode\":\"noop\""
            << ",\"backendName\":\"windows-plugin\""
            << ",\"backendState\":\""
            << EscapeJsonString(tunnel_last_error_.has_value() ? "failed"
                                                               : (ip_present ? "started" : "idle"))
            << "\""
            << ",\"backendLastError\":";
    if (tunnel_last_error_.has_value()) {
      runtime << "\"" << EscapeJsonString(*tunnel_last_error_) << "\"";
    } else {
      runtime << "null";
    }
    runtime << ",\"backendLastStartedAtMs\":null"
            << ",\"backendPeerVirtualIp\":\"" << EscapeJsonString(peer_virtual_ip) << "\""
            << ",\"backendSelectedEndpoint\":null"
            << ",\"peerVirtualIp\":\"" << EscapeJsonString(peer_virtual_ip) << "\""
            << ",\"peerPublicKey\":\"peer-debug-public-key\""
            << ",\"selectedEndpoint\":null"
            << ",\"interfaceName\":\"" << EscapeJsonString(interface_alias_utf8) << "\""
            << ",\"dnsServers\":[\"1.1.1.1\"]"
            << ",\"allowedIps\":[\"" << EscapeJsonString(peer_virtual_ip + "/32") << "\"]"
            << ",\"localVirtualIp\":\""
            << EscapeJsonString(tunnel_local_virtual_ip_.value_or(""))
            << "\""
            << ",\"remoteAddress\":\"\""
            << ",\"mtu\":1280"
            << ",\"interfaceAddresses\":[\""
            << EscapeJsonString(
                   tunnel_local_virtual_ip_.has_value() && tunnel_local_prefix_len_.has_value()
                       ? *tunnel_local_virtual_ip_ + "/" +
                             std::to_string(*tunnel_local_prefix_len_)
                       : "")
            << "\"]"
            << ",\"includedRoutes\":[\"" << EscapeJsonString(peer_virtual_ip + "/32") << "\"]"
            << ",\"packetRxCount\":0,\"packetRxBytes\":0,\"packetTxCount\":0,\"packetTxBytes\":0"
            << ",\"lastPacketAtMs\":null,\"lastAppliedAtMs\":null,\"lastError\":";
    if (tunnel_last_error_.has_value()) {
      runtime << "\"" << EscapeJsonString(*tunnel_last_error_) << "\"";
    } else {
      runtime << "null";
    }
    runtime << "}";
    result->Success(flutter::EncodableValue(JsonEnvelope(true, runtime.str())));
    return true;
  }

  return false;
}

}  // namespace slan_app_core_plugin_windows
