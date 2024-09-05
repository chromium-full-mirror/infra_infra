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
        experiment_percentage = None,
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
            experiment_percentage = experiment_percentage,
            location_filters = location_filters,
        )

# Linux as the main platform to test with the most recent Go version (aka
# "bleeding_edge"). It was picked arbitrarily.
#
# All OSX builders are testing specifically with the older Go version
# (aka "legacy") to reflect the fact that OSX amd64 binaries we build need to
# run on relatively ancient OSX versions that don't support the bleeding edge
# Go.

# CI Linux.
ci_builder(name = "infra-continuous-jammy-64", os = "Ubuntu-22.04", tree_closing = True, properties = {
    "go_version_variant": "bleeding_edge",
})
ci_builder(name = "infra-continuous-jammy-arm64", os = "Ubuntu-22.04", cpu = "arm64", console_category = "linux|22.04|ARM", pool = "luci.flex.ci", properties = {
    "go_version_variant": "bleeding_edge",
})

# CI OSX.
ci_builder(name = "infra-continuous-mac-10.13-64", os = "Mac-10.13", tree_closing = True, properties = {
    "go_version_variant": "legacy",
})
ci_builder(name = "infra-continuous-mac-10.14-64", os = "Mac-10.14", tree_closing = True, properties = {
    "go_version_variant": "legacy",
})
ci_builder(name = "infra-continuous-mac-10.15-64", os = "Mac-10.15", tree_closing = True, properties = {
    "go_version_variant": "legacy",
})

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
try_builder(name = "infra-try-jammy-64", os = "Ubuntu-22.04", properties = {
    "go_version_variant": "bleeding_edge",
})

try_builder(name = "infra-try-mac", os = "Mac-10.15", properties = {
    "go_version_variant": "legacy",
})

# It is occasionally useful to test code on OSX 10.14, but we don't have enough
# capacity to have this trybot in CQ by default. It can be triggered manually
# though.
try_builder(name = "infra-try-mac-10.14", os = "Mac-10.14", properties = {
    "go_version_variant": "legacy",
}, in_cq = False)

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
    name = "infra-analysis",
    os = "Ubuntu-22.04",
    recipe = "tricium_infra",
    properties = {
        "gclient_config_name": "infra",
        "patch_root": "infra",
        "analyzers": ["Copyright", "Gosec", "Spellchecker", "InclusiveLanguageCheck"],
    },
    in_cq = False,
)

try_builder(
    name = "infra-go-lint",
    os = "Ubuntu-22.04",
    properties = {
        "run_lint": True,
    },
    # This is False due to restriction of tricium, see
    # https://crbug.com/1410717#c6 for more details.
    in_cq = False,
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
    builder = "infra-internal:try/build_limited Roll Tester (infra)",
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
