# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import argparse
import logging
import sys

from google.api_core import exceptions
from google.cloud import secretmanager
import google_crc32c

MAX_ATTEMPTS = 3


def main(args):
  parser = argparse.ArgumentParser()
  parser.add_argument('--project', required=True, help='Project name on GCP')
  parser.add_argument(
      '--secret', required=True, help='Secret name in Secret Manager')
  parser.add_argument(
      '--output', required=True, help='The file path to store the secret')

  opts = parser.parse_args(args)
  client = secretmanager.SecretManagerServiceClient()
  name = f'projects/{opts.project}/secrets/{opts.secret}/versions/latest'

  retry = 0
  response = None
  while retry < MAX_ATTEMPTS:
    try:
      # Access the latest secret version.
      response = client.access_secret_version(
          request={'name': name}, timeout=30.0)
      crc32c = google_crc32c.Checksum()
      crc32c.update(response.payload.data)
      if response.payload.data_crc32c != int(crc32c.hexdigest(), 16):
        logging.error('Secret checksum fail %s', response.payload.data_crc32c)
        return 1
      with open(opts.output, 'w', encoding='UTF-8') as file:
        file.write(response.payload.data.decode('UTF-8'))
      return 0
    except exceptions.RetryError:
      retry += 1
      logging.error(
          'Fetching OTA from Secret Manager has timed out. '
          'Retrying %d', retry)
  # If we come here, we have hit the retry limit. Fail this run.
  logging.error('Failed to fetch the OTA password.')
  return 1


if __name__ == '__main__':
  logging.basicConfig(
      format='[%(asctime)s %(levelname)s] %(message)s', level=logging.INFO)
  main(sys.argv[1:])
