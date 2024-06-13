#!/usr/bin/env vpython3
# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.
"""Pulls the most recent AMD RX 7600 drivers and uploads them to CIPD.

This is done by grabbing the HTML for the driver download page and scraping it.
The URL for a particular GPU is fixed and the download link/other information
on the page gets updated when a new driver is available.
"""

import argparse
import os
import re
import sys

import bs4
import certifi
import requests

DRIVER_PAGE_URL = ('https://www.amd.com/en/support/downloads/drivers.html/'
                   'graphics/radeon-rx/radeon-rx-7000-series/'
                   'amd-radeon-rx-7600.html')
# Driver downloads do not work if not referred to the download link by AMD.
REFERER = 'https://www.amd.com/'
# Needs to be non-empty for requests to be processed. Sometimes requests work
# fine as long as the UA string is non-empty, sometimes only if it looks like
# a real one. So, use a real one to be safe.
USER_AGENT = ('Mozilla/5.0 (iPhone; CPU iPhone OS 13_2 like Mac OS X) '
              'AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/120.0.0.0 '
              'Mobile/15E148 Safari/604.1')
# We are unsure if these are actually necessary for requests to be handled
# properly, but the successful workaround attempts for the hanging issue on
# certain machines that necessitates the full user agent string also included
# these headers, so include them to be safe.
HEADER_ACCEPT = ('text/html,application/xhtml+xml,application/xml;q=0.9,image/'
                 'webp,image/apng,*/*;q=0.8,application/'
                 'signed-exchange;v=b3;q=0.7')
HEADER_ACCEPT_ENCODING = 'gzip, deflate, br'
HEADER_ACCEPT_LANGUAGE = 'en-US,en;q=0.9'
# Extracts the X.Y.Z driver version.
DRIVER_VERSION_REGEX = re.compile(r'.*[^\d\.](\d+\.\d+\.\d+)[^\d\.].*')
ONE_HUNDRED_MEGABYTES = 100 * 1024 * 1024
RESPONSE_TIMEOUT = 5


def _get_html(url: str) -> str:
  """Gets the HTML response from a URL.

  Args:
    url: A string containing the URL to GET.

  Returns:
    A string containing the HTML response.
  """
  r = requests.get(
      url,
      headers={
          'User-Agent': USER_AGENT,
          'Accept': HEADER_ACCEPT,
          'Accept-Encoding': HEADER_ACCEPT_ENCODING,
          'Accept-Language': HEADER_ACCEPT_LANGUAGE,
      },
      verify=certifi.where(),
      timeout=RESPONSE_TIMEOUT)
  if r.status_code != requests.codes.ok:
    raise RuntimeError(
        'Got non-OK response code %d getting %s. Response text: %s' %
        (r.status_code, url, r.text))
  return r.text


def _get_windows_10_driver_group(soup: bs4.BeautifulSoup, url: str) -> bs4.Tag:
  """Finds the group that contains all Windows 10 drivers.

  Args:
    soup: A BeautifulSoup object that has parsed the HTML for an AMD driver
        download page.
    url: A string containing the URL that provided the HTML parsed by |soup|.

  Returns: A BeautifulSoup Tag object for the Windows 10 driver group.
  """
  os_groups = soup.find_all('div', attrs={'class': 'accordion-item'})
  windows_10_group = None
  for group in os_groups:
    button = group.find('button', attrs={'class': 'accordion-button collapsed'})
    if 'windows 10' in button.string.lower():
      windows_10_group = group
      break
  else:
    raise RuntimeError('Could not find Windows 10 group on %s' % url)
  return windows_10_group


def _get_download_button(soup: bs4.BeautifulSoup, url: str) -> bs4.Tag:
  """Finds the relevant download button for the driver.

  Specifically looks for Windows 10 consumer drivers.

  Args:
    soup: A BeautifulSoup object that has parsed the HTML for an AMD driver
        download page.
    url: A string containing the URL that provided the HTML parsed by |soup|.

  Returns: A BeautifulSoup Tag object for the download button for the driver.
  """
  windows_10_group = _get_windows_10_driver_group(soup, url)
  download_buttons = windows_10_group.find_all('a')
  matching_buttons = []
  for button in download_buttons:
    href = button.get('href', '')
    # Adrenalin is AMD's branding for their consumer driver package.
    # Looking for 'win10' is still necessary to distinguish from the
    # auto-detecting installer that downloads the drivers on the fly.
    if 'win10' in href and 'adrenalin' in href:
      matching_buttons.append(button)
  if not matching_buttons:
    raise RuntimeError('Unable to find any relevant download buttons on %s' %
                       url)
  if len(matching_buttons) > 1:
    raise RuntimeError('Found multiple relevant download buttons on %s: %s' %
                       (url, ' '.join([b['href'] for b in matching_buttons])))
  return matching_buttons[0]


def _get_driver_version(soup: bs4.BeautifulSoup, url: str) -> str:
  """Scrapes the driver version from the download page.

  Args:
    soup: A BeautifulSoup object that has parsed the HTML for an AMD driver
        download page.
    url: A string containing the URL that provided the HTML parsed by |soup|.

  Returns:
    A string containing the parsed driver version.
  """
  # Downloads are organized by OS type with several download options available
  # for each OS. We look for the Windows 10 consumer drivers.
  # The revision is a sibling of the first <strong>Revision Number</strong>
  # tag in the group, so look for that.
  os_group = _get_windows_10_driver_group(soup, url)
  # Look for the "Revision Number" <strong>, then grab the sibling <p> which
  # contains the actual revision number.
  labels = os_group.find_all('strong')
  for l in labels:
    if 'revision' not in l.string.lower():
      continue
    # Check to make sure that this is the correct driver type. Previously,
    # simply looking for the revision was sufficient, but the auto-detecting
    # installer added a revision field as well.
    grandparent = l.parent.parent
    adrenalin_label = grandparent.find(string=re.compile('Adrenalin Edition'))
    if adrenalin_label is not None:
      revision_label = l
      break
  else:
    raise RuntimeError('Unable to find revision label from %s' % url)
  revision_item = revision_label.parent.find('p')
  match = DRIVER_VERSION_REGEX.match(revision_item.string)
  if not match:
    raise RuntimeError('Unable to extract driver version from %s' % url)
  return match.group(1)


def _get_driver_binary(soup: bs4.BeautifulSoup, url: str) -> bytes:
  """Downloads the driver binary.

  Args:
    soup: A BeautifulSoup object that has parsed the HTML for an AMD driver
        download page.
    url: A string containing the URL that provided the HTML parsed by |soup|.

  Returns:
    Bytes containing the downloaded driver binary.
  """
  download_button = _get_download_button(soup, url)
  download_url = download_button['href']
  r = requests.get(
      download_url,
      # We don't set the Accept* headers here since they don't appear to be
      # necessary.
      headers={
          'User-Agent': USER_AGENT,
          'referer': REFERER,
      },
      verify=certifi.where(),
      timeout=RESPONSE_TIMEOUT)
  if r.status_code != requests.codes.ok:
    raise RuntimeError('Got non-OK status code %d downloading driver from %s' %
                       (r.status_code, download_url))
  # The drivers should be large, so if we didn't get back much data, that's
  # indicative of something going wrong or we downloaded the wrong thing.
  if len(r.content) < ONE_HUNDRED_MEGABYTES:
    raise RuntimeError(
        'Only got back %d bytes from driver download, which is much less than '
        'expected' % len(r.content))
  return r.content


def _get_soup(url: str) -> bs4.BeautifulSoup:
  html = _get_html(url)
  return bs4.BeautifulSoup(html, 'html.parser')


def cmd_get_latest_version(_: argparse.Namespace) -> None:
  soup = _get_soup(DRIVER_PAGE_URL)
  driver_version = _get_driver_version(soup, DRIVER_PAGE_URL)
  print(driver_version)


def cmd_checkout(args: argparse.Namespace) -> None:
  requested_driver_version = os.environ.get('_3PP_VERSION')
  if not requested_driver_version:
    raise RuntimeError('checkout command requires _3PP_VERSION to be set')

  soup = _get_soup(DRIVER_PAGE_URL)
  actual_version = _get_driver_version(soup, DRIVER_PAGE_URL)
  if requested_driver_version != actual_version:
    raise RuntimeError(
        'Requested driver version %s did not match available version %s' %
        (requested_driver_version, actual_version))

  driver_binary = _get_driver_binary(soup, DRIVER_PAGE_URL)
  # We use the same naming scheme as what get_url produces for consistency
  # across different driver types. This simplifies use in automation slightly.
  filename = 'raw_source_0.exe'
  with open(os.path.join(args.checkout_path, filename), 'wb') as outfile:
    outfile.write(driver_binary)


def _parse_args() -> argparse.Namespace:
  parser = argparse.ArgumentParser()
  subparsers = parser.add_subparsers(dest='action', required=True)

  get_latest_version_parser = subparsers.add_parser('latest')
  get_latest_version_parser.set_defaults(func=cmd_get_latest_version)

  # We use the "checkout" approach instead of the preferred "get_url" approach
  # since we will need to specify the referer for the download link to work.
  checkout_parser = subparsers.add_parser('checkout')
  checkout_parser.add_argument('checkout_path')
  checkout_parser.set_defaults(func=cmd_checkout)

  # 'get_url' and 'verify' commands not supported.

  return parser.parse_args()


def main() -> int:
  args = _parse_args()
  args.func(args)
  return 0


if __name__ == '__main__':
  sys.exit(main())
