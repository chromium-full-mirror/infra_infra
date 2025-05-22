# Copyright 2025 The LUCI Authors. All rights reserved.
# Use of this source code is governed under the Apache License, Version 2.0
# that can be found in the LICENSE file.
#
FROM gcr.io/distroless/static:latest
# The binary is built on the host. Copy it into the image.
COPY fleet_automation_governor /
USER nobody
ENTRYPOINT ["/fleet_automation_governor"]

