# Copyright 2019 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Definitions of luci-go.git CI resources."""

load("//lib/infra.star", "infra")

REPO_URL = "https://chromium.googlesource.com/infra/luci/luci-go"

infra.console_view(
    name = "luci-go",
    title = "luci-go repository console",
    repo = REPO_URL,
)
infra.cq_group(name = "luci-go", repo = REPO_URL)

def ci_builder(name, os, tree_closing = False, properties = None, use_python3 = True):
    infra.builder(
        name = name,
        bucket = "ci",
        executable = infra.recipe("luci_go", use_python3 = use_python3),
        os = os,
        properties = properties,
        triggered_by = [
            luci.gitiles_poller(
                name = "luci-go-gitiles-trigger",
                bucket = "ci",
                repo = REPO_URL,
                refs = ["refs/heads/main"],
            ),
        ],
        notifies = infra.tree_closing_notifiers() if tree_closing else None,
    )
    luci.console_view_entry(
        builder = name,
        console_view = "luci-go",
        category = infra.category_from_os(os),
    )

def try_builder(
        name,
        os,
        recipe = None,
        properties = None,
        caches = None,
        in_cq = True,
        disable_reuse = None,
        experiment_percentage = None,
        owner_whitelist = None,
        mode_allowlist = None,
        use_python3 = True,
        location_filters = None):
    infra.builder(
        name = name,
        bucket = "try",
        executable = infra.recipe(recipe or "luci_go", use_python3 = use_python3),
        os = os,
        properties = properties,
        caches = caches,
    )
    if in_cq:
        luci.cq_tryjob_verifier(
            builder = name,
            cq_group = "luci-go",
            disable_reuse = disable_reuse,
            experiment_percentage = experiment_percentage,
            owner_whitelist = owner_whitelist,
            mode_allowlist = mode_allowlist,
            location_filters = location_filters,
        )

ci_builder(name = "luci-go-continuous-jammy-64", os = "Ubuntu-22.04", tree_closing = True, properties = {
    "run_integration_tests": True,
})
ci_builder(name = "luci-go-continuous-mac-10.15-64", os = "Mac-10.15", tree_closing = True)
ci_builder(name = "luci-go-continuous-win10-64", os = "Windows-10", tree_closing = True)

try_builder(name = "luci-go-try-linux", os = "Ubuntu-22.04", properties = {
    "run_integration_tests": True,
})
try_builder(name = "luci-go-try-mac", os = "Mac-10.15")
try_builder(name = "luci-go-try-win", os = "Windows-10")

try_builder(
    name = "luci-go-try-presubmit",
    os = "Ubuntu",
    properties = {"presubmit": True},
    mode_allowlist = [cq.MODE_DRY_RUN, cq.MODE_FULL_RUN, cq.MODE_NEW_PATCHSET_RUN],
)

# Experimental trybot for building docker images out of luci-go.git CLs.
try_builder(
    name = "luci-go-try-images",
    os = "Ubuntu",
    recipe = "images_builder",
    experiment_percentage = 100,
    properties = {
        "mode": "MODE_CL",
        "project": "PROJECT_LUCI_GO",
        "infra": "try",
        "manifests": ["infra/build/images/deterministic/luci"],
    },
)

try_builder(
    name = "luci-go-lint",
    os = "Ubuntu-22.04",
    properties = {
        "run_lint": True,
    },
    disable_reuse = True,
    owner_whitelist = ["project-infra-tryjob-access"],
    mode_allowlist = [cq.MODE_NEW_PATCHSET_RUN],
)

try_builder(
    name = "luci-go-try-frontend",
    os = "Ubuntu-22.04",
    recipe = "infra_frontend_tester",
    caches = [
        swarming.cache("nodejs"),
        swarming.cache("npmcache"),
    ],
    location_filters = [
        cq.location_filter(path_regexp = "milo/ui/.+"),
        cq.location_filter(path_regexp = "swarming/server/ui2/.+"),
    ],
)
