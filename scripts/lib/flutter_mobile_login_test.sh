#!/usr/bin/env bash

slan_mobile_login_common_defines() {
  local biz_url="$1"
  local email="$2"
  local password="$3"
  local register_user="${4:-false}"
  local wait_mqtt="${5:-true}"
  cat <<EOF
--dart-define=SLAN_TEST_BIZ_URL=$biz_url
--dart-define=SLAN_EMBEDDED_CONTROL_BASE_URL=$biz_url
--dart-define=SLAN_TEST_EMAIL=$email
--dart-define=SLAN_TEST_PASSWORD=$password
--dart-define=SLAN_TEST_REGISTER_USER=$register_user
--dart-define=SLAN_TEST_WAIT_MQTT=$wait_mqtt
EOF
}

slan_mobile_login_message_send_defines() {
  local target_device_id="$1"
  local body="$2"
  cat <<EOF
--dart-define=SLAN_TEST_SEND_TARGET_DEVICE_ID=$target_device_id
--dart-define=SLAN_TEST_SEND_BODY=$body
EOF
}

slan_mobile_login_message_expect_defines() {
  local from_device_id="$1"
  local body="$2"
  local mqtt_timeout_seconds="${3:-}"
  local message_timeout_seconds="${4:-}"
  cat <<EOF
--dart-define=SLAN_TEST_EXPECT_MESSAGE_FROM_DEVICE_ID=$from_device_id
--dart-define=SLAN_TEST_EXPECT_MESSAGE_BODY=$body
EOF
  if [[ -n "$mqtt_timeout_seconds" ]]; then
    printf '%s\n' "--dart-define=SLAN_TEST_EXPECT_MQTT_TIMEOUT_SECONDS=$mqtt_timeout_seconds"
  fi
  if [[ -n "$message_timeout_seconds" ]]; then
    printf '%s\n' "--dart-define=SLAN_TEST_EXPECT_MESSAGE_TIMEOUT_SECONDS=$message_timeout_seconds"
  fi
}
