#!/usr/bin/env bash

slan_mobile_activation_common_defines() {
  local biz_url="$1"
  local authorization_key="$2"
  local wait_mqtt="${3:-true}"
  cat <<EOF
--dart-define=SLAN_TEST_BIZ_URL=$biz_url
--dart-define=SLAN_EMBEDDED_CONTROL_BASE_URL=$biz_url
--dart-define=SLAN_TEST_DEVICE_AUTHORIZATION_KEY=$authorization_key
--dart-define=SLAN_TEST_WAIT_MQTT=$wait_mqtt
EOF
}

slan_mobile_message_send_defines() {
  local target_device_id="$1"
  local body="$2"
  cat <<EOF
--dart-define=SLAN_TEST_SEND_TARGET_DEVICE_ID=$target_device_id
--dart-define=SLAN_TEST_SEND_BODY=$body
EOF
}

slan_mobile_message_expect_defines() {
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
