# Copyright 2024 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# We copy the fleetconsoleserver binary into /app/fleetconsoleserver
# and serve from the /app directory.

FROM gcr.io/distroless/static:latest
WORKDIR /

COPY ./fleetconsoleserver /app/fleetconsoleserver
ENTRYPOINT ["/app/fleetconsoleserver"]
