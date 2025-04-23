# Copyright 2019 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Functions and constants related to build.git used by all modules."""

load("//lib/infra.star", "infra")

def poller():
    """Defines a gitiles poller polling build.git repo."""
    return luci.gitiles_poller(
        name = "build-gitiles-trigger",
        bucket = "ci",
        repo = "https://chromium.googlesource.com/chromium/tools/build",
        refs = ["refs/heads/main"],
    )

def recipe(name, use_python3 = False):
    """Defines a recipe hosted in the build.git recipe bundle.

    Args:
      name: name of the recipe.
      use_python3: a boolean to use python3 to run the recipe.

    Returns:
      A luci.recipe(...) object.
    """
    return luci.recipe(
        name = name,
        recipe = name,
        cipd_package = "infra/recipe_bundles/chromium.googlesource.com/chromium/tools/build",
        cipd_version = "refs/heads/main",
        use_python3 = use_python3,
    )

def presubmit(
        *,
        name,
        cq_group,
        repo_name = None,  # e.g. 'infra' or 'luci_py', as expected by the recipe
        run_hooks = True,
        timeout_s = 480,
        os = None,
        experiment_percentage = None):
    """Defines a try builder that runs 'run_presubmit' recipe.

    Args:
      name: name of the builder.
      cq_group: cq group the builder belongs to.
      repo_name: name of the repo this builder runs presubmit for.
      run_hooks: flag for whether running hooks.
      timeout_s: timeout in seconds.
      os: this builder's os dimension.
      experiment_percentage: percentage for CV to trigger this builder. When
        this field is present, the builder is be marked as experimental by CV.
    """
    props = {
        "repo_name": repo_name,
        "$depot_tools/presubmit": {
            "runhooks": run_hooks,
            "timeout_s": timeout_s,
        },
    }
    pool = "luci.infra.try"
    if os and os.startswith("Mac"):
        pool = "luci.flex.try"
    luci.builder(
        name = name,
        bucket = "try",
        executable = build.recipe("presubmit", use_python3 = True),
        properties = props,
        service_account = infra.SERVICE_ACCOUNT_TRY,
        dimensions = {
            "os": os or "Ubuntu-22.04",
            "cpu": "x86-64",
            "pool": pool,
        },
        task_template_canary_percentage = 30,
        resultdb_settings = resultdb.settings(
            enable = True,
        ),
    )
    luci.cq_tryjob_verifier(
        builder = name,
        cq_group = cq_group,
        disable_reuse = True,
        experiment_percentage = experiment_percentage,
    )

build = struct(
    poller = poller,
    recipe = recipe,
    presubmit = presubmit,
)
