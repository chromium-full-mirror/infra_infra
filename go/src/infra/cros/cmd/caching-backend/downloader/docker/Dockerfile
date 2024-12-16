# Copyright 2022 The LUCI Authors. All rights reserved.
# Use of this source code is governed under the Apache License, Version 2.0
# that can be found in the LICENSE file.

# Have to use ubuntu base image because the downloader depends on some file
# system tools/utils unavailable from the go stdlib.
FROM docker.io/library/ubuntu:24.04

RUN apt-get update && apt-get install -y --no-install-recommends \
  ca-certificates=20240203 \
  squashfs-tools=1:4.6.1-1build1 \
  erofs-utils=1.7.1-1build2 \
  && rm -rf /var/lib/apt/lists/*

# The binary is built on the host. Copy it into the image.
COPY downloader /app/

USER nobody
ENTRYPOINT ["/app/downloader"]
