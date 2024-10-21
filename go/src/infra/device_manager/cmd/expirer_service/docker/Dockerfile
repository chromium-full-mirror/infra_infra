# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# Use an empty base image
# https://github.com/GoogleContainerTools/distroless
FROM gcr.io/distroless/static:latest

# The binary is built on the host. Copy it into the image.
COPY expirer-service /

USER nobody

ENTRYPOINT ["/expirer-service"]
