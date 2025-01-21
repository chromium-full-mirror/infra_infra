# Copyright 2019 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Definitions of infra.git CI resources."""

load("//lib/build.star", "build")
load("//lib/infra.star", "infra")
load("//lib/recipes.star", "recipes")

infra.console_view(name = "infra", title = "infra/infra repository console")
infra.cq_group(name = "infra", tree_status_name = "infra")

def ci_builder(
        name,
        os,
        cpu = None,
        recipe = None,
        use_python3 = True,
        console_category = None,
        properties = None,
        extra_dimensions = None,
        schedule = None,
        infra_triggered = True,
        tree_closing = False,
        **kwargs):
    infra.builder(
        name = name,
        bucket = "ci",
        executable = infra.recipe(recipe or "infra_continuous", use_python3 = use_python3),
        os = os,
        cpu = cpu,
        triggered_by = [infra.poller()] if infra_triggered else None,
        schedule = schedule,
        properties = properties,
        extra_dimensions = extra_dimensions,
        notifies = infra.tree_closing_notifiers() if tree_closing else None,
        **kwargs
    )
    luci.console_view_entry(
        builder = name,
        console_view = "infra",
        category = console_category or infra.category_from_os(os),
    )

def try_builder(
        name,
        os,
        cpu = None,
        recipe = None,
        disable_reuse = None,
        experiment_percentage = None,
        owner_whitelist = None,
        mode_allowlist = None,
        properties = None,
        caches = None,
        in_cq = True,
        use_python3 = True,
        location_filters = None):
    infra.builder(
        name = name,
        bucket = "try",
        executable = infra.recipe(recipe or "infra_repo_trybot", use_python3 = use_python3),
        os = os,
        cpu = cpu,
        properties = properties,
        caches = caches,
    )
    if in_cq:
        luci.cq_tryjob_verifier(
            builder = name,
            cq_group = "infra",
            disable_reuse = disable_reuse,
            experiment_percentage = experiment_percentage,
            location_filters = location_filters,
            owner_whitelist = owner_whitelist,
            mode_allowlist = mode_allowlist,
        )

# CI Linux.
ci_builder(name = "infra-continuous-jammy-64", os = "Ubuntu-22.04", tree_closing = True)
ci_builder(name = "infra-continuous-jammy-arm64", os = "Ubuntu-22.04", cpu = "arm64", console_category = "linux|22.04|ARM", pool = "luci.flex.ci")

# CI OSX.
ci_builder(name = "infra-continuous-mac-10.15-64", os = "Mac-10.15", tree_closing = True)

# CI Win.
ci_builder(name = "infra-continuous-win10-64", os = "Windows-10", tree_closing = True)

# CI for building docker images.
ci_builder(
    name = "infra-continuous-images",
    os = "Ubuntu",  # note: exact Linux version doesn't really matter
    recipe = "images_builder",
    console_category = "misc",
    properties = {
        "mode": "MODE_CI",
        "project": "PROJECT_INFRA",
        "infra": "ci",
        "manifests": ["infra/build/images/deterministic"],
    },
)

# All trybots.
try_builder(name = "infra-try-jammy-64", os = "Ubuntu-22.04")
try_builder(name = "infra-try-mac", os = "Mac-10.15")
try_builder(name = "infra-try-win", os = "Windows-10")

try_builder(
    name = "infra-try-frontend",
    os = "Ubuntu-22.04",
    recipe = "infra_frontend_tester",
    caches = [
        swarming.cache("nodejs"),
        swarming.cache("npmcache"),
    ],
    location_filters = [
        # see infra/recipes/recipes/infra_frontend_tester.py
        cq.location_filter(path_regexp = "appengine/monorail/.+"),
        cq.location_filter(path_regexp = "go/src/infra/appengine/dashboard/frontend/.+"),
        cq.location_filter(path_regexp = "go/src/infra/appengine/chrome-test-health/frontend/.+"),
    ],
)

try_builder(
    name = "infra-go-lint",
    os = "Ubuntu-22.04",
    properties = {
        "run_lint": True,
    },
    disable_reuse = True,
    owner_whitelist = ["project-infra-tryjob-access"],
    mode_allowlist = [cq.MODE_NEW_PATCHSET_RUN],
)

# Experimental trybot for building docker images out of infra.git CLs.
try_builder(
    name = "infra-try-images",
    os = "Ubuntu",
    recipe = "images_builder",
    experiment_percentage = 100,
    properties = {
        "mode": "MODE_CL",
        "project": "PROJECT_INFRA",
        "infra": "try",
        "manifests": ["infra/build/images/deterministic"],
    },
)

# Presubmit trybot.
build.presubmit(
    name = "infra-try-presubmit",
    cq_group = "infra",
    repo_name = "infra",
    os = "Ubuntu-22.04",
)

# Recipes ecosystem.
recipes.simulation_tester(
    name = "infra-continuous-recipes-tests",
    project_under_test = "infra",
    triggered_by = infra.poller(),
    console_view = "infra",
    console_category = "misc",
    os = "Ubuntu-22.04",
)

# Recipe rolls from Infra.
recipes.roll_trybots(
    upstream = "infra",
    downstream = [
        "build",
    ],
    cq_group = "infra",
)

luci.cq_tryjob_verifier(
    builder = "infra-internal:try/build_internal Roll Tester (infra)",
    cq_group = "infra",
)

luci.cq_tryjob_verifier(
    builder = "infra-internal:try/chrome_release Roll Tester (infra)",
    cq_group = "infra",
)

# Tryjobs for 3pp wheel builders.
def wheel_tryjob(builder):
    luci.cq_tryjob_verifier(
        builder = builder,
        cq_group = "infra",
        location_filters = [
            cq.location_filter(path_regexp = "infra/tools/dockerbuild/.+"),
            # Exclude doc-only changes.
            cq.location_filter(
                path_regexp = "infra/tools/dockerbuild/.*README.*",
                exclude = True,
            ),
            cq.location_filter(
                path_regexp = "infra/tools/dockerbuild/OWNERS",
                exclude = True,
            ),
        ],
    )

wheel_tryjob("infra-internal:try/Universal wheel builder")
wheel_tryjob("infra-internal:try/Linux x64 wheel builder")
wheel_tryjob("infra-internal:try/Linux ARM wheel builder py3.8")
wheel_tryjob("infra-internal:try/Linux ARM wheel builder py3.11")
wheel_tryjob("infra-internal:try/Mac wheel builder")
wheel_tryjob("infra-internal:try/Mac ARM64 wheel builder")
wheel_tryjob("infra-internal:try/Windows-x64 wheel builder")
wheel_tryjob("infra-internal:try/Windows-x86 wheel builder")

# Tryjobs for 3pp package builders.
def tpp_tryjob(builder, experiment_percentage = None):
    luci.cq_tryjob_verifier(
        builder = builder,
        cq_group = "infra",
        experiment_percentage = experiment_percentage,
        location_filters = [
            cq.location_filter(path_regexp = "3pp/.+"),
            cq.location_filter(
                gerrit_project_regexp = "infra/infra",
                path_regexp = "go/src/infra/tools/pkgbuild/.+",
            ),
        ],
    )

tpp_tryjob("infra-internal:try/3pp linux-amd64")
tpp_tryjob("infra-internal:try/3pp linux-arm64")
tpp_tryjob("infra-internal:try/3pp linux-armv6l")
tpp_tryjob("infra-internal:try/3pp linux-riscv64", experiment_percentage = 100)
tpp_tryjob("infra-internal:try/3pp mac-amd64")
tpp_tryjob("infra-internal:try/3pp mac-arm64")
tpp_tryjob("infra-internal:try/3pp windows-386")
tpp_tryjob("infra-internal:try/3pp windows-amd64")
tpp_tryjob("infra-internal:try/3pp windows-arm64")

# Placeholder tryjob for Buildbucket integration testing
infra.builder(
    bucket = "try",
    name = "placeholder",
    os = "Ubuntu",
    executable = luci.recipe(
        name = "engine_placeholder",
        recipe = "placeholder",
        cipd_package = "infra/recipe_bundles/chromium.googlesource.com/infra/luci/recipes-py",
        use_python3 = True,
    ),
    properties = {
        "status": "SUCCESS",
    },
)

# (b/324418194): Temporary builder configuration to test autoroller recipe
infra.builder(
    bucket = "try",
    name = "gcs_dep_autoroller_temp",
    os = "Ubuntu",
    executable = infra.recipe("gcs_dep_autoroller", use_python3 = True),
    properties = {
        "source_url": "https://chromium.googlesource.com/chromium/src.git",
        "destination_url": "https://foo.googlesource.com/bar.git",
    },
)
