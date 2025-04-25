# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.
"""Module for the Command class responsible for build and execution actions."""

import subprocess


class Command:

  def __init__(
      self,
      command: str,
      sub_command: str = None,
      command_path: str = None,
      options_map: dict = None,
      critical_errors: tuple = (),
  ):
    self._command = command
    self._sub_command = sub_command
    self._command_path = "./" if command_path is None else command_path
    self._options_map = {} if options_map is None else options_map
    self._critical_errors = critical_errors

  def __str__(self):
    return " ".join(self.build())

  def build(self) -> list:
    """Command builder

        Returns:
          List representation of the built cli command
        """
    options_tup_list = [
        ((f"--{option_name}", str(value)) if not isinstance(value, bool) else
         (f"--{option_name}",))
        for option_name, value in self._options_map.items()
        if (isinstance(value, bool) and value) or (not isinstance(value, bool))
    ]
    options_list = [
        option_item for options_tup in options_tup_list
        for option_item in options_tup if option_item is not None
    ]

    return ([f"{self._command_path}{self._command}"] +
            ([self._sub_command] if self._sub_command is not None else []) +
            options_list)

  def execute(self) -> tuple:
    """Executes the ttcp ddd_cli command

        Returns:
          Output tuple of subprocess command
        """
    process = subprocess.Popen(
        self.__str__(),
        shell=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    print(f"Executing command:\n{self.__str__()}")
    stdout, stderr = process.communicate()
    return process.returncode, stdout, stderr
