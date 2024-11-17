#!/usr/bin/env vpython3
# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import json
import os
import unittest
from unittest import mock

import pull_current_nvidia_drivers as pcnd


class RequestsMockingUnittest(unittest.TestCase):

  def setUp(self):
    self._get_patcher = mock.patch.object(pcnd.requests, 'get')
    self._get_mock = self._get_patcher.start()
    self.addCleanup(self._get_patcher.stop)

  def _setJson(self, json_object):
    self._get_mock.return_value = mock.Mock(
        status_code=200, text=json.dumps(json_object))


class GetJsonFromUrlUnittest(RequestsMockingUnittest):

  def test_success(self):
    """Tests the happy path/success case when getting JSON."""
    self._setJson({'foo': 'bar'})
    self.assertEqual(pcnd._get_json_from_url('foo.com'), {'foo': 'bar'})

  def test_non_okay_status_code(self):
    """Tests behavior when a non-OK status code is reported."""
    self._get_mock.return_value = mock.Mock(status_code=404, text='error')
    with self.assertRaisesRegex(
        RuntimeError,
        'Got non-OK response code 404 getting foo.com. Response text: error'):
      pcnd._get_json_from_url('foo.com')


class GetMostRecentDriverDownloadInfoUnittest(RequestsMockingUnittest):

  def test_success(self):
    """Tests the happy path/success case when getting the download info."""
    response = {
        'Success': '1',
        'IDS': [{
            'downloadInfo': {
                'foo': 'bar',
            },
        },],
    }
    self._setJson(response)
    self.assertEqual(pcnd._get_most_recent_driver_download_info(),
                     {'foo': 'bar'})

  def test_no_success_field(self):
    """Tests behavior when no success field is present."""
    response = {
        'IDS': [{
            'downloadInfo': {
                'foo': 'bar',
            },
        },],
    }
    self._setJson(response)
    with self.assertRaisesRegex(
        RuntimeError, 'Did not detect successful driver query\\. Raw JSON: .*'):
      pcnd._get_most_recent_driver_download_info()

  def test_no_success(self):
    """Tests behavior when a non-successful query is reported."""
    response = {
        'Success': '0',
        'IDS': [{
            'downloadInfo': {
                'foo': 'bar',
            },
        },],
    }
    self._setJson(response)
    with self.assertRaisesRegex(
        RuntimeError, 'Did not detect successful driver query\\. Raw JSON: .*'):
      pcnd._get_most_recent_driver_download_info()

  def test_no_ids_field(self):
    """Tests behavior when no IDS field is present."""
    response = {
        'Success': '1',
    }
    self._setJson(response)
    with self.assertRaisesRegex(
        pcnd.MalformedJsonError,
        'Malformed JSON response without IDS field\\. Raw JSON: .*'):
      pcnd._get_most_recent_driver_download_info()

  def test_no_ids_data(self):
    """Tests behavior when an IDS field is present but empty."""
    response = {
        'Success': '1',
        'IDS': [],
    }
    self._setJson(response)
    with self.assertRaisesRegex(RuntimeError,
                                'Expected 1 set of driver information, got 0'):
      pcnd._get_most_recent_driver_download_info()

  def test_too_much_ids_data(self):
    """Tests behavior when the IDS field has too many elements."""
    response = {
        'Success':
            '1',
        'IDS': [
            {
                'downloadInfo': {
                    'foo': 'bar',
                },
            },
            {
                'downloadInfo': {
                    'bar': 'baz',
                },
            },
        ],
    }
    self._setJson(response)
    with self.assertRaisesRegex(RuntimeError,
                                'Expected 1 set of driver information, got 2'):
      pcnd._get_most_recent_driver_download_info()

  def test_no_download_info_field(self):
    """Tests behavior when no downloadInfo field is present."""
    response = {
        'Success': '1',
        'IDS': [{
            'not_downloadInfo': {
                'foo': 'bar',
            },
        },],
    }
    self._setJson(response)
    with self.assertRaisesRegex(
        pcnd.MalformedJsonError,
        'Malformed JSON response without downloadInfo field\\. Raw JSON: .*'):
      pcnd._get_most_recent_driver_download_info()


class CmdGetLatestVersionUnittest(RequestsMockingUnittest):

  def test_success(self):
    """Tests the happy path/success case when getting the latest version."""
    response = {
        'Success':
            '1',
        'IDS': [{
            'downloadInfo': {
                'DownloadURL': 'foo.com/download',
                'Version': '123.45',
            },
        },],
    }
    self._setJson(response)
    pcnd.cmd_get_latest_version()

  def test_missing_version_field(self):
    """Tests behavior when the Version field is not present."""
    response = {
        'Success': '1',
        'IDS': [{
            'downloadInfo': {
                'DownloadURL': 'foo.com/download',
            },
        },],
    }
    self._setJson(response)
    with self.assertRaisesRegex(
        pcnd.MalformedJsonError,
        'Malformed JSON response without Version field\\. Raw JSON: .*'):
      pcnd.cmd_get_latest_version()

  def test_invalid_version_format(self):
    """Tests behavior when invalid driver version formats are found."""
    bad_versions = (
        '',
        # Wrong number of digits before decimal.
        '12.45',
        '1234.45',
        '.45'
        # Wrong number of digits after decimal.
        '123.4',
        '123.456',
        '123.',
        # Extra decimals.
        '123.45.67',
        # Non-numerical.
        '12a.45',
        '123.a5',
        # Whitespace.
        ' 123.45',
        '123.45 ',
    )

    response = {
        'Success': '1',
        'IDS': [{
            'downloadInfo': {
                'DownloadURL': 'foo.com/download',
            },
        },],
    }

    for bv in bad_versions:
      response['IDS'][0]['downloadInfo']['Version'] = bv
      self._setJson(response)
      with self.assertRaisesRegex(
          RuntimeError, 'Driver version %s did not match expected format' % bv):
        pcnd.cmd_get_latest_version()


class CmdGetUrlUnittest(RequestsMockingUnittest):

  @mock.patch.dict(os.environ, {'_3PP_VERSION': '123.45'}, clear=True)
  def test_success(self):
    """Tests the happy path/success case when getting the download URL."""
    response = {
        'Success':
            '1',
        'IDS': [{
            'downloadInfo': {
                'DownloadURL': 'foo.com/download',
                'Version': '123.45',
            },
        },],
    }
    self._setJson(response)
    pcnd.cmd_get_url()

  @mock.patch.dict(os.environ, {}, clear=True)
  def test_no_3pp_version(self):
    """Tests behavior when no _3PP_VERSION is set."""
    response = {
        'Success':
            '1',
        'IDS': [{
            'downloadInfo': {
                'DownloadURL': 'foo.com/download',
                'Version': '123.45',
            },
        },],
    }
    self._setJson(response)
    with self.assertRaisesRegex(
        RuntimeError, 'get_url command requires _3PP_VERSION to be set'):
      pcnd.cmd_get_url()

  @mock.patch.dict(os.environ, {'_3PP_VERSION': '123.45'}, clear=True)
  def test_no_version_field(self):
    """Tests behavior when no Version field is present."""
    response = {
        'Success': '1',
        'IDS': [{
            'downloadInfo': {
                'DownloadURL': 'foo.com/download',
            },
        },],
    }
    self._setJson(response)
    with self.assertRaisesRegex(
        RuntimeError,
        'Requested driver version 123.45 did not match available version None'):
      pcnd.cmd_get_url()

  @mock.patch.dict(os.environ, {'_3PP_VERSION': '123.45'}, clear=True)
  def test_mismatched_version(self):
    """Tests behavior when the available version != requested version"""
    response = {
        'Success':
            '1',
        'IDS': [{
            'downloadInfo': {
                'DownloadURL': 'foo.com/download',
                'Version': '234.56',
            },
        },],
    }
    self._setJson(response)
    with self.assertRaisesRegex(
        RuntimeError,
        'Requested driver version 123.45 did not match available version '
        '234.56'):
      pcnd.cmd_get_url()

  @mock.patch.dict(os.environ, {'_3PP_VERSION': '123.45'}, clear=True)
  def test_no_download_url_field(self):
    """Tests behavior when no DownloadURL field is present."""
    response = {
        'Success': '1',
        'IDS': [{
            'downloadInfo': {
                'Version': '123.45',
            },
        },],
    }
    self._setJson(response)
    with self.assertRaisesRegex(
        pcnd.MalformedJsonError,
        'Malformed JSON response without DownloadURL field\\. Raw JSON: .*'):
      pcnd.cmd_get_url()


if __name__ == '__main__':
  unittest.main()
