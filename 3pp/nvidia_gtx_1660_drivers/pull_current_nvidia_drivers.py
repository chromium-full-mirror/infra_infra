#!/usr/bin/env vpython3
# Copyright 2023 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.
"""Pulls the most recent NVIDIA GTX 1660 GPU drivers and uploads them to CIPD.

This is done by querying the endpoint that gets an ordered list of driver
versions for a specified configuration. If the version is one that we do not
have, we can grab the download URL from the same endpoint.
"""

import argparse
import dataclasses
import json
import os
import re
import sys
from typing import Union

import requests

# The GPU constants were determined by using NVIDIA's advanced driver search at
# https://www.nvidia.com/Download/Find.aspx, entering the correct information
# for the GTX 1660, and observing the network calls that are made when the
# "Search" button is clicked. The other constants were determined by following
# what https://github.com/lord-carlos/nvidia-update does, which is a third
# party tool for checking/installing the newest NVIDIA driver.
SERIES_ID = 112  # GeForce 16 series.
PRODUCT_ID = 895  # GeForce GTX 1660.
LANGUAGE_CODE = 1033  # en-US.
SORT_ORDER = 0  # Most recent driver first.
NUMBER_OF_RESULTS = 1


@dataclasses.dataclass
class PlatformSpecificSettings:
  os_id: int
  is_whql: int
  is_dch: int
  file_extension: str


def GetPlatformSpecificSettings() -> PlatformSpecificSettings:
  _3pp_platform = os.environ['_3PP_PLATFORM']
  if _3pp_platform == 'windows-amd64':
    return PlatformSpecificSettings(
        # Windows 10 64-bit.
        os_id=57,
        # Only look for WHQL-certified drivers.
        is_whql=1,
        # Only look for DCH drivers, which are the newer Windows standard.
        is_dch=1,
        file_extension='.exe')
  elif _3pp_platform == 'linux-amd64':
    return PlatformSpecificSettings(
        # Linux 64-bit.
        os_id=12,
        # WHQL is only relevant for Windows.
        is_whql=0,
        # DCH is only relevant for Windows.
        is_dch=0,
        file_extension='.run')
  else:
    raise RuntimeError(f'Unsupported target platform {_3pp_platform}')

# When called with the correct GET parameters, this should return JSON
# information for the most recent relevant driver. Sample JSON with irrelevant
# data removed:
# {
#   "Success": "1",
#   "IDS": [
#     {
#       "downloadInfo": {
#         "ID": "204771",
#         "Version": "531.79",
#         "IsBeta": "0",
#         "IsWHQL": "1",
#         "IsDC": "1",
#         "IsActive": "1",
#         "ReleaseDateTime": "Tue May 02, 2023",
#         "DetailsURL": "https://www.nvidia.com/Download/driverResults.aspx/204771/en-us",
#         "DownloadURL": "https://us.download.nvidia.com/Windows/531.79/531.79-desktop-win10-win11-64bit-international-dch-whql.exe",
#         "DownloadURLFileSize": "895.77 MB",
#         "LanguageName": "English%20(US)",
#         "OSName": "Windows%2011",
#         "OsCode": "10.0",
#         "OSList": [
#           {
#             "OSName": "Windows%2010%2064-bit",
#             "OsCode": "10.0"
#           },
#           {
#             "OSName": "Windows%2011",
#             "OsCode": "10.0"
#           }
#         ],
#         "Is64Bit": "1",
#       }
#     }
#   ],
#   "Request": [
#     {
#       "psid": "112",
#       "pfid": "895",
#       "osID": "57",
#       "languageCode": "1033",
#       "beta": "0",
#       "isWHQL": "1",
#       "dltype": "1",
#       "sort1": "0",
#       "numberOfResults": "1"
#     }
#   ]
# }
DRIVER_CHECK_BASE_URL = (
    'https://gfwsl.geforce.com/'
    'services_toolkit/services/com/nvidia/services/AjaxDriverService.php'
    '?func=DriverManualLookup')


def GetDriverCheckUrl() -> str:
  platform_settings = GetPlatformSpecificSettings()
  return (f'{DRIVER_CHECK_BASE_URL}'
          f'&psid={SERIES_ID}'
          f'&pfid={PRODUCT_ID}'
          f'&osID={platform_settings.os_id}'
          f'&languageCode={LANGUAGE_CODE}'
          f'&isWHQL={platform_settings.is_whql}'
          f'&dch={platform_settings.is_dch}'
          f'&sort1={SORT_ORDER}'
          f'&numberOfResults={NUMBER_OF_RESULTS}')


# NVIDIA driver versions are always in the format 123.45
DRIVER_VERSION_REGEX = re.compile(r'^\d{3}\.\d{2}$')

SUCCESS_KEY = 'Success'
IDS_KEY = 'IDS'
DOWNLOAD_INFO_KEY = 'downloadInfo'
VERSION_KEY = 'Version'
DOWNLOAD_URL_KEY = 'DownloadURL'


class MalformedJsonError(RuntimeError):

  def __init__(self, field_name, json_object):
    super().__init__('Malformed JSON response without %s field. Raw JSON: %s' %
                     (field_name, json.dumps(json_object, indent=2)))


def _get_json_from_url(url: str) -> Union[dict, list]:
  """Gets the JSON response from a URL.

  Args:
    url: A string containing the URL to GET.

  Returns:
    The JSON response from the URL.
  """
  r = requests.get(url)
  if r.status_code != requests.codes.ok:
    raise RuntimeError(
        'Got non-OK response code %d getting %s. Response text: %s' %
        (r.status_code, url, r.text))
  return json.loads(r.text)


def _get_most_recent_driver_download_info() -> dict:
  """Gets the download info JSON for the most recent driver.

  Returns:
    A dict representing the value of they downloadInfo entry for the most recent
    driver.
  """
  driver_json = _get_json_from_url(GetDriverCheckUrl())
  if SUCCESS_KEY not in driver_json or int(driver_json[SUCCESS_KEY]) != 1:
    raise RuntimeError('Did not detect successful driver query. Raw JSON: %s' %
                       json.dumps(driver_json, indent=2))
  if IDS_KEY not in driver_json:
    raise MalformedJsonError(IDS_KEY, driver_json)
  if len(driver_json[IDS_KEY]) != 1:
    raise RuntimeError('Expected 1 set of driver information, got %d' %
                       len(driver_json[IDS_KEY]))
  driver_json = driver_json[IDS_KEY][0]
  if DOWNLOAD_INFO_KEY not in driver_json:
    raise MalformedJsonError(DOWNLOAD_INFO_KEY, driver_json)
  return driver_json[DOWNLOAD_INFO_KEY]


def cmd_get_latest_version() -> None:
  download_info = _get_most_recent_driver_download_info()
  if VERSION_KEY not in download_info:
    raise MalformedJsonError(VERSION_KEY, download_info)
  version = download_info[VERSION_KEY]
  if not DRIVER_VERSION_REGEX.match(version):
    raise RuntimeError('Driver version %s did not match expected format' %
                       version)
  print(version)


def cmd_get_url() -> None:
  requested_driver_version = os.environ.get('_3PP_VERSION')
  if not requested_driver_version:
    raise RuntimeError('get_url command requires _3PP_VERSION to be set')
  platform_settings = GetPlatformSpecificSettings()
  download_info = _get_most_recent_driver_download_info()
  actual_version = download_info.get(VERSION_KEY)
  if requested_driver_version != actual_version:
    raise RuntimeError(
        'Requested driver version %s did not match available version %s' %
        (requested_driver_version, actual_version))
  download_url = download_info.get(DOWNLOAD_URL_KEY)
  if not download_url:
    raise MalformedJsonError(DOWNLOAD_URL_KEY, download_info)
  download_manifest = {
      'url': [download_url],
      'ext': platform_settings.file_extension,
  }
  print(json.dumps(download_manifest))


def _parse_args() -> argparse.Namespace:
  parser = argparse.ArgumentParser()
  subparsers = parser.add_subparsers(dest='action', required=True)

  get_latest_version_parser = subparsers.add_parser('latest')
  get_latest_version_parser.set_defaults(func=cmd_get_latest_version)

  get_url_parser = subparsers.add_parser('get_url')
  get_url_parser.set_defaults(func=cmd_get_url)

  # 'checkout' and 'verify' commands not supported.

  return parser.parse_args()


def main() -> int:
  args = _parse_args()
  args.func()
  return 0


if __name__ == '__main__':
  sys.exit(main())
