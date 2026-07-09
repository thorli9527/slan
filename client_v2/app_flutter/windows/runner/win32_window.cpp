#ifndef WIN32_LEAN_AND_MEAN
#define WIN32_LEAN_AND_MEAN
#endif
#include <winsock2.h>
#include <ws2tcpip.h>

#include "win32_window.h"

#include <dwmapi.h>
#include <flutter_windows.h>
#include <shellapi.h>
#include <windowsx.h>

#include "resource.h"
#include "utils.h"

#include <cstdlib>
#include <cstring>
#include <string>

namespace {

/// Window attribute that enables dark mode window decorations.
///
/// Redefined in case the developer's machine has a Windows SDK older than
/// version 10.0.22000.0.
/// See: https://docs.microsoft.com/windows/win32/api/dwmapi/ne-dwmapi-dwmwindowattribute
#ifndef DWMWA_USE_IMMERSIVE_DARK_MODE
#define DWMWA_USE_IMMERSIVE_DARK_MODE 20
#endif

constexpr const wchar_t kWindowClassName[] = L"FLUTTER_RUNNER_WIN32_WINDOW";
constexpr UINT kTrayIconMessage = WM_APP + 1;
constexpr UINT kTrayIconId = 1;
constexpr const wchar_t kTrayOpenTitle[] = L"Open";
constexpr const wchar_t kTrayNetworkTitle[] = L"Network";
constexpr const wchar_t kTrayQuitTitle[] = L"Quit";
constexpr const wchar_t kTrayTooltipUnavailable[] =
    L"SLAN Client V2 - Service unavailable";
constexpr const wchar_t kTrayTooltipEnabled[] =
    L"SLAN Client V2 - Network enabled";
constexpr const wchar_t kTrayTooltipDisabled[] =
    L"SLAN Client V2 - Network disabled";
constexpr const wchar_t kTrayTooltipSignedOut[] =
    L"SLAN Client V2 - Signed out";

/// Registry key for app theme preference.
///
/// A value of 0 indicates apps should use dark mode. A non-zero or missing
/// value indicates apps should use light mode.
constexpr const wchar_t kGetPreferredBrightnessRegKey[] =
  L"Software\\Microsoft\\Windows\\CurrentVersion\\Themes\\Personalize";
constexpr const wchar_t kGetPreferredBrightnessRegValue[] = L"AppsUseLightTheme";

// The number of Win32Window objects that currently exist.
static int g_active_window_count = 0;

struct TrayServiceState {
  bool reachable = false;
  bool signed_in = false;
  bool network_enabled = false;
  bool syncing = false;
  bool switch_enabled = false;
};

TrayServiceState QueryTrayServiceState();
void ToggleNetworkFromTray();
void UpdateTrayIconState(HWND window, const TrayServiceState& state);
HICON CreateVLTrayIcon(bool network_enabled, bool service_available);
bool IsTrayNetworkActionEnabled(const TrayServiceState& state);
const wchar_t* TrayTooltip(const TrayServiceState& state);

using EnableNonClientDpiScaling = BOOL __stdcall(HWND hwnd);

// Scale helper to convert logical scaler values to physical using passed in
// scale factor
int Scale(int source, double scale_factor) {
  return static_cast<int>(source * scale_factor);
}

// Dynamically loads the |EnableNonClientDpiScaling| from the User32 module.
// This API is only needed for PerMonitor V1 awareness mode.
void EnableFullDpiSupportIfAvailable(HWND hwnd) {
  HMODULE user32_module = LoadLibraryA("User32.dll");
  if (!user32_module) {
    return;
  }
  auto enable_non_client_dpi_scaling =
      reinterpret_cast<EnableNonClientDpiScaling*>(
          GetProcAddress(user32_module, "EnableNonClientDpiScaling"));
  if (enable_non_client_dpi_scaling != nullptr) {
    enable_non_client_dpi_scaling(hwnd);
  }
  FreeLibrary(user32_module);
}

}  // namespace

void AddTrayIcon(HWND window) {
  NOTIFYICONDATA notify_icon{};
  notify_icon.cbSize = sizeof(NOTIFYICONDATA);
  notify_icon.hWnd = window;
  notify_icon.uID = kTrayIconId;
  notify_icon.uFlags = NIF_MESSAGE | NIF_ICON | NIF_TIP;
  notify_icon.uCallbackMessage = kTrayIconMessage;
  notify_icon.hIcon = CreateVLTrayIcon(false, false);
  wcscpy_s(notify_icon.szTip, L"SLAN Client V2");
  Shell_NotifyIcon(NIM_ADD, &notify_icon);
  DestroyIcon(notify_icon.hIcon);
  UpdateTrayIconState(window, QueryTrayServiceState());
}

void RemoveTrayIcon(HWND window) {
  NOTIFYICONDATA notify_icon{};
  notify_icon.cbSize = sizeof(NOTIFYICONDATA);
  notify_icon.hWnd = window;
  notify_icon.uID = kTrayIconId;
  Shell_NotifyIcon(NIM_DELETE, &notify_icon);
}

HICON CreateVLTrayIcon(bool network_enabled, bool service_available) {
  constexpr int kSize = 32;
  HDC screen_dc = GetDC(nullptr);
  HDC memory_dc = CreateCompatibleDC(screen_dc);
  HBITMAP color_bitmap = CreateCompatibleBitmap(screen_dc, kSize, kSize);
  HBITMAP old_bitmap = static_cast<HBITMAP>(SelectObject(memory_dc, color_bitmap));

  const COLORREF background = network_enabled
      ? RGB(10, 102, 242)
      : (service_available ? RGB(105, 116, 135) : RGB(210, 54, 48));
  HBRUSH background_brush = CreateSolidBrush(background);
  RECT rect{0, 0, kSize, kSize};
  FillRect(memory_dc, &rect, background_brush);
  DeleteObject(background_brush);

  HFONT font = CreateFontW(
      -18, 0, 0, 0, FW_BOLD, FALSE, FALSE, FALSE, DEFAULT_CHARSET,
      OUT_OUTLINE_PRECIS, CLIP_DEFAULT_PRECIS, CLEARTYPE_QUALITY,
      DEFAULT_PITCH | FF_SWISS, L"Segoe UI");
  HFONT old_font = static_cast<HFONT>(SelectObject(memory_dc, font));
  SetBkMode(memory_dc, TRANSPARENT);
  SetTextColor(memory_dc, RGB(255, 255, 255));
  DrawTextW(memory_dc, L"VL", -1, &rect, DT_CENTER | DT_VCENTER | DT_SINGLELINE);
  SelectObject(memory_dc, old_font);
  DeleteObject(font);
  SelectObject(memory_dc, old_bitmap);
  DeleteDC(memory_dc);
  ReleaseDC(nullptr, screen_dc);

  HBITMAP mask_bitmap = CreateBitmap(kSize, kSize, 1, 1, nullptr);
  ICONINFO icon_info{};
  icon_info.fIcon = TRUE;
  icon_info.hbmMask = mask_bitmap;
  icon_info.hbmColor = color_bitmap;
  HICON icon = CreateIconIndirect(&icon_info);
  DeleteObject(color_bitmap);
  DeleteObject(mask_bitmap);
  return icon;
}

void ShowTrayMenu(HWND window) {
  const TrayServiceState state = QueryTrayServiceState();
  UpdateTrayIconState(window, state);
  HMENU menu = CreatePopupMenu();
  AppendMenu(menu, MF_STRING, ID_TRAY_SETTINGS, kTrayOpenTitle);
  UINT network_flags = MF_STRING;
  network_flags |= IsTrayNetworkActionEnabled(state) ? MF_ENABLED : MF_GRAYED;
  network_flags |= state.network_enabled ? MF_CHECKED : MF_UNCHECKED;
  AppendMenu(menu, network_flags, ID_TRAY_NETWORK, kTrayNetworkTitle);
  AppendMenu(menu, MF_SEPARATOR, 0, nullptr);
  AppendMenu(menu, MF_STRING, ID_TRAY_QUIT, kTrayQuitTitle);

  POINT cursor;
  GetCursorPos(&cursor);
  SetForegroundWindow(window);
  TrackPopupMenu(
      menu, TPM_RIGHTBUTTON | TPM_BOTTOMALIGN | TPM_LEFTALIGN,
      cursor.x, cursor.y, 0, window, nullptr);
  DestroyMenu(menu);
}

void UpdateTrayIconState(HWND window, const TrayServiceState& state) {
  NOTIFYICONDATA notify_icon{};
  notify_icon.cbSize = sizeof(NOTIFYICONDATA);
  notify_icon.hWnd = window;
  notify_icon.uID = kTrayIconId;
  notify_icon.uFlags = NIF_ICON | NIF_TIP;
  notify_icon.hIcon = CreateVLTrayIcon(state.network_enabled, state.reachable);
  const wchar_t* tip = TrayTooltip(state);
  wcscpy_s(notify_icon.szTip, tip);
  Shell_NotifyIcon(NIM_MODIFY, &notify_icon);
  DestroyIcon(notify_icon.hIcon);
}

bool IsTrayNetworkActionEnabled(const TrayServiceState& state) {
  return state.signed_in && state.switch_enabled && !state.syncing;
}

const wchar_t* TrayTooltip(const TrayServiceState& state) {
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

void RestoreWindow(HWND window) {
  if (IsIconic(window)) {
    ShowWindow(window, SW_RESTORE);
  } else {
    ShowWindow(window, SW_SHOWNORMAL);
  }
  CenterWindowOnCurrentMonitor(window);
  SetForegroundWindow(window);
}

std::string WideToUtf8(const std::wstring& value) {
  if (value.empty()) {
    return "";
  }
  const int size = WideCharToMultiByte(
      CP_UTF8, 0, value.c_str(), static_cast<int>(value.size()), nullptr, 0, nullptr, nullptr);
  std::string utf8(size, '\0');
  WideCharToMultiByte(
      CP_UTF8, 0, value.c_str(), static_cast<int>(value.size()), utf8.data(), size, nullptr, nullptr);
  return utf8;
}

std::string ServiceHost() {
  const DWORD size = GetEnvironmentVariableW(L"SLAN_CLIENT_CORE_SERVICE_HOST", nullptr, 0);
  if (size == 0) {
    return "127.0.0.1:46392";
  }
  std::wstring value(size - 1, L'\0');
  if (GetEnvironmentVariableW(L"SLAN_CLIENT_CORE_SERVICE_HOST", value.data(), size) == 0) {
    return "127.0.0.1:46392";
  }
  const auto host = WideToUtf8(value);
  return host.empty() ? "127.0.0.1:46392" : host;
}

bool SplitHostPort(const std::string& host_port, std::string* host, unsigned short* port) {
  const auto separator = host_port.rfind(':');
  if (separator == std::string::npos || separator == 0 || separator + 1 >= host_port.size()) {
    return false;
  }
  *host = host_port.substr(0, separator);
  char* end = nullptr;
  const auto port_value = std::strtoul(host_port.substr(separator + 1).c_str(), &end, 10);
  if (end == nullptr || *end != '\0') {
    return false;
  }
  if (port_value <= 0 || port_value > 65535) {
    return false;
  }
  *port = static_cast<unsigned short>(port_value);
  return true;
}

bool SendServiceCommand(const char* method, std::string* response) {
  WSADATA winsock_data{};
  if (WSAStartup(MAKEWORD(2, 2), &winsock_data) != 0) {
    return false;
  }

  std::string host;
  unsigned short port = 46392;
  if (!SplitHostPort(ServiceHost(), &host, &port)) {
    WSACleanup();
    return false;
  }

  addrinfo hints{};
  hints.ai_family = AF_UNSPEC;
  hints.ai_socktype = SOCK_STREAM;
  hints.ai_protocol = IPPROTO_TCP;

  addrinfo* resolved = nullptr;
  const auto port_text = std::to_string(port);
  if (getaddrinfo(host.c_str(), port_text.c_str(), &hints, &resolved) != 0) {
    WSACleanup();
    return false;
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
    return false;
  }

  const DWORD timeout_ms = 2000;
  setsockopt(socket, SOL_SOCKET, SO_SNDTIMEO,
             reinterpret_cast<const char*>(&timeout_ms), sizeof(timeout_ms));
  setsockopt(socket, SOL_SOCKET, SO_RCVTIMEO,
             reinterpret_cast<const char*>(&timeout_ms), sizeof(timeout_ms));

  const std::string request = std::string("{\"method\":\"") + method + "\",\"args\":{}}\n";
  send(socket, request.c_str(), static_cast<int>(request.size()), 0);
  shutdown(socket, SD_SEND);

  char buffer[4096];
  while (true) {
    const int received = recv(socket, buffer, sizeof(buffer), 0);
    if (received <= 0) {
      break;
    }
    if (response != nullptr) {
      response->append(buffer, buffer + received);
    }
  }
  closesocket(socket);
  WSACleanup();
  return true;
}

bool JsonBoolField(const std::string& json, const char* field) {
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

TrayServiceState QueryTrayServiceState() {
  std::string response;
  TrayServiceState state{};
  state.reachable = SendServiceCommand("localState", &response);
  if (!state.reachable) {
    return state;
  }
  state.signed_in = JsonBoolField(response, "signedIn");
  state.network_enabled = JsonBoolField(response, "networkEnabled");
  state.syncing = JsonBoolField(response, "syncing");
  state.switch_enabled = JsonBoolField(response, "switchEnabled");
  return state;
}

void ToggleNetworkFromTray() {
  const TrayServiceState state = QueryTrayServiceState();
  if (!state.signed_in || state.syncing || !state.switch_enabled) {
    return;
  }
  SendServiceCommand(state.network_enabled ? "localNetworkDeactivate" : "localNetworkActivate", nullptr);
}

void ShutdownNetworkBeforeQuit() {
  SendServiceCommand("localNetworkShutdown", nullptr);
}

// Manages the Win32Window's window class registration.
class WindowClassRegistrar {
 public:
  ~WindowClassRegistrar() = default;

  // Returns the singleton registrar instance.
  static WindowClassRegistrar* GetInstance() {
    if (!instance_) {
      instance_ = new WindowClassRegistrar();
    }
    return instance_;
  }

  // Returns the name of the window class, registering the class if it hasn't
  // previously been registered.
  const wchar_t* GetWindowClass();

  // Unregisters the window class. Should only be called if there are no
  // instances of the window.
  void UnregisterWindowClass();

 private:
  WindowClassRegistrar() = default;

  static WindowClassRegistrar* instance_;

  bool class_registered_ = false;
};

WindowClassRegistrar* WindowClassRegistrar::instance_ = nullptr;

const wchar_t* WindowClassRegistrar::GetWindowClass() {
  if (!class_registered_) {
    WNDCLASS window_class{};
    window_class.hCursor = LoadCursor(nullptr, IDC_ARROW);
    window_class.lpszClassName = kWindowClassName;
    window_class.style = 0;
    window_class.cbClsExtra = 0;
    window_class.cbWndExtra = 0;
    window_class.hInstance = GetModuleHandle(nullptr);
    window_class.hIcon =
        LoadIcon(window_class.hInstance, MAKEINTRESOURCE(IDI_APP_ICON));
    window_class.hbrBackground = 0;
    window_class.lpszMenuName = nullptr;
    window_class.lpfnWndProc = Win32Window::WndProc;
    RegisterClass(&window_class);
    class_registered_ = true;
  }
  return kWindowClassName;
}

void WindowClassRegistrar::UnregisterWindowClass() {
  UnregisterClass(kWindowClassName, nullptr);
  class_registered_ = false;
}

Win32Window::Win32Window() {
  ++g_active_window_count;
}

Win32Window::~Win32Window() {
  --g_active_window_count;
  Destroy();
}

bool Win32Window::Create(const std::wstring& title,
                         const Point& origin,
                         const Size& size) {
  Destroy();

  const wchar_t* window_class =
      WindowClassRegistrar::GetInstance()->GetWindowClass();

  const POINT target_point = {static_cast<LONG>(origin.x),
                              static_cast<LONG>(origin.y)};
  HMONITOR monitor = MonitorFromPoint(target_point, MONITOR_DEFAULTTONEAREST);
  UINT dpi = FlutterDesktopGetDpiForMonitor(monitor);
  double scale_factor = dpi / 96.0;

  HWND window = CreateWindow(
      window_class, title.c_str(),
      WS_OVERLAPPED | WS_CAPTION | WS_SYSMENU,
      Scale(origin.x, scale_factor), Scale(origin.y, scale_factor),
      Scale(size.width, scale_factor), Scale(size.height, scale_factor),
      nullptr, nullptr, GetModuleHandle(nullptr), this);

  if (!window) {
    return false;
  }

  UpdateTheme(window);

  return OnCreate();
}

bool Win32Window::Show() {
  CenterWindowOnCurrentMonitor(window_handle_);
  return ShowWindow(window_handle_, SW_SHOWNORMAL);
}

// static
LRESULT CALLBACK Win32Window::WndProc(HWND const window,
                                      UINT const message,
                                      WPARAM const wparam,
                                      LPARAM const lparam) noexcept {
  if (message == WM_NCCREATE) {
    auto window_struct = reinterpret_cast<CREATESTRUCT*>(lparam);
    SetWindowLongPtr(window, GWLP_USERDATA,
                     reinterpret_cast<LONG_PTR>(window_struct->lpCreateParams));

    auto that = static_cast<Win32Window*>(window_struct->lpCreateParams);
    EnableFullDpiSupportIfAvailable(window);
    that->window_handle_ = window;
  } else if (Win32Window* that = GetThisFromHandle(window)) {
    return that->MessageHandler(window, message, wparam, lparam);
  }

  return DefWindowProc(window, message, wparam, lparam);
}

LRESULT
Win32Window::MessageHandler(HWND hwnd,
                            UINT const message,
                            WPARAM const wparam,
                            LPARAM const lparam) noexcept {
  switch (message) {
    case WM_SYSCOMMAND:
      if ((wparam & 0xfff0) == SC_RESTORE) {
        CenterWindowOnCurrentMonitor(hwnd);
      }
      break;

    case WM_CLOSE:
      if (tray_mode_ && !quit_from_tray_) {
        ShowWindow(hwnd, SW_HIDE);
        return 0;
      }
      break;

    case WM_COMMAND:
      switch (LOWORD(wparam)) {
        case ID_TRAY_SETTINGS:
          RestoreWindow(hwnd);
          return 0;
        case ID_TRAY_NETWORK:
          ToggleNetworkFromTray();
          UpdateTrayIconState(hwnd, QueryTrayServiceState());
          return 0;
        case ID_TRAY_QUIT:
          ShutdownNetworkBeforeQuit();
          quit_from_tray_ = true;
          Destroy();
          PostQuitMessage(0);
          return 0;
      }
      break;

    case kTrayIconMessage:
      if (lparam == WM_LBUTTONUP) {
        UpdateTrayIconState(hwnd, QueryTrayServiceState());
        RestoreWindow(hwnd);
        return 0;
      }
      if (lparam == WM_LBUTTONDBLCLK) {
        UpdateTrayIconState(hwnd, QueryTrayServiceState());
        RestoreWindow(hwnd);
        return 0;
      }
      if (lparam == WM_RBUTTONUP || lparam == WM_CONTEXTMENU) {
        ShowTrayMenu(hwnd);
        return 0;
      }
      break;

    case WM_DESTROY:
      if (tray_mode_) {
        RemoveTrayIcon(hwnd);
      }
      window_handle_ = nullptr;
      Destroy();
      if (quit_on_close_) {
        PostQuitMessage(0);
      }
      return 0;

    case WM_DPICHANGED: {
      auto newRectSize = reinterpret_cast<RECT*>(lparam);
      LONG newWidth = newRectSize->right - newRectSize->left;
      LONG newHeight = newRectSize->bottom - newRectSize->top;

      SetWindowPos(hwnd, nullptr, newRectSize->left, newRectSize->top, newWidth,
                   newHeight, SWP_NOZORDER | SWP_NOACTIVATE);

      return 0;
    }
    case WM_SIZE: {
      RECT rect = GetClientArea();
      if (child_content_ != nullptr) {
        // Size and position the child window.
        MoveWindow(child_content_, rect.left, rect.top, rect.right - rect.left,
                   rect.bottom - rect.top, TRUE);
      }
      return 0;
    }

    case WM_ACTIVATE:
      if (child_content_ != nullptr) {
        SetFocus(child_content_);
      }
      return 0;

    case WM_DWMCOLORIZATIONCOLORCHANGED:
      UpdateTheme(hwnd);
      return 0;
  }

  return DefWindowProc(window_handle_, message, wparam, lparam);
}

void Win32Window::Destroy() {
  OnDestroy();

  if (window_handle_) {
    DestroyWindow(window_handle_);
    window_handle_ = nullptr;
  }
  if (g_active_window_count == 0) {
    WindowClassRegistrar::GetInstance()->UnregisterWindowClass();
  }
}

Win32Window* Win32Window::GetThisFromHandle(HWND const window) noexcept {
  return reinterpret_cast<Win32Window*>(
      GetWindowLongPtr(window, GWLP_USERDATA));
}

void Win32Window::SetChildContent(HWND content) {
  child_content_ = content;
  SetParent(content, window_handle_);
  RECT frame = GetClientArea();

  MoveWindow(content, frame.left, frame.top, frame.right - frame.left,
             frame.bottom - frame.top, true);

  SetFocus(child_content_);
}

RECT Win32Window::GetClientArea() {
  RECT frame;
  GetClientRect(window_handle_, &frame);
  return frame;
}

HWND Win32Window::GetHandle() {
  return window_handle_;
}

void Win32Window::SetQuitOnClose(bool quit_on_close) {
  quit_on_close_ = quit_on_close;
}

void Win32Window::SetTrayMode(bool tray_mode) {
  tray_mode_ = tray_mode;
  if (window_handle_ != nullptr && tray_mode_) {
    AddTrayIcon(window_handle_);
  }
}

bool Win32Window::OnCreate() {
  if (tray_mode_ && window_handle_ != nullptr) {
    AddTrayIcon(window_handle_);
  }
  // No-op; provided for subclasses.
  return true;
}

void Win32Window::OnDestroy() {
  if (tray_mode_ && window_handle_ != nullptr) {
    RemoveTrayIcon(window_handle_);
  }
  // No-op; provided for subclasses.
}

void Win32Window::UpdateTheme(HWND const window) {
  DWORD light_mode;
  DWORD light_mode_size = sizeof(light_mode);
  LSTATUS result = RegGetValue(HKEY_CURRENT_USER, kGetPreferredBrightnessRegKey,
                               kGetPreferredBrightnessRegValue,
                               RRF_RT_REG_DWORD, nullptr, &light_mode,
                               &light_mode_size);

  if (result == ERROR_SUCCESS) {
    BOOL enable_dark_mode = light_mode == 0;
    DwmSetWindowAttribute(window, DWMWA_USE_IMMERSIVE_DARK_MODE,
                          &enable_dark_mode, sizeof(enable_dark_mode));
  }
}
