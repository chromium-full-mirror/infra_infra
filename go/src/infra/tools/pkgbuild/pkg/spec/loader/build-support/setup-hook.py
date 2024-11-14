# Copyright 2022 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Setup script for adapting 3pp spec."""
# pylint: disable=global-at-module-level
# pylint: disable=undefined-variable


#############################################################################
# Phases
#############################################################################

global configure_phase
global build_phase
global install_phase
global install_verify_phase


def configure_phase(_) -> None:
  return


def build_phase(exe) -> None:
  """Run build command in the source directory."""
  import json
  import os
  import pathlib
  import shutil

  # Copy source to output if no install section presented in the spec.
  if exe.env.get('_3PP_NO_INSTALL'):
    shutil.copytree(os.getcwd(), exe.env['out'],
                    symlinks=True, dirs_exist_ok=True)
    return

  args = json.loads(exe.env['fromSpecInstall'])

  script = pathlib.Path(exe.env['_3PP_DEF'], args[0])
  args[0] = str(script)
  args.insert(0, 'python3' if script.suffix == '.py' else 'bash')

  args.append(exe.env['out'])
  args.append(exe.env['_3PP_PREFIX'])

  exe.execute_cmd(args)


def install_phase(_) -> None:
  return


def install_verify_phase(exe) -> None:
  import json
  import pathlib
  import shutil

  if test_raw := exe.env['fromSpecTest']:
    args = json.loads(test_raw)
  else:
    return

  script = pathlib.Path(exe.env['_3PP_DEF'], args[0])
  args[0] = str(script)
  args.insert(0, 'python3' if script.suffix == '.py' else 'bash')

  out_file = str(pathlib.Path('_pkg.cipd').absolute())
  exe.execute_cmd(['cipd', 'pkg-build',
      '-in', exe.env['out'],
      '-out', out_file,
      '-name', 'install_verify_pkg',
  ])
  args.append(out_file)

  exe.execute_cmd(args)
  return

#############################################################################
# Hooks
#############################################################################

def setup(exe):
  """Copy all libraries into a single directory."""
  import itertools
  import json
  import os
  import pathlib
  import shutil
  import stat

  d = os.path.join(os.getcwd(), '3pp_prefix')
  exe.env['_3PP_PREFIX'] = d

  # Workaround for packages putting executable files under the root directory.
  # e.g. ninja, pip_bootstrap
  def activate_pkg(exe) -> bool:
    ctx = exe.current_context
    if ctx.host == PlatType.BUILD and not ctx.pkg.joinpath('bin').is_dir():
      exe.append_to_search_path(Execution.ENV_PATH, ctx.pkg)
    return True

  # Copy the source before unpack if _3PP_UNPACK_ARCHIVE is not set.
  def skip_unpack(exe):
    if exe.env.get('_3PP_UNPACK_ARCHIVE'):
      return False

    # Do nothing if src is relative to working directory.
    if not os.path.isabs(exe.current_context.src):
      return True

    # ...otherwise copy src from other derivation.
    return extract.copy_cmd(exe)

  # We don't actually flatten the directories. We chdir into the single
  # directories instead to Simulate the behaviour of
  # api.file.flatten_single_directories recipe.
  def set_source_root(exe) -> bool:
    root = os.getcwd()
    if exe.env.get('_3PP_NO_ARCHIVE_PRUNE'):
      exe.env[exe.ENV_SOURCE_ROOT] = root
      return True

    # Move the root if it only contains a single directory.
    while len(names := os.listdir(root)) == 1 and os.path.isdir(names[0]):
      root = names[0]

    exe.env[exe.ENV_SOURCE_ROOT] = root
    return True

  def pre_unpack(exe) -> bool:
    if exe.env.get('_3PP_FETCH_CHECKOUT_WORKFLOW'):
      checkout = pathlib.Path('checkout').absolute()
      vpython = pathlib.Path('vpython-root').absolute()

      checkout.mkdir()

      args = json.loads(exe.env['fromSpecFetch'])
      script = pathlib.Path(exe.env['_3PP_DEF'], args[0])
      args[0] = str(script)
      if script.suffix == '.py':
        args = ['vpython3', '-vpython-root', str(vpython)] + args
      else:
        args = ['bash'] + args
      args.append('checkout')
      args.append(str(checkout))

      exe.execute_cmd(args)

      exe.env[exe.ENV_SOURCES] = os.path.pathsep.join(os.listdir(checkout))
      os.chdir(checkout) # Move to checkout dir and use it as working directory.
    return True

  def post_unpack(exe) -> bool:
    def pkgs(name: str) -> List[str]:
      if e := exe.env.get(name):
        return e.split(os.path.pathsep)
      return []

    prefix_dir = exe.env['_3PP_PREFIX']
    for pkg in itertools.chain(pkgs('depsHostHost'), pkgs('depsHostTarget')):
      shutil.copytree(pkg, prefix_dir, symlinks=True, dirs_exist_ok=True)

      # Reset all permission to write. This is a Workaround for copying from
      # different packages into same directory since cipd marks everything
      # readonly.
      os.chmod(prefix_dir, mode=os.stat(prefix_dir).st_mode | stat.S_IWUSR)
      for root, dirs, files in os.walk(prefix_dir):
        for name in itertools.chain(dirs, files):
          path = os.path.join(root, name)
          os.chmod(
              path,
              mode=os.stat(path).st_mode | stat.S_IWUSR,
              follow_symlinks=True,
          )

    return True

  exe.add_hook('activatePkg', activate_pkg)
  exe.add_hook('preUnpack', pre_unpack)
  exe.add_hook('unpackCmd', skip_unpack)
  exe.add_hook('setSourceRoot', set_source_root)
  exe.add_hook('postUnpack', post_unpack)

setup(exe)
