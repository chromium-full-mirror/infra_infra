#!/bin/bash
# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

TF_KEY_FILE_DIR="/tmp/keyfile"
TF_KEY_FILE="${TF_KEY_FILE_DIR}/key.json"

echo "Checking default service key file access."
if [[ -f "${TF_KEY_FILE}" ]]; then
  echo "Service key ${TF_KEY_FILE} exists."
else
  echo "Service key ${TF_KEY_FILE} not found."
  mkdir -p "${TF_KEY_FILE_DIR}"
  # shellcheck disable=SC2154
  echo "Checking JSON_KEY_PATH variable [=${JSON_KEY_PATH}]."
  if [[ -f "${JSON_KEY_PATH}" ]]; then
    cp "${JSON_KEY_PATH}" "${TF_KEY_FILE}"
  fi
fi
export GOOGLE_APPLICATION_CREDENTIALS="${TF_KEY_FILE}"
export APE_API_KEY="${TF_KEY_FILE}"

# Set adb vendor keys
ADB_VENDOR_KEYS=$(find /tradefed/android_vendor_keys/ -name ".adb_key*" | tr '\n' ':')
export ADB_VENDOR_KEYS

# Do not report lab as clearcut users
export DISABLE_CLEARCUT=1

if [[ -z "${TF_GLOBAL_CONFIG}" && -z "${TF_GLOBAL_CONFIG_SERVER_CONFIG}"  ]]
then
  echo "Neither TF_GLOBAL_CONFIG nor TF_GLOBAL_CONFIG_SERVER_CONFIG configured."
  echo "Using TF_GLOBAL_CONFIG_SERVER_CONFIG=google/atp/gcs-lab-config-server-config."
  # Host config is always required.
  TF_GLOBAL_CONFIG_SERVER_CONFIG=google/atp/gcs-lab-config-server-config
fi

echo "Starting TradeFed console..."

if [[ -n "${TF_GLOBAL_CONFIG}" && -n "${TF_GLOBAL_CONFIG_SERVER_CONFIG}"  ]]
then
  # If both TF_GLOBAL_CONFIG and TF_GLOBAL_CONFIG_SERVER_CONFIG are configured,
  # get the TF_GLOBAL_CONFIG from TF_GLOBAL_CONFIG_SERVER_CONFIG.
  echo "Using ${TF_GLOBAL_CONFIG_SERVER_CONFIG} to fetch ${TF_GLOBAL_CONFIG}".
  /tradefed/tradefed.sh --gcs-lab-config-server:config-path "${TF_GLOBAL_CONFIG}" "$@"
elif [[ -n "${TF_GLOBAL_CONFIG}" ]]
then
  # If only TF_GLOBAL_CONFIG is configured, use bundled or local host config.
  echo "Using ${TF_GLOBAL_CONFIG}."
  /tradefed/tradefed.sh "$@"
fi
