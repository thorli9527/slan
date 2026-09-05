#!/usr/bin/env sh

SLAN_LINUX_APP_NAME="${SLAN_LINUX_APP_NAME:-SLAN Client V2}"
SLAN_LINUX_PACKAGE_NAME="${SLAN_LINUX_PACKAGE_NAME:-slan-client-v2}"
SLAN_LINUX_PACKAGE_VERSION="${SLAN_CLIENT_V2_VERSION:-0.1.0}"
SLAN_LINUX_INSTALL_ROOT="${SLAN_INSTALL_ROOT:-/opt/slan-client-v2}"
SLAN_LINUX_CONFIG_DIR="${SLAN_CONFIG_DIR:-/etc/slan}"
SLAN_LINUX_STATE_DIR="${SLAN_STATE_DIR:-/var/lib/slan}"
SLAN_LINUX_SERVICE_NAME="${SLAN_LINUX_SERVICE_NAME:-slan-client-v2.service}"
SLAN_LINUX_SERVICE_BIN_NAME="${SLAN_LINUX_SERVICE_BIN_NAME:-client-core-service}"
SLAN_LINUX_GUI_BIN_NAME="${SLAN_LINUX_GUI_BIN_NAME:-slan_client_v2}"
SLAN_LINUX_DEFAULT_CONTROL_BASE_URL="http://47.245.40.231:28080"
SLAN_LINUX_DEFAULT_SERVICE_HOST="127.0.0.1:46392"
SLAN_LINUX_INSTALL_ENV_NAME="client-v2-install.env"
SLAN_LINUX_RUNTIME_ENV_NAME="client-v2.env"
SLAN_LINUX_CONSOLE_ENV_NAME="client-v2-console.env"
SLAN_LINUX_BOOTSTRAP_ENV_NAME="bootstrap.env"
SLAN_LINUX_DESKTOP_POLICY_NAME="client-v2-desktop.policy"
SLAN_LINUX_SERVICE_UNIT_PATH="/usr/lib/systemd/system/$SLAN_LINUX_SERVICE_NAME"
SLAN_LINUX_ETC_SERVICE_UNIT_PATH="/etc/systemd/system/$SLAN_LINUX_SERVICE_NAME"
SLAN_LINUX_SERVICE_BIN="$SLAN_LINUX_INSTALL_ROOT/bin/$SLAN_LINUX_SERVICE_BIN_NAME"
SLAN_LINUX_GUI_BIN="$SLAN_LINUX_INSTALL_ROOT/gui/$SLAN_LINUX_GUI_BIN_NAME"
SLAN_LINUX_LIB_DIR="$SLAN_LINUX_INSTALL_ROOT/lib"
SLAN_LINUX_INSTALL_LIB="$SLAN_LINUX_LIB_DIR/slan-linux-install.sh"
SLAN_LINUX_CONSOLE_BIN="/usr/bin/slan-client-v2-console"
SLAN_LINUX_DESKTOP_FILE="/usr/share/applications/slan-client-v2.desktop"
SLAN_LINUX_RUNTIME_PROCESSES="slan_client_v2 client-core-service"

slan_linux_repo_root() {
  CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd
}

slan_linux_host_arch() {
  slan_linux_normalize_arch "$(uname -m)"
}

slan_linux_normalize_arch() {
  case "$1" in
    x86_64|amd64)
      echo "amd64"
      ;;
    aarch64|arm64)
      echo "arm64"
      ;;
    armv7l|armhf)
      echo "armhf"
      ;;
    *)
      uname -m
      ;;
  esac
}

slan_linux_service_arch() {
  service_bin="$1"

  if [ ! -f "$service_bin" ]; then
    return 1
  fi

  if command -v readelf >/dev/null 2>&1; then
    machine="$(readelf -h "$service_bin" 2>/dev/null | awk -F: '/Machine:/ {gsub(/^[ \t]+/, "", $2); print $2; exit}')"
    case "$machine" in
      *X86-64*|*"Advanced Micro Devices X86-64"*)
        echo "amd64"
        return 0
        ;;
      *AArch64*)
        echo "arm64"
        return 0
        ;;
      *ARM*)
        echo "armhf"
        return 0
        ;;
    esac
  fi

  if command -v file >/dev/null 2>&1; then
    description="$(file "$service_bin" 2>/dev/null || true)"
    case "$description" in
      *ELF*x86-64*|*ELF*x86_64*)
        echo "amd64"
        return 0
        ;;
      *ELF*aarch64*|*ELF*AArch64*|*ELF*ARM64*)
        echo "arm64"
        return 0
        ;;
      *ELF*ARM*)
        echo "armhf"
        return 0
        ;;
    esac
  fi

  return 1
}

slan_linux_flutter_arch_dir() {
  case "$1" in
    amd64)
      echo "x64"
      ;;
    arm64)
      echo "arm64"
      ;;
    armhf)
      echo "arm"
      ;;
    *)
      echo "$1"
      ;;
  esac
}

slan_linux_safe_install_root() {
  case "$1" in
    ""|"/"|"/bin"|"/etc"|"/lib"|"/opt"|"/sbin"|"/usr"|"/var")
      return 1
      ;;
  esac
  return 0
}

slan_linux_stop_runtime() {
  if command -v systemctl >/dev/null 2>&1; then
    systemctl stop "$SLAN_LINUX_SERVICE_NAME" >/dev/null 2>&1 || true
    systemctl disable "$SLAN_LINUX_SERVICE_NAME" >/dev/null 2>&1 || true
    systemctl reset-failed "$SLAN_LINUX_SERVICE_NAME" >/dev/null 2>&1 || true
  fi

  for process_name in $SLAN_LINUX_RUNTIME_PROCESSES; do
    pkill -x "$process_name" >/dev/null 2>&1 || true
  done

  # Linux truncates process names to 15 bytes, so client-core-service cannot
  # reliably be stopped with pkill -x. Match only the installed executables.
  pkill -f "^$SLAN_LINUX_SERVICE_BIN([[:space:]]|$)" >/dev/null 2>&1 || true
  pkill -f "^$SLAN_LINUX_GUI_BIN([[:space:]]|$)" >/dev/null 2>&1 || true

  stop_attempt=0
  while pgrep -f "^$SLAN_LINUX_SERVICE_BIN([[:space:]]|$)" >/dev/null 2>&1 && \
    [ "$stop_attempt" -lt 20 ]; do
    stop_attempt=$((stop_attempt + 1))
    sleep 1
  done

  rm -f "$SLAN_LINUX_ETC_SERVICE_UNIT_PATH" "$SLAN_LINUX_SERVICE_UNIT_PATH"
  if command -v systemctl >/dev/null 2>&1; then
    systemctl daemon-reload >/dev/null 2>&1 || true
  fi
}

slan_linux_restart_service() {
  if command -v systemctl >/dev/null 2>&1 && \
    { [ -f "$SLAN_LINUX_ETC_SERVICE_UNIT_PATH" ] || [ -f "$SLAN_LINUX_SERVICE_UNIT_PATH" ]; }; then
    systemctl daemon-reload >/dev/null 2>&1 || true
    systemctl enable "$SLAN_LINUX_SERVICE_NAME" >/dev/null 2>&1 || true
    systemctl restart "$SLAN_LINUX_SERVICE_NAME" >/dev/null 2>&1 || true
  fi
}

slan_linux_write_runtime_env_example() {
  env_path="$1"
  cat > "$env_path" <<EOF
SLAN_CONTROL_BASE_URL=$SLAN_LINUX_DEFAULT_CONTROL_BASE_URL
SLAN_CLIENT_CORE_SERVICE_HOST=$SLAN_LINUX_DEFAULT_SERVICE_HOST
EOF
}
