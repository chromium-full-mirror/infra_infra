#!/usr/bin/env bash

# Copyright 2025 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# add ssh keys idempotently
gcloud compute os-login ssh-keys add --key="$(ssh-add -L | grep publickey)" --project=fleet-console-dev

# create a tunnel
gcloud compute ssh alloydb-bastion \
    --project fleet-console-dev \
    --zone us-central1-c \
    --ssh-flag="-L 5432:10.89.112.2:5432"\
    -- -o Hostname=nic0.alloydb-bastion.us-central1-c.c.fleet-console-dev.internal.gcpnode.com