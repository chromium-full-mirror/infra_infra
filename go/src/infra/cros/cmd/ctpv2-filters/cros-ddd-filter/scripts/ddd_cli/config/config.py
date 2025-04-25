# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

from configparser import ConfigParser


class Config:

  def __init__(self, file_path="config.ini"):
    self._config = ConfigParser()
    self._config.read(file_path)
    self._ttcp_section = "ttcp"

  @property
  def category_list(self):
    return self._config.get(
        section=self._ttcp_section, option="category_list",
        fallback=None).split(",")

  @property
  def ddd_dir(self):
    return self._config.get(
        section=self._ttcp_section, option="ttcp_dir", fallback=None)
