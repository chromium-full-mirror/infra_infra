#!/usr/bin/env python3
# Copyright 2023 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import argparse
import json

apr_version = "1.7.2"
apr_iconv_version = "1.2.2"
apr_util_version = "1.5.4"
gawk_version = "3.1.6-1"
httpd_version = "2.4.55"
openssl_version = "1.1.1j"
pcre_version = "8.45"
php_version = "8.2.5"
zlib_version = "1.2.13"


def do_latest():
  print('httpd{0}-php{1}'.format(httpd_version, php_version))


def get_download_url():
  urls = [
      "https://archive.apache.org/dist/apr/"
      "apr-{}.tar.gz".format(apr_version),
      "https://archive.apache.org/dist/apr/"
      "apr-util-{}.tar.gz".format(apr_util_version),
      "https://archive.apache.org/dist/apr/"
      "apr-iconv-{}.tar.gz".format(apr_iconv_version),
      "https://archive.apache.org/dist/httpd/"
      "httpd-{}.tar.gz".format(httpd_version),
      "https://www.openssl.org/source/"
      "openssl-{}.tar.gz".format(openssl_version),
      "https://sourceforge.net/projects/pcre/files/pcre/"
      "{0}/pcre-{0}.tar.gz/download".format(pcre_version),
      "https://secure.php.net/distributions/"
      "php-{}.tar.gz".format(php_version),
      "https://sourceforge.net/projects/gnuwin32/files/gawk/{0}/gawk-{0}-bin.zip/download"
      .format(gawk_version),
      "https://github.com/madler/zlib/archive/refs/tags/v{}.tar.gz".format(
          zlib_version),
  ]

  packages = [
      "apr-{}.tar.gz".format(apr_version),
      "apr-util-{}.tar.gz".format(apr_util_version),
      "apr-iconv-{}.tar.gz".format(apr_iconv_version),
      "httpd-{}.tar.gz".format(httpd_version),
      "openssl-{}.tar.gz".format(openssl_version),
      "pcre-{}.tar.gz".format(pcre_version),
      "php-{}.tar.gz".format(php_version),
      "gawk-{}-bin.zip".format(gawk_version),
      "zlib-{}.tar.gz".format(zlib_version),
  ]

  partial_manifest = {
      'url': urls,
      'name': packages,
      'ext': '.tar.gz',
  }
  print(json.dumps(partial_manifest))


def main():
  ap = argparse.ArgumentParser()
  sub = ap.add_subparsers(dest='action', required=True)

  latest = sub.add_parser("latest")
  latest.set_defaults(func=lambda _opts: do_latest())

  download = sub.add_parser("get_url")
  download.set_defaults(func=lambda _opts: get_download_url())

  opts = ap.parse_args()
  opts.func(opts)


if __name__ == '__main__':
  main()