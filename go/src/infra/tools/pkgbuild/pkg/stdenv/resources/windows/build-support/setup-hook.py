# Copyright 2022 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Setup script with windows specified hooks."""
# pylint: disable=global-at-module-level
# pylint: disable=undefined-variable


def setup(exe) -> None:
  """Build hooks for windows."""
  import json
  import os
  import pathlib
  import subprocess

  def _resolve_links(path):
    with open(path, 'rb') as f:
      if f.read(10) != b'!<symlink>':
        return path
      src = f.read().decode('utf-16').rstrip('\x00')
    return _resolve_links(src)

  def _lookup(exe, name):
    # Remove '.exe' suffix.
    if name[-4:] == '.exe':
      name = name[:-4]
    for parent in _split(exe.env[Execution.ENV_PATH], os.pathsep):
      if (
          (p := pathlib.Path(parent).joinpath(name)).exists() or
          (p := pathlib.Path(parent).joinpath(name + '.exe')).exists()
      ):
        return p

  def pre_unpack(exe) -> bool:
    sdk = pathlib.Path(exe.env['winsdk_root'])
    with open(next(sdk.glob(f'**/SetEnv.{exe.env["sdk_arch"]}.json'))) as f:
      # SDK cipd packages prior to 10.0.19041.0 contain entries like:
      #  "INCLUDE": [["..","..","win_sdk","Include","10.0.17134.0","um"]
      # are all specified relative to the SetEnv.*.json.
      # For 10.0.19041.0 and later, the cipd SDK package json is like:
      #  "INCLUDE": [["Windows Kits","10","Include","10.0.19041.0","um"].
      for k, vs in json.load(f).get('env', {}).items():
        normalized = []
        for v in vs:
          if v[0] == '..' and (v[1] == '..' or v[1] == '..\\'):
            normalized.append(sdk.joinpath(*v[2:]).absolute())
          else:
            normalized.append(sdk.joinpath(*v).absolute())

        # PATH is special-cased because we don't want to overwrite other things
        # like C:\Windows\System32. Others are replacements because prepending
        # doesn't necessarily makes sense, like VSINSTALLDIR.
        # Iterate reversely to keep the order since we are prepending the path.
        if k == Execution.ENV_PATH:
          for v in reversed(normalized):
            exe.prepend_to_search_path(k, v)
        else:
          exe.env[k] = os.path.pathsep.join(map(str, normalized))

    # CMD.EXE is required by NMAKE but NMAKE doesn't recognize MinGW's symlink.
    # Set COMSPEC instead.
    if cmd := _lookup(exe, 'cmd'):
      exe.env['COMSPEC'] = str(_resolve_links(cmd))

    return True

  def execute_cmd(exe) -> bool:
    ctx = exe.current_context
    args = ctx.args.copy()

    if not os.path.isabs(args[0]) and (p := _lookup(exe, args[0])):
      args[0] = str(p)

    args[0] = _resolve_links(args[0])
    subprocess.check_call(args, env=exe.env)
    return True

  exe.add_hook('preUnpack', pre_unpack)
  exe.add_hook('executeCmd', execute_cmd)

  exe.env['TMP'] = os.getcwd()
  exe.env['TEMP'] = os.getcwd()


setup(exe)
