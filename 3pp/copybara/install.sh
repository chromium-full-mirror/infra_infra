#!/bin/bash
# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

set -e
set -x
set -o pipefail

PREFIX="$1"
DEPS_PREFIX="$2"

# Bazel requires USER to be set, see this bug:
# https://github.com/bazelbuild/bazel/issues/16500
export USER="unused_placeholder_user"
export JAVA_HOME=$DEPS_PREFIX/current

# The source is the GitHub repo: https://github.com/google/copybara
cd copybara
# Build copybara with Bazel and chromium/third_party/jdk
bazel build //java/com/google/copybara
# Create an executable uberjar
bazel build //java/com/google/copybara:copybara_deploy.jar
# Copybara binary is at bazel-bin/java/com/google/copybara/copybara_deploy.jar
cp -r "$(bazel info bazel-bin)/java/com/google/copybara" "$PREFIX"
