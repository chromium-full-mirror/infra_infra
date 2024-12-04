FROM ubuntu:24.04
RUN dpkg --add-architecture i386
# chromium uses python3
# ld.ldd requires libxml2 libicu70
# libbase.so requires libglib2.0-0
# libcrcrypto.so requires libnss3
# some actions requires gperf
#  e.g. third_party/blink/renderer/build/scripts/core/css/make_css_value_keywords.py
# wayland_scanner requires libwayland-client0
# libc6:i386, libgcc1:i386 is required for nacl_x86_glibc
# rpm, dpkg-dev, devscripts: for packaging
RUN apt-get update && \
 DEBIAN_FRONTEND=noninteractive apt-get install -y \
   build-essential \
   devscripts \
   dpkg-dev \
   gperf \
   libc6:i386 \
   libgcc1:i386 \
   libglib2.0-0 \
   libicu74 \
   libnss3 \
   libwayland-client0 \
   libxml2 \
   python3 \
   rpm \
   && rm -rf /var/lib/apt/lists/*
