# Copyright 2025 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""The package definition file."""

pkg.declare(
    name = "@infra/config",
    lucicfg = "1.45.0",
)

pkg.entrypoint("main.star")
pkg.entrypoint("dev.star")

pkg.resources(["data/*"])

pkg.options.lint_checks(["default"])
