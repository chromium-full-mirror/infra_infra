#!/usr/bin/env vpython3
# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import argparse
import os
import unittest
from unittest import mock

import pull_current_amd_drivers as pcad

import bs4
from pyfakefs import fake_filesystem_unittest


class GetHtmlUnittest(unittest.TestCase):

  def setUp(self):
    self._get_patcher = mock.patch.object(pcad.requests, 'get')
    self._get_mock = self._get_patcher.start()
    self.addCleanup(self._get_patcher.stop)

  def test_success(self):
    """Tests the happy path/success case when getting HTML."""
    self._get_mock.return_value = mock.Mock(status_code=200, text='text')
    self.assertEqual(pcad._get_html('foo.com'), 'text')

  def test_non_ok_status_code(self):
    """Tests behavior when a non-OK status code is reported."""
    self._get_mock.return_value = mock.Mock(status_code=404, text='error')
    with self.assertRaisesRegex(
        RuntimeError,
        'Got non-OK response code 404 getting foo.com. Response text: error'):
      pcad._get_html('foo.com')


class GetDownloadButtonUnittest(unittest.TestCase):

  def test_success(self):
    """Tests the happy path/success case when getting the download button."""
    html = """\
<body>
<div data-cmp-hooks-accordion="item" class="accordion-item">
  <h3 class="accordion-header">
    <button class="accordion-button collapsed">
      Windows 11 - 64-Bit Edition
    </button>
  </h3>
  <div class="container">
    <div>
      <h4>Auto-Detect and Install</h4>
    </div>
    <div>
      <strong>Revision Number</strong>
    </div>
    <div>
      <a href="foo.com/adrenalin-minimalsetup.exe">
        <span>Download</span>
      </a>
    </div>
  </div>
  <div class="container">
    <div>
      <h4>AMD Software: Adrenalin Edition</h4>
    </div>
    <div>
      <strong>Revision Number</strong>
      <p>Adrenalin 1.2.3 (WHQL Recommended)</p>
    </div>
    <div>
      <a href="foo.com/win10-win11-adrenalin-123.exe">
        <span>Download</span>
      </a>
    </div>
  </div>
</div>
<div data-cmp-hooks-accordion="item" class="accordion-item">
  <h3 class="accordion-header">
    <button class="accordion-button collapsed">
      Windows 10 - 64-Bit Edition
    </button>
  </h3>
  <div class="container">
    <div>
      <h4>Auto-Detect and Install</h4>
    </div>
    <div>
      <strong>Revision Number</strong>
    </div>
    <div>
      <a href="foo.com/adrenalin-minimalsetup.exe">
        <span>Download</span>
      </a>
    </div>
  </div>
  <div class="container">
    <div>
      <h4>AMD Software: Adrenalin Edition</h4>
    </div>
    <div>
      <strong>Revision Number</strong>
      <p>Adrenalin 2.3.4 (WHQL Recommended)</p>
    </div>
    <div>
      <a href="foo.com/win10-win11-adrenalin-234.exe">
        <span>Download</span>
      </a>
    </div>
  </div>
</div>
<div data-cmp-hooks-accordion="item" class="accordion-item">
  <h3 class="accordion-header">
    <button class="accordion-button collapsed">
      Ubuntu x86 64-Bit
    </button>
  </h3>
  <div class="container">
    <div>
      <h4>Radeon Software for Linux</h4>
    </div>
    <div>
      <strong>Revision Number</strong>
      <p>23.40.2</p>
    </div>
    <div>
      <a href="foo.com/amdgpu-install.deb">
        <span>Download</span>
      </a>
    </div>
  </div>
</div>
</body>
"""
    soup = bs4.BeautifulSoup(html, 'html.parser')
    button = pcad._get_download_button(soup, 'url')
    button_only_soup = bs4.BeautifulSoup(
        '<a href="foo.com/win10-win11-adrenalin-234.exe">Download</a>',
        'html.parser')
    self.assertEqual(button['href'], button_only_soup.a['href'])
    self.assertEqual(button['href'], 'foo.com/win10-win11-adrenalin-234.exe')

  def test_no_windows_10_group(self):
    """Test behavior when no Windows 10 group is present."""
    # test_success' HTML but with the Windows 10 group deleted.
    html = """\
<body>
<div data-cmp-hooks-accordion="item" class="accordion-item">
  <h3 class="accordion-header">
    <button class="accordion-button collapsed">
      Windows 11 - 64-Bit Edition
    </button>
  </h3>
  <div class="container">
    <div>
      <h4>Auto-Detect and Install</h4>
    </div>
    <div>
      <strong>Revision Number</strong>
    </div>
    <div>
      <a href="foo.com/adrenalin-minimalsetup.exe">
        <span>Download</span>
      </a>
    </div>
  </div>
  <div class="container">
    <div>
      <h4>AMD Software: Adrenalin Edition</h4>
    </div>
    <div>
      <strong>Revision Number</strong>
      <p>Adrenalin 1.2.3 (WHQL Recommended)</p>
    </div>
    <div>
      <a href="foo.com/win10-win11-adrenalin-123.exe">
        <span>Download</span>
      </a>
    </div>
  </div>
</div>
<div data-cmp-hooks-accordion="item" class="accordion-item">
  <h3 class="accordion-header">
    <button class="accordion-button collapsed">
      Ubuntu x86 64-Bit
    </button>
  </h3>
  <div class="container">
    <div>
      <h4>Radeon Software for Linux</h4>
    </div>
    <div>
      <strong>Revision Number</strong>
      <p>23.40.2</p>
    </div>
    <div>
      <a href="foo.com/amdgpu-install.deb">
        <span>Download</span>
      </a>
    </div>
  </div>
</div>
</body>
"""
    soup = bs4.BeautifulSoup(html, 'html.parser')
    with self.assertRaisesRegex(RuntimeError,
                                'Could not find Windows 10 group on url'):
      pcad._get_download_button(soup, 'url')

  def test_no_relevant_button(self):
    """Tests behavior when no relevant download button is found."""
    # test_success' HTML but with the Windows 10 download link malformed.
    html = """\
<body>
<div data-cmp-hooks-accordion="item" class="accordion-item">
  <h3 class="accordion-header">
    <button class="accordion-button collapsed">
      Windows 11 - 64-Bit Edition
    </button>
  </h3>
  <div class="container">
    <div>
      <h4>Auto-Detect and Install</h4>
    </div>
    <div>
      <strong>Revision Number</strong>
    </div>
    <div>
      <a href="foo.com/adrenalin-minimalsetup.exe">
        <span>Download</span>
      </a>
    </div>
  </div>
  <div class="container">
    <div>
      <h4>AMD Software: Adrenalin Edition</h4>
    </div>
    <div>
      <strong>Revision Number</strong>
      <p>Adrenalin 1.2.3 (WHQL Recommended)</p>
    </div>
    <div>
      <a href="foo.com/win10-win11-adrenalin-123.exe">
        <span>Download</span>
      </a>
    </div>
  </div>
</div>
<div data-cmp-hooks-accordion="item" class="accordion-item">
  <h3 class="accordion-header">
    <button class="accordion-button collapsed">
      Windows 10 - 64-Bit Edition
    </button>
  </h3>
  <div class="container">
    <div>
      <h4>Auto-Detect and Install</h4>
    </div>
    <div>
      <strong>Revision Number</strong>
    </div>
    <div>
      <a href="foo.com/adrenalin-minimalsetup.exe">
        <span>Download</span>
      </a>
    </div>
  </div>
  <div class="container">
    <div>
      <h4>AMD Software: Adrenalin Edition</h4>
    </div>
    <div>
      <strong>Revision Number</strong>
      <p>Adrenalin 2.3.4 (WHQL Recommended)</p>
    </div>
    <div>
      <a href="foo.com/win-adrenalin-234.exe">
        <span>Download</span>
      </a>
    </div>
  </div>
</div>
<div data-cmp-hooks-accordion="item" class="accordion-item">
  <h3 class="accordion-header">
    <button class="accordion-button collapsed">
      Ubuntu x86 64-Bit
    </button>
  </h3>
  <div class="container">
    <div>
      <h4>Radeon Software for Linux</h4>
    </div>
    <div>
      <strong>Revision Number</strong>
      <p>23.40.2</p>
    </div>
    <div>
      <a href="foo.com/amdgpu-install.deb">
        <span>Download</span>
      </a>
    </div>
  </div>
</div>
</body>
"""
    soup = bs4.BeautifulSoup(html, 'html.parser')
    with self.assertRaisesRegex(
        RuntimeError, 'Unable to find any relevant download buttons on url'):
      pcad._get_download_button(soup, 'url')

  def test_too_many_buttons(self):
    """Tests behavior when multiple relevant download buttons are found."""
    # test_success' HTML but with an additional Windows 10 download link added.
    html = """\
<body>
<div data-cmp-hooks-accordion="item" class="accordion-item">
  <h3 class="accordion-header">
    <button class="accordion-button collapsed">
      Windows 11 - 64-Bit Edition
    </button>
  </h3>
  <div class="container">
    <div>
      <h4>Auto-Detect and Install</h4>
    </div>
    <div>
      <strong>Revision Number</strong>
    </div>
    <div>
      <a href="foo.com/adrenalin-minimalsetup.exe">
        <span>Download</span>
      </a>
    </div>
  </div>
  <div class="container">
    <div>
      <h4>AMD Software: Adrenalin Edition</h4>
    </div>
    <div>
      <strong>Revision Number</strong>
      <p>Adrenalin 1.2.3 (WHQL Recommended)</p>
    </div>
    <div>
      <a href="foo.com/win10-win11-adrenalin-123.exe">
        <span>Download</span>
      </a>
    </div>
  </div>
</div>
<div data-cmp-hooks-accordion="item" class="accordion-item">
  <h3 class="accordion-header">
    <button class="accordion-button collapsed">
      Windows 10 - 64-Bit Edition
    </button>
  </h3>
  <div class="container">
    <div>
      <h4>Auto-Detect and Install</h4>
    </div>
    <div>
      <strong>Revision Number</strong>
    </div>
    <div>
      <a href="foo.com/adrenalin-minimalsetup.exe">
        <span>Download</span>
      </a>
    </div>
  </div>
  <div class="container">
    <div>
      <h4>AMD Software: Adrenalin Edition</h4>
    </div>
    <div>
      <strong>Revision Number</strong>
      <p>Adrenalin 2.3.4 (WHQL Recommended)</p>
    </div>
    <div>
      <a href="foo.com/win10-win11-adrenalin-234.exe">
        <span>Download</span>
      </a>
    </div>
    <div>
      <strong>Revision Number</strong>
      <p>Adrenalin 3.4.5 (WHQL Recommended)</p>
    </div>
    <div>
      <a href="foo.com/win10-win11-adrenalin-345.exe">
        <span>Download</span>
      </a>
    </div>
  </div>
</div>
<div data-cmp-hooks-accordion="item" class="accordion-item">
  <h3 class="accordion-header">
    <button class="accordion-button collapsed">
      Ubuntu x86 64-Bit
    </button>
  </h3>
  <div class="container">
    <div>
      <h4>Radeon Software for Linux</h4>
    </div>
    <div>
      <strong>Revision Number</strong>
      <p>23.40.2</p>
    </div>
    <div>
      <a href="foo.com/amdgpu-install.deb">
        <span>Download</span>
      </a>
    </div>
  </div>
</div>
</body>
"""
    soup = bs4.BeautifulSoup(html, 'html.parser')
    with self.assertRaisesRegex(RuntimeError,
                                ('Found multiple relevant download buttons on '
                                 'url: foo.com/win10-win11-adrenalin-234.exe '
                                 'foo.com/win10-win11-adrenalin-345.exe')):
      pcad._get_download_button(soup, 'url')


class GetDriverVersionUnittest(unittest.TestCase):

  def test_success(self):
    """Tests the happy path/success case when getting the driver version."""
    html = """\
<body>
<div data-cmp-hooks-accordion="item" class="accordion-item">
  <h3 class="accordion-header">
    <button class="accordion-button collapsed">
      Windows 11 - 64-Bit Edition
    </button>
  </h3>
  <div class="container">
    <div>
      <h4>Auto-Detect and Install</h4>
    </div>
    <div>
      <strong>Revision Number</strong>
    </div>
    <div>
      <a href="foo.com/adrenalin-minimalsetup.exe">
        <span>Download</span>
      </a>
    </div>
  </div>
  <div class="container">
    <div>
      <h4>AMD Software: Adrenalin Edition</h4>
    </div>
    <div>
      <strong>Revision Number</strong>
      <p>Adrenalin 1.2.3 (WHQL Recommended)</p>
    </div>
    <div>
      <a href="foo.com/win10-win11-adrenalin-123.exe">
        <span>Download</span>
      </a>
    </div>
  </div>
</div>
<div data-cmp-hooks-accordion="item" class="accordion-item">
  <h3 class="accordion-header">
    <button class="accordion-button collapsed">
      Windows 10 - 64-Bit Edition
    </button>
  </h3>
  <div class="container">
    <div>
      <h4>Auto-Detect and Install</h4>
    </div>
    <div>
      <strong>Revision Number</strong>
    </div>
    <div>
      <a href="foo.com/adrenalin-minimalsetup.exe">
        <span>Download</span>
      </a>
    </div>
  </div>
  <div class="container">
    <div>
      <h4>AMD Software: Adrenalin Edition</h4>
    </div>
    <div>
      <strong>Revision Number</strong>
      <p>Adrenalin 2.3.4 (WHQL Recommended)</p>
    </div>
    <div>
      <a href="foo.com/win10-win11-adrenalin-234.exe">
        <span>Download</span>
      </a>
    </div>
  </div>
</div>
</body>
"""
    soup = bs4.BeautifulSoup(html, 'html.parser')
    self.assertEqual(pcad._get_driver_version(soup, 'url'), '2.3.4')

  def test_missing_revision_label(self):
    """Tests behavior when no valid revision label is found."""
    # test_success' HTML but with the revision label malformed and Windows 11
    # download removed.
    html = """\
<body>
<div data-cmp-hooks-accordion="item" class="accordion-item">
  <h3 class="accordion-header">
    <button class="accordion-button collapsed">
      Windows 10 - 64-Bit Edition
    </button>
  </h3>
  <div class="container">
    <div>
      <h4>Auto-Detect and Install</h4>
    </div>
    <div>
      <strong>Revision Number</strong>
    </div>
    <div>
      <a href="foo.com/adrenalin-minimalsetup.exe">
        <span>Download</span>
      </a>
    </div>
  </div>
  <div class="container">
    <div>
      <h4>AMD Software: Adrenalin Edition</h4>
    </div>
    <div>
      <strong>Version Number</strong>
      <p>Adrenalin 2.3.4 (WHQL Recommended)</p>
    </div>
    <div>
      <a href="foo.com/win10-win11-adrenalin-234.exe">
        <span>Download</span>
      </a>
    </div>
  </div>
</div>
</body>
"""
    soup = bs4.BeautifulSoup(html, 'html.parser')
    with self.assertRaisesRegex(RuntimeError,
                                'Unable to find revision label from url'):
      pcad._get_driver_version(soup, 'url')

  def test_missing_revision(self):
    """Tests behavior when no valid revision is found."""
    # test_success' HTML but with the revision number malformed and Windows 11
    # download removed.
    html = """\
<body>
<div data-cmp-hooks-accordion="item" class="accordion-item">
  <h3 class="accordion-header">
    <button class="accordion-button collapsed">
      Windows 10 - 64-Bit Edition
    </button>
  </h3>
  <div class="container">
    <div>
      <h4>Auto-Detect and Install</h4>
    </div>
    <div>
      <strong>Revision Number</strong>
    </div>
    <div>
      <a href="foo.com/adrenalin-minimalsetup.exe">
        <span>Download</span>
      </a>
    </div>
  </div>
  <div class="container">
    <div>
      <h4>AMD Software: Adrenalin Edition</h4>
    </div>
    <div>
      <strong>Revision Number</strong>
      <p>Adrenalin 2.3.4.5 (WHQL Recommended)</p>
    </div>
    <div>
      <a href="foo.com/win10-win11-adrenalin-234.exe">
        <span>Download</span>
      </a>
    </div>
  </div>
</div>
</body>
"""
    soup = bs4.BeautifulSoup(html, 'html.parser')
    with self.assertRaisesRegex(RuntimeError,
                                'Unable to extract driver version from url'):
      pcad._get_driver_version(soup, 'url')


class GetDriverBinaryUnittest(unittest.TestCase):

  def setUp(self):
    self._get_patcher = mock.patch.object(pcad.requests, 'get')
    self._get_mock = self._get_patcher.start()
    self.addCleanup(self._get_patcher.stop)

    self.html = """\
<body>
<div data-cmp-hooks-accordion="item" class="accordion-item">
  <h3 class="accordion-header">
    <button class="accordion-button collapsed">
      Windows 11 - 64-Bit Edition
    </button>
  </h3>
  <div class="container">
    <div>
      <h4>Auto-Detect and Install</h4>
    </div>
    <div>
      <strong>Revision Number</strong>
    </div>
    <div>
      <a href="foo.com/adrenalin-minimalsetup.exe">
        <span>Download</span>
      </a>
    </div>
  </div>
  <div class="container">
    <div>
      <h4>AMD Software: Adrenalin Edition</h4>
    </div>
    <div>
      <strong>Revision Number</strong>
      <p>Adrenalin 1.2.3 (WHQL Recommended)</p>
    </div>
    <div>
      <a href="foo.com/win10-win11-adrenalin-123.exe">
        <span>Download</span>
      </a>
    </div>
  </div>
</div>
<div data-cmp-hooks-accordion="item" class="accordion-item">
  <h3 class="accordion-header">
    <button class="accordion-button collapsed">
      Windows 10 - 64-Bit Edition
    </button>
  </h3>
  <div class="container">
    <div>
      <h4>Auto-Detect and Install</h4>
    </div>
    <div>
      <strong>Revision Number</strong>
    </div>
    <div>
      <a href="foo.com/adrenalin-minimalsetup.exe">
        <span>Download</span>
      </a>
    </div>
  </div>
  <div class="container">
    <div>
      <h4>AMD Software: Adrenalin Edition</h4>
    </div>
    <div>
      <strong>Revision Number</strong>
      <p>Adrenalin 2.3.4 (WHQL Recommended)</p>
    </div>
    <div>
      <a href="foo.com/win10-win11-adrenalin-234.exe">
        <span>Download</span>
      </a>
    </div>
  </div>
</div>
</body>
"""

  def test_success(self):
    """Tests the happy path/success case when downloading the driver."""
    soup = bs4.BeautifulSoup(self.html, 'html.parser')
    byte_string = ('a' * 110 * 1024 * 1024).encode('utf-8')
    self._get_mock.return_value = mock.Mock(
        status_code=200, content=byte_string)
    self.assertEqual(pcad._get_driver_binary(soup, 'url'), byte_string)

  def test_non_ok_status_code(self):
    """Tests behavior when a non-OK status code is received."""
    soup = bs4.BeautifulSoup(self.html, 'html.parser')
    self._get_mock.return_value = mock.Mock(status_code=404)
    with self.assertRaisesRegex(
        RuntimeError, ('Got non-OK status code 404 downloading driver '
                       'from foo.com/win10-win11-adrenalin-234.exe')):
      pcad._get_driver_binary(soup, 'url')

  def test_not_enough_bytes(self):
    """Tests behavior when we get back fewer bytes than we reasonably expect."""
    soup = bs4.BeautifulSoup(self.html, 'html.parser')
    byte_string = ('a' * 10).encode('utf-8')
    self._get_mock.return_value = mock.Mock(
        status_code=200, content=byte_string)
    with self.assertRaisesRegex(
        RuntimeError, ('Only got back %d bytes from driver download, '
                       'which is much less than expected' % len(byte_string))):
      pcad._get_driver_binary(soup, 'url')


class CmdGetCheckoutUnittest(fake_filesystem_unittest.TestCase):

  def setUp(self):
    self.setUpPyfakefs()
    self._soup_patcher = mock.patch.object(pcad, '_get_soup')
    self._soup_mock = self._soup_patcher.start()
    self.addCleanup(self._soup_patcher.stop)

    html = """\
<body>
<div data-cmp-hooks-accordion="item" class="accordion-item">
  <h3 class="accordion-header">
    <button class="accordion-button collapsed">
      Windows 11 - 64-Bit Edition
    </button>
  </h3>
  <div class="container">
    <div>
      <h4>Auto-Detect and Install</h4>
    </div>
    <div>
      <strong>Revision Number</strong>
    </div>
    <div>
      <a href="foo.com/adrenalin-minimalsetup.exe">
        <span>Download</span>
      </a>
    </div>
  </div>
  <div class="container">
    <div>
      <h4>AMD Software: Adrenalin Edition</h4>
    </div>
    <div>
      <strong>Revision Number</strong>
      <p>Adrenalin 1.2.3 (WHQL Recommended)</p>
    </div>
    <div>
      <a href="foo.com/win10-win11-adrenalin-123.exe">
        <span>Download</span>
      </a>
    </div>
  </div>
</div>
<div data-cmp-hooks-accordion="item" class="accordion-item">
  <h3 class="accordion-header">
    <button class="accordion-button collapsed">
      Windows 10 - 64-Bit Edition
    </button>
  </h3>
  <div class="container">
    <div>
      <h4>Auto-Detect and Install</h4>
    </div>
    <div>
      <strong>Revision Number</strong>
    </div>
    <div>
      <a href="foo.com/adrenalin-minimalsetup.exe">
        <span>Download</span>
      </a>
    </div>
  </div>
  <div class="container">
    <div>
      <h4>AMD Software: Adrenalin Edition</h4>
    </div>
    <div>
      <strong>Revision Number</strong>
      <p>Adrenalin 2.3.4 (WHQL Recommended)</p>
    </div>
    <div>
      <a href="foo.com/win10-win11-adrenalin-234.exe">
        <span>Download</span>
      </a>
    </div>
  </div>
</div>
<div data-cmp-hooks-accordion="item" class="accordion-item">
  <h3 class="accordion-header">
    <button class="accordion-button collapsed">
      Ubuntu x86 64-Bit
    </button>
  </h3>
  <div class="container">
    <div>
      <h4>Radeon Software for Linux</h4>
    </div>
    <div>
      <strong>Revision Number</strong>
      <p>23.40.2</p>
    </div>
    <div>
      <a href="foo.com/amdgpu-install.deb">
        <span>Download</span>
      </a>
    </div>
  </div>
</div>
</body>
"""
    self._soup_mock.return_value = bs4.BeautifulSoup(html, 'html.parser')

    self._get_patcher = mock.patch.object(pcad.requests, 'get')
    self._get_mock = self._get_patcher.start()
    self.addCleanup(self._get_patcher.stop)

    self.checkout_path = os.path.abspath(os.path.join('tmp', 'amd_drivers'))
    os.makedirs(self.checkout_path)
    parser = argparse.ArgumentParser()
    parser.add_argument('checkout_path')
    self.args = parser.parse_args([self.checkout_path])

  def test_success(self):
    """Tests the happy path/success case when running the get_checkout cmd."""
    byte_string = ('a' * 110 * 1024 * 1024).encode('utf-8')
    self._get_mock.return_value = mock.Mock(
        status_code=200, content=byte_string)
    with mock.patch.dict(os.environ, {'_3PP_VERSION': '2.3.4'}, clear=True):
      pcad.cmd_checkout(self.args)
    with open(os.path.join(self.checkout_path, 'raw_source_0.exe'),
              'rb') as infile:
      self.assertEqual(infile.read(), byte_string)

  def test_no_requested_driver_version(self):
    """Tests behavior when no requested driver version is set."""
    with mock.patch.dict(os.environ, {}, clear=True):
      with self.assertRaisesRegex(
          RuntimeError, 'checkout command requires _3PP_VERSION to be set'):
        pcad.cmd_checkout(self.args)

  def test_mismatched_driver_version(self):
    """Tests behavior when the requested and actual driver versions differ."""
    with mock.patch.dict(os.environ, {'_3PP_VERSION': '1.2.3'}, clear=True):
      with self.assertRaisesRegex(RuntimeError,
                                  ('Requested driver version 1.2.3 did not '
                                   'match available version 2.3.4')):
        pcad.cmd_checkout(self.args)


if __name__ == '__main__':
  unittest.main()