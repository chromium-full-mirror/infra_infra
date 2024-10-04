# Copyright 2019 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Definitions of build.git CI resources."""

load("//lib/build.star", "build")
load("//lib/infra.star", "infra")
load("//lib/recipes.star", "recipes")

REPO_URL = "https://chromium.googlesource.com/chromium/tools/build"

infra.console_view(
    name = "build",
    title = "build repository console",
    repo = REPO_URL,
)

luci.cq_group(
    name = "build",
    watch = cq.refset(repo = REPO_URL, refs = [r"refs/heads/main"]),
    retry_config = cq.RETRY_TRANSIENT_FAILURES,
    user_limits = [
        cq.user_limit(
            name = "chromium-infra-emergency-quota",
            groups = ["chromium-infra-emergency-quota"],
            run = cq.run_limits(max_active = None),
        ),
        cq.user_limit(
            name = "luci-cv-quota-dogfooders",
            groups = ["luci-cv-quota-dogfooders"],
            run = cq.run_limits(max_active = 3),
        ),
        cq.user_limit(
            name = "googlers",
            groups = ["googlers"],
            run = cq.run_limits(max_active = 50),
        ),
    ],
)

# Presubmit trybots.
build.presubmit(
    name = "Build Presubmit",
    cq_group = "build",
    repo_name = "build",
    timeout_s = 600,
)

# Trybot that launches a task via 'led' to verify updated recipes work.
recipes.led_recipes_tester(
    name = "Build Recipes Tester",
    cq_group = "build",
    repo_name = "build",
)

# Recipes ecosystem.
recipes.simulation_tester(
    name = "build-recipes-tests",
    project_under_test = "build",
    triggered_by = build.poller(),
    console_view = "build",
)

# External tester for checking the Universal Test Runner against recipe changes
luci.cq_tryjob_verifier(
    builder = "chromium:try/linux-utr-tester",
    cq_group = "build",
)

# External testers (defined in another projects) for recipe rolls.
luci.cq_tryjob_verifier(
    builder = "infra-internal:try/build_internal Roll Tester (build)",
    cq_group = "build",
)
luci.cq_tryjob_verifier(
    builder = "infra-internal:try/chrome_release Roll Tester (build)",
    cq_group = "build",
)
