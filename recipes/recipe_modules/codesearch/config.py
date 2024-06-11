# Copyright 2017 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

from recipe_engine.config import config_item_context, ConfigGroup
from recipe_engine.config import Single, Static
from recipe_engine.config_types import Path


def BaseConfig(PROJECT,
               CHECKOUT_PATH,
               PLATFORM=None,
               EXPERIMENTAL=False,
               SYNC_GENERATED_FILES=False,
               GEN_REPO_BRANCH='main',
               GEN_REPO_OUT_DIR='',
               CORPUS=None,
               BUILD_CONFIG=None,
               **_kwargs):
  """Filter out duplicate compilation units.

  Args:
    PROJECT: The project this config is for. Only 'chromium', 'chrome',
      'chromiumos' and 'chromeos' are supported currently.
    CHECKOUT_PATH: the source checkout path.
    PLATFORM: The platform or board for which the code is compiled.
    EXPERIMENTAL: If True, appends '_experimental' to the generated kzip file,
      indicating that it isn't ready for ingestion by Kythe, and syncs/uploads
      files in dryrun mode.
    SYNC_GENERATED_FILES: Whether to sync generated files into a git repo.
    GEN_REPO_BRANCH: Which branch in the generated files repo to sync to.
    GEN_REPO_OUT_DIR: Which output dir in the generated files repo to sync to.
    CORPUS: Kythe corpus to specify in the kzip.
    BUILD_CONFIG: Kythe build config to specify in the kzip.
  """
  return ConfigGroup(
      PROJECT=Static(PROJECT),
      CHECKOUT_PATH=Static(CHECKOUT_PATH),
      PLATFORM=Static(PLATFORM),
      EXPERIMENTAL=Static(EXPERIMENTAL),
      SYNC_GENERATED_FILES=Static(SYNC_GENERATED_FILES),
      GEN_REPO_BRANCH=Static(GEN_REPO_BRANCH),
      GEN_REPO_OUT_DIR=Static(GEN_REPO_OUT_DIR),
      CORPUS=Static(CORPUS),
      BUILD_CONFIG=Static(BUILD_CONFIG),
      out_path=Single(Path),
      compile_commands_json_file=Single(Path),
      gn_targets_json_file=Single(Path),
      javac_extractor_output_dir=Single(Path),
      bucket_name=Single(str, required=False),
      generated_repo=Single(str, required=False),
      generated_author_email=Single(
          str,
          required=False,
          empty_val='git-generated-files-sync@chromium.org'),
      generated_author_name=Single(
          str, required=False, empty_val='Automatic Generated Files Sync'),
  )


config_ctx = config_item_context(BaseConfig)


@config_ctx(is_root=True)
def base(c):
  c.out_path = c.CHECKOUT_PATH.joinpath('out', c.GEN_REPO_OUT_DIR or 'Debug')
  c.compile_commands_json_file = c.out_path / 'compile_commands.json'
  c.gn_targets_json_file = c.out_path / 'gn_targets.json'
  c.javac_extractor_output_dir = c.out_path / 'kzip'


@config_ctx(includes=['chromium_gs'])
def chromium(c):
  c.generated_repo = 'https://chromium.googlesource.com/chromium/src/out'


@config_ctx(includes=['chromium_gs'])
def chromiumos(c):
  c.out_path = c.CHECKOUT_PATH.joinpath('out', c.PLATFORM)
  c.generated_repo = (
      'https://chromium.googlesource.com/chromiumos/codesearch/gen/' +
      c.PLATFORM)
  c.compile_commands_json_file = c.out_path / 'compile_commands.json'
  c.gn_targets_json_file = c.out_path / 'gn_targets.json'
  c.javac_extractor_output_dir = None


@config_ctx()
def chromium_gs(c):
  c.bucket_name = 'chrome-codesearch'


@config_ctx(includes=['chrome_gs'])
def chrome(c):
  c.generated_repo = (
      'https://chrome-internal.googlesource.com/chrome/src-internal/out')


@config_ctx(includes=['chrome_gs'])
def chromeos(c):
  c.out_path = c.CHECKOUT_PATH.joinpath('out', c.PLATFORM)
  c.generated_repo = (
      'https://chrome-internal.googlesource.com/chromeos/codesearch/gen/' +
      c.PLATFORM)
  c.compile_commands_json_file = c.out_path / 'compile_commands.json'
  c.gn_targets_json_file = c.out_path / 'gn_targets.json'
  c.javac_extractor_output_dir = None


@config_ctx()
def chrome_gs(c):
  c.bucket_name = 'chrome-internal-codesearch'
