# Copyright 2017 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

from typing import Iterable, Optional

from recipe_engine import config_types
from recipe_engine import recipe_api


class CodesearchApi(recipe_api.RecipeApi):
  _PROJECT_BROWSER, _PROJECT_OS, _PROJECT_UNSUPPORTED = range(3)

  def __init__(self, **kwargs):
    super().__init__(**kwargs)
    self._kythe_dir = None

  @property
  def _is_experimental(self) -> bool:
    """Return whether this build is running in experimental mode."""
    # TODO(jsca): Delete the second part of the below condition after LUCI
    # migration is complete.
    return self.c.EXPERIMENTAL or self.m.runtime.is_experimental

  def get_config_defaults(self):
    return {
        'CHECKOUT_PATH': self.m.path.checkout_dir,
    }

  def cleanup_old_generated(
      self,
      age_days: int = 7,
      checkout_dir: Optional[config_types.Path] = None,
  ) -> None:
    """Clean up generated files older than the specified number of days.

    Args:
      age_days: Minimum age in days for files to delete (integer).
      checkout_dir: The directory where code is checked out. If not specified,
        use checkout_dir initialized in path module by default
    """
    checkout_dir = checkout_dir or self.m.path.checkout_dir
    if self.c.PLATFORM.startswith('win'):
      # Flag explanations for the Windows command:
      # /p <path>  -- Search files in the given path
      # /s         -- Search recursively
      # /m *       -- Use the search mask *. This includes files without an
      #               extension (the default is *.*).
      # /d -<age>  -- Find files last modified before <age> days ago
      # /c <cmd>   -- Run <cmd> for each file. In our case we delete the file if
      #               it's not a directory.
      delete_command = [
          'forfiles', '/p', checkout_dir / 'out', '/s', '/m', '*', '/d',
          ('-%d' % age_days), '/c', 'cmd /c if @isdir==FALSE del @path'
      ]
      try:
        self.m.step('delete old generated files', delete_command)
      except self.m.step.StepFailure as f:
        # On Windows, forfiles returns an error code if it found no files. We
        # don't want to fail if this happens, so convert it to a warning.
        self.m.step.active_result.presentation.step_text = f.reason_message()
        self.m.step.active_result.presentation.status = self.m.step.WARNING
    else:
      # Flag explanations for the Linux command:
      # find <path>    -- Find files recursively in the given path
      # -mtime +<age>  -- Find files last modified before <age> days ago
      # -type f        -- Find files only (not directories)
      # -delete        -- Delete the found files
      delete_command = [
          'find', checkout_dir / 'out', '-mtime', ('+%d' % age_days), '-type',
          'f', '-delete'
      ]
      self.m.step('delete old generated files', delete_command)

  def add_kythe_metadata(self):
    """Adds inline Kythe metadata to Mojom generated files.

    This metadata is used to connect things in the generated file to the thing
    in the Mojom file which generated it. This is made possible by annotations
    added to the generated file by the Mojo compiler.
    """
    self.m.step('add kythe metadata', [
        'python3',
        self.resource('add_kythe_metadata.py'),
        '--corpus',
        self.c.CORPUS,
        self.c.out_path,
    ])

  def ensure_kythe(self):
    """Ensures kythe is installed using cipd and returns its root directory.
    """
    if self._kythe_dir is None:
      packages_root = self.m.path.start_dir / 'packages'
      ensure_file = self.m.cipd.EnsureFile().add_package(
          'infra/3pp/tools/kythe', 'version:3@v0.0.71', 'kythe')
      self.m.cipd.ensure(packages_root, ensure_file)
      self._kythe_dir = packages_root / 'kythe'
    return self._kythe_dir

  def run_rust_project_extractor(self, checkout_dir: config_types.Path):
    # In package_index, we generate paths relative to the output root.
    # We'll tell Kythe's Rust extractor that it's the root as well.
    # Kythe's Rust extractor also needs to know how these outroot-relative
    # paths are mapped to vnames like how we do it in package_index.
    # Spec:
    # https://kythe.io/examples/#:~:text=%7D%0A%20%20%20%7D%0A%20%7D-,VName%20configuration,-Next%2C%20you%20will
    vnames_config = [
        {
            # If the output root is src/out/Default
            # and source root is src/
            # Then the expected path of src/foo/bar.rs would be
            # ../../foo/bar.rs
            # And the expected vname would be foo/bar.rs
            'pattern':
                self.m.path.relpath(
                    checkout_dir,
                    self.c.out_path,
                ) + '/(.*)',
            'vname': {
                'path': '@1@'
            },
        },
        {
            # Chromium puts its generated files under out/ and vname for
            # generated files should have root set to separate them from other
            # source files. See b/354949952 for more info.
            'pattern': '([^.].*)',  # crude regex to filter out relative paths
            'vname': {
                'root': 'out',
                'path': '@1@'
            },
        },
    ]
    vnames_config_json = self.m.json.dumps(vnames_config)

    rust_kzip_path = self.m.path.mkstemp()
    self.m.step(
        "extract Rust kzips",
        [
            self.ensure_kythe().joinpath('extractors', 'rustproject_extractor'),
            '--corpus',
            self.c.CORPUS,
            '--output',
            rust_kzip_path,
            '--vnames_json_path',
            self.m.raw_io.input_text(vnames_config_json, suffix='.json'),
            '--root',
            self.c.out_path,
            '--project_json',
            self.c.out_path / 'rust-project.json',
        ],
    )
    return rust_kzip_path

  def run_kzip_merge(self, *kzip_inputs):
    """Merges multiple kzips into a single kzip.
    The kzips are assumed to all be protobuf-encoded.
    Note that the package_index tool already performs specialized merging
    in parallel. This API should only be used where package_index is not
    adequate, as the merge will be performed as a discrete serial step.

    Args:
      kzip_inputs: Paths to kzip files to merge.
    """
    combined_kzip_path = self.m.path.mkstemp()
    self.m.step(
        "merge kzips",
        [
            self.ensure_kythe().joinpath('tools', 'kzip'),
            'merge',
            '--encoding',
            'PROTO',
            '--output',
            combined_kzip_path,
            *kzip_inputs,
        ],
    )
    return combined_kzip_path

  def clone_clang_tools(self, clone_dir):
    """Clone chromium/src clang tools."""
    clang_dir = clone_dir / 'clang'
    with self.m.context(cwd=clone_dir):
      self.m.file.rmtree('remove previous instance of clang tools', clang_dir)
      self.m.git('clone',
                 'https://chromium.googlesource.com/chromium/src/tools/clang')
    return clang_dir

  def run_clang_tool(
      self,
      clang_dir: Optional[config_types.Path] = None,
      run_dirs: Optional[Iterable[config_types.Path]] = None,
      target_architecture: Optional[str] = None,
  ) -> None:
    """Download and run the clang tool.

    Args:
      clang_dir: Path to clone clang into.
      run_dirs: Dirs in which to run the clang tool.
      target_architecture: If given, the architecture to transpile for.
    """
    # TODO(crbug.com/329113288): Don't use self.m.path.checkout_dir
    clang_dir = clang_dir or self.m.path.checkout_dir.joinpath('tools', 'clang')

    # Download the clang tool.
    translation_unit_dir = self.m.path.mkdtemp()
    self.m.step(
        name='download translation_unit clang tool',
        cmd=[
            'python3', '-u',
            clang_dir.joinpath('scripts',
                               'update.py'), '--package=translation_unit',
            '--output-dir=' + str(translation_unit_dir)
        ])

    # Run the clang tool.
    args = [
        '--tool', 'translation_unit', '--tool-path',
        translation_unit_dir / 'bin', '-p', self.c.out_path, '--all'
    ]

    if target_architecture is not None:
      # We want to tell clang the target architecture, but that needs to pass
      # through a few layers of wrapper scripts. It's a little confusing, so
      # let's work backward to understand it.
      #
      # 1.  Ultimately, we want to call `clang` with `--target=${ARCH}` (where
      #     `${ARCH}` stands for our target architecture).
      #
      # 2.  But we aren't calling `clang` directly. `clang` gets invoked by
      #     Chromium's `translation_unit` tool. That tool has a flag,
      #     `--extra-arg`, whose values get forwarded to `clang`. So we need to
      #     call `translation_unit`  with `--extra-arg=--target=${ARCH}`.
      #
      # 3.  But we aren't calling `translation_unit` directly, either. We're
      #     calling it via Chromium's `run_script.py` wrapper. That wrapper has
      #     another flag, `--tool-arg`, whose values get forwarded to
      #     `translation_unit`. Thus, we need to call `run_script.py` with
      #     `--tool-arg=--extra-arg=--target=${ARCH}.`
      #
      # Also, notice that we're sending CLI flags whose values start with
      # dashes, and therefore look like flags themselves. In order for these
      # flags to be parsed correctly, we need to send them as `key=value`, like
      # `--tool-arg=--extra-arg`, not `--tool-arg --extra-arg`.
      #
      # The next few lines aren't the most succinct way to append that flag, but
      # hopefully it's clearer than a one-liner.
      clang_flag = f'--target={target_architecture}'
      translation_unit_flag = f'--extra-arg={clang_flag}'
      run_script_flag = f'--tool-arg={translation_unit_flag}'
      assert (run_script_flag ==
              f'--tool-arg=--extra-arg=--target={target_architecture}')
      args.append(run_script_flag)

    if run_dirs is None:
      run_dirs = [self.m.context.cwd]
    for run_dir in run_dirs:
      try:
        with self.m.context(cwd=run_dir):
          self.m.step(
              'run translation_unit clang tool',
              ['python3', '-u',
               clang_dir.joinpath('scripts', 'run_tool.py')] + args)

      except self.m.step.StepFailure as f:  # pragma: nocover
        # For some files, the clang tool produces errors. This is a known issue,
        # but since it only affects very few files (currently 9), we ignore
        # these errors for now. At least this means we can already have cross
        # reference support for the files where it works.
        # TODO(crbug/1284439): Investigate translation_unit failures for CrOS.
        self.m.step.active_result.presentation.step_text = f.reason_message()
        self.m.step.active_result.presentation.status = self.m.step.WARNING

  def _get_project_type(self):
    """Returns the type of the project.
    """
    if self.c.PROJECT in ('chromium', 'chrome'):
      return self._PROJECT_BROWSER
    if self.c.PROJECT in ('chromiumos', 'chromeos'):
      return self._PROJECT_OS
    return self._PROJECT_UNSUPPORTED  # pragma: nocover

  def create_and_upload_kythe_index_pack(
      self,
      commit_hash: str,
      commit_timestamp: int,
      commit_position: Optional[str] = None,
      clang_target_arch: Optional[str] = None,
      checkout_dir: Optional[config_types.Path] = None,
  ) -> config_types.Path:
    """Create the kythe index pack and upload it to google storage.
    Legacy wrapper around create_kythe_index_pack and upload_kythe_index_pack.

    Args:
      commit_hash: Hash of the commit at which we're creating the index pack,
        if None use got_revision.
      commit_timestamp: Timestamp of the commit at which we're creating the
        index pack, in integer seconds since the UNIX epoch.
      commit_position: The commit position of the project. Required only for
        the browser project.
      clang_target_arch: Target architecture to cross-compile for.
      checkout_dir: The directory where code is checked out. If not specified,
        use checkout_dir initialized in path module by default.

    Returns:
      Path to the generated index pack.
    """
    index_pack_path = self.create_kythe_index_pack(
        clang_target_arch=clang_target_arch, checkout_dir=checkout_dir)
    if not self.m.tryserver.is_tryserver:  # pragma: no cover
      self.upload_kythe_index_pack(
          index_pack_kythe_path=index_pack_path,
          commit_hash=commit_hash,
          commit_timestamp=commit_timestamp,
          commit_position=commit_position,
      )
    return index_pack_path

  def create_kythe_index_pack(
      self,
      clang_target_arch: Optional[str] = None,
      checkout_dir: Optional[config_types.Path] = None,
  ) -> config_types.Path:
    """Create the kythe index pack.

    Args:
      clang_target_arch: Target architecture to cross-compile for.
      checkout_dir: The directory where code is checked out. If not specified,
        use checkout_dir initialized in path module by default.

    Returns:
      Path to the generated index pack.
    """
    index_pack_kythe_base = '%s_%s' % (self.c.PROJECT, self.c.PLATFORM)
    index_pack_kythe_name = '%s.kzip' % index_pack_kythe_base
    index_pack_kythe_path = self.c.out_path / index_pack_kythe_name

    # TODO(crbug.com/329113288): Don't use self.m.path.checkout_dir
    checkout_dir = checkout_dir or self.m.path.checkout_dir
    exec_path = self.m.cipd.ensure_tool("infra/tools/package_index/${platform}",
                                        "latest")
    args = [
        '--checkout_dir',
        checkout_dir,
        '--path_to_compdb',
        self.c.compile_commands_json_file,
        '--path_to_gn_targets',
        self.c.gn_targets_json_file,
        '--path_to_archive_output',
        index_pack_kythe_path,
        '--corpus',
        self.c.CORPUS,
        '--project',
        self.c.PROJECT,
    ]

    if clang_target_arch is not None:
      args.extend(['--clang_target_arch', clang_target_arch])

    if self.c.javac_extractor_output_dir:
      args.extend(['--path_to_java_kzips', self.c.javac_extractor_output_dir])

    # If out_path is /path/to/src/out/foo and checkout_dir is /path/to/src/,
    # then out_dir wants src/out/foo.
    args.extend([
        '--out_dir',
        self.m.path.relpath(
            self.c.out_path,
            self.m.path.dirname(checkout_dir),
        )
    ])

    if self.c.BUILD_CONFIG:
      args.extend(['--build_config', self.c.BUILD_CONFIG])
    self.m.step('create kythe index pack', [exec_path] + args)
    return index_pack_kythe_path

  def upload_kythe_index_pack(
      self,
      index_pack_kythe_path: config_types.Path,
      commit_hash: str,
      commit_timestamp: int,
      commit_position: Optional[str] = None,
  ):
    """Upload the given kythe index pack to google storage.

    Args:
      index_pack_kythe_path: Path to the index pack to upload.
      commit_hash: Hash of the commit at which we're creating the index pack,
        if None use got_revision.
      commit_timestamp: Timestamp of the commit at which we're creating the
        index pack, in integer seconds since the UNIX epoch.
      commit_position: The commit position of the project. Required only for
        the browser project.

    Returns:
      Path to the generated index pack.
    """
    experimental_suffix = '_experimental' if self._is_experimental else ''
    index_pack_kythe_base, _ = self.m.path.splitext(
        self.m.path.basename(index_pack_kythe_path))
    index_pack_kythe_name_with_id = ''
    project_type = self._get_project_type()
    if project_type == self._PROJECT_BROWSER:
      assert commit_position, 'invalid commit_position %s' % commit_position
      index_pack_kythe_name_with_id = '%s_%s_%s+%d%s.kzip' % (
          index_pack_kythe_base, commit_position, commit_hash, commit_timestamp,
          experimental_suffix)
    elif project_type == self._PROJECT_OS:
      index_pack_kythe_name_with_id = '%s_%s+%d%s.kzip' % (
          index_pack_kythe_base, commit_hash, commit_timestamp,
          experimental_suffix)
    else:  # pragma: no cover
      assert False, 'Unsupported codesearch project %s' % self.c.PROJECT

    assert self.c.bucket_name, (
        'Trying to upload Kythe index pack but no google storage bucket name')
    self.m.gsutil.upload(
        name='upload kythe index pack',
        source=index_pack_kythe_path,
        bucket=self.c.bucket_name,
        dest='prod/%s' % index_pack_kythe_name_with_id,
        dry_run=self._is_experimental)

    # Also upload compile_commands and gn_targets for debugging purposes.
    compdb_name_with_revision = 'compile_commands_%s_%s.json' % (
        self.c.PLATFORM, commit_position or commit_hash)
    self._upload_compile_commands_json(self.c.bucket_name,
                                       compdb_name_with_revision)
    if project_type == self._PROJECT_BROWSER:
      gn_name_with_revision = 'gn_targets_%s_%s.json' % (self.c.PLATFORM,
                                                         commit_position)
      self._upload_gn_targets_json(self.c.bucket_name, gn_name_with_revision)

    return index_pack_kythe_path

  def _upload_compile_commands_json(self, bucket_name, destination_filename):
    """Upload the compile_commands.json file to Google Storage.

    This is useful for debugging.

    Args:
      bucket_name: Name of the Google Storage bucket to upload to
      destination_filename: Name to use for the compile_commands file in
                            Google Storage
    """
    self.m.gsutil.upload(
        name='upload compile_commands.json',
        source=self.c.compile_commands_json_file,
        bucket=bucket_name,
        dest='debug/%s' % destination_filename,
        dry_run=self._is_experimental)

  def _upload_gn_targets_json(self, bucket_name, destination_filename):
    """Upload the gn_targets.json file to Google Storage.

    This is useful for debugging.

    Args:
      bucket_name: Name of the Google Storage bucket to upload to
      destination_filename: Name to use for the compile_commands file in
                            Google Storage
    """
    self.m.gsutil.upload(
        name='upload gn_targets.json',
        source=self.c.gn_targets_json_file,
        bucket=bucket_name,
        dest='debug/%s' % destination_filename,
        dry_run=self._is_experimental)

  def checkout_generated_files_repo_and_sync(self,
                                             copy,
                                             revision,
                                             kzip_path=None,
                                             ignore=None):
    """Check out the generated files repo and sync the generated files
       into this checkout.

    Args:
      copy: A dict that describes how generated files should be synced. Keys are
        paths to local directories and values are where they are copied to in
        the generated files repo.

          {
              '/path/to/foo': 'foo',
              '/path/to/bar': 'baz/bar',
          }

        The above copy config would result in a generated files repo like:

          repo/
          repo/foo/
          repo/baz/bar/

      kzip_path: Path to kzip that will be used to prune uploaded files.
      ignore: List of paths that shouldn't be synced.
      revision: A commit hash to be used in the commit message.
    """
    if not self.c.SYNC_GENERATED_FILES:
      return
    if self.m.tryserver.is_tryserver:  # pragma: no cover
      return
    assert self.c.generated_repo, ('Trying to check out generated files repo,'
                                   ' but the repo is not indicated')

    # Check out the generated files repo. We use a named cache so that the
    # checkout stays around between builds (this saves ~15 mins of build time).
    generated_repo_dir = self.m.path.cache_dir / 'generated'

    # Windows is unable to checkout files with names longer than 260 chars.
    # This git setting works around this limitation.
    if self.c.PLATFORM.startswith('win'):
      try:
        with self.m.context(cwd=generated_repo_dir):
          self.m.git(
              'config', 'core.longpaths', 'true', name='set core.longpaths')
      except self.m.step.StepFailure as f:  # pragma: nocover
        # If the bot runs with an empty cache, generated_repo_dir won't be a git
        # directory yet, causing git config to fail. In this case, we should
        # continue the run anyway. If the checkout fails on the next step due to
        # a long filename, this is no big deal as it should pass on the next
        # run.
        self.m.step.active_result.presentation.step_text = f.reason_message()
        self.m.step.active_result.presentation.status = self.m.step.WARNING

    env = {
        # Turn off the low speed limit, since checkout will be long.
        'GIT_HTTP_LOW_SPEED_LIMIT': '0',
        'GIT_HTTP_LOW_SPEED_TIME': '0',
    }
    with self.m.context(env=env):
      self.m.git.checkout(
          self.c.generated_repo,
          ref=self.c.GEN_REPO_BRANCH,
          dir_path=generated_repo_dir,
          submodules=False,
          depth=1)
    with self.m.context(cwd=generated_repo_dir):
      self.m.git('config', 'user.email', self.c.generated_author_email)
      self.m.git('config', 'user.name', self.c.generated_author_name)

    # Sync the generated files into this checkout.
    cmd = ['vpython3', self.resource('sync_generated_files.py')]
    for src, dest in copy.items():
      cmd.extend(['--copy', '%s;%s' % (src, dest)])
    cmd.extend([
        '--message',
        'Generated files from "%s" build %d, revision %s' %
        (self.m.buildbucket.builder_name, self.m.buildbucket.build.id,
         revision),
        '--dest-branch',
        self.c.GEN_REPO_BRANCH,
        generated_repo_dir,
    ])

    if self._is_experimental:
      cmd.append('--dry-run')
    if kzip_path:
      cmd.extend(['--kzip-prune', kzip_path])
    if ignore:
      for i in ignore:
        cmd.extend(['--ignore', i])

    if self._get_project_type() == self._PROJECT_BROWSER:
      cmd.append('--nokeycheck')

    self.m.step('sync generated files', cmd)
