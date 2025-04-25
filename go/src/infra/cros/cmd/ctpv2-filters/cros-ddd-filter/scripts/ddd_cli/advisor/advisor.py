# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

from ddd_cli.command import Command


class Advisor(Command):

  def __init__(
      self,
      sub_command: str = None,
      command_path: str = None,
      options_map: dict = None,
  ):
    if options_map:
      options_map = {
          option_name: option_value
          for option_name, option_value in options_map.items()
          if option_value is not None
      }
    super(Advisor, self).__init__(
        command="advisor",
        sub_command=sub_command,
        command_path=command_path,
        options_map=options_map,
    )
