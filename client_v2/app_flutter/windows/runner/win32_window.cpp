#ifndef WIN32_LEAN_AND_MEAN
#define WIN32_LEAN_AND_MEAN
#endif
#include <winsock2.h>
#include <ws2tcpip.h>

#include "win32_window.h"

#include <dwmapi.h>
#include <flutter_windows.h>
#include <shellapi.h>

#include "resource.h"

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

/// Registry key for app theme preference.
///
/// A value of 0 indicates apps should use dark mode. A non-zero or missing
/// value indicates apps should use light mode.
constexpr const wchar_t kGetPreferredBrightnessRegKey[] =
  L"Software\\Microsoft\\Windows\\CurrentVersion\\Themes\\Personalize";
constexpr const wchar_t kGetPreferredBrightnessRegValue[] = L"AppsUseLightTheme";

// The number of Win32Window objects that currently exist.
static int g_active_window_count = 0;

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

void CenterWindowOnCurrentMonitor(HWND window) {
  RECT window_rect;
  if (!GetWindowRect(window, &window_rect)) {
    return;
  }

  HMONITOR monitor = MonitorFromWindow(window, MONITOR_DEFAULTTONEAREST);
  MONITORINFO monitor_info{};
  monitor_info.cbSize = sizeof(MONITORINFO);
  if (!GetMonitorInfo(monitor, &monitor_info)) {
    return;
  }

  const int window_width = window_rect.right - window_rect.left;
  const int window_height = window_rect.bottom - window_rect.top;
  const RECT work_area = monitor_info.rcWork;
  const int x = work_area.left + ((work_area.right - work_area.left) - window_width) / 2;
  const int y = work_area.top + ((work_area.bottom - work_area.top) - window_height) / 2;

  SetWindowPos(
      window, nullptr, x, y, 0, 0,
      SWP_NOSIZE | SWP_NOZORDER | SWP_NOACTIVATE);
}

}  // namespace

void AddTrayIcon(HWND window) {
  NOTIFYICONDATA notify_icon{};
  notify_icon.cbSize = sizeof(NOTIFYICONDATA);
  notify_icon.hWnd = window;
  notify_icon.uID = kTrayIconId;
  notify_icon.uFlags = NIF_MESSAGE | NIF_ICON | NIF_TIP;
  notify_icon.uCallbackMessage = kTrayIconMessage;
  notify_icon.hIcon = LoadIcon(GetModuleHandle(nullptr), MAKEINTRESOURCE(IDI_APP_ICON));
  wcscpy_s(notify_icon.szTip, L"SLAN Client V2");
  Shell_NotifyIcon(NIM_ADD, &notify_icon);
}

void RemoveTrayIcon(HWND window) {
  NOTIFYICONDATA notify_icon{};
  notify_icon.cbSize = sizeof(NOTIFYICONDATA);
  notify_icon.hWnd = window;
  notify_icon.uID = kTrayIconId;
  Shell_NotifyIcon(NIM_DELETE, &notify_icon);
}

void ShowTrayMenu(HWND window) {
  HMENU menu = CreatePopupMenu();
  AppendMenu(menu, MF_STRING, ID_TRAY_OPEN, L"Open SLAN Client");
  AppendMenu(menu, MF_SEPARATOR, 0, nullptr);
  AppendMenu(menu, MF_STRING, ID_TRAY_QUIT, L"Quit");

  POINT cursor;
  GetCursorPos(&cursor);
  SetForegroundWindow(window);
  TrackPopupMenu(
      menu, TPM_RIGHTBUTTON | TPM_BOTTOMALIGN | TPM_LEFTALIGN,
      cursor.x, cursor.y, 0, window, nullptr);
  DestroyMenu(menu);
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

void ShutdownNetworkBeforeQuit() {
  WSADATA winsock_data{};
  if (WSAStartup(MAKEWORD(2, 2), &winsock_data) != 0) {
    return;
  }

  std::string host;
  unsigned short port = 46392;
  if (!SplitHostPort(ServiceHost(), &host, &port)) {
    WSACleanup();
    return;
  }

  addrinfo hints{};
  hints.ai_family = AF_UNSPEC;
  hints.ai_socktype = SOCK_STREAM;
  hints.ai_protocol = IPPROTO_TCP;

  addrinfo* resolved = nullptr;
  const auto port_text = std::to_string(port);
  if (getaddrinfo(host.c_str(), port_text.c_str(), &hints, &resolved) != 0) {
    WSACleanup();
    return;
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
    return;
  }

  const DWORD timeout_ms = 2000;
  setsockopt(socket, SOL_SOCKET, SO_SNDTIMEO,
             reinterpret_cast<const char*>(&timeout_ms), sizeof(timeout_ms));
  setsockopt(socket, SOL_SOCKET, SO_RCVTIMEO,
             reinterpret_cast<const char*>(&timeout_ms), sizeof(timeout_ms));

  const char request[] =
      "{\"method\":\"shutdownNetwork\",\"args\":{}}\n";
  send(socket, request, static_cast<int>(strlen(request)), 0);
  shutdown(socket, SD_SEND);

  char buffer[256];
  recv(socket, buffer, sizeof(buffer), 0);
  closesocket(socket);
  WSACleanup();
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
        case ID_TRAY_OPEN:
          RestoreWindow(hwnd);
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
      if (lparam == WM_LBUTTONDBLCLK) {
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
      if (LOWORD(wparam) != WA_INACTIVE) {
        CenterWindowOnCurrentMonitor(hwnd);
      }
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
