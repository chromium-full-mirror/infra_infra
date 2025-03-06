# ChromeOS Skylab Admin

[TOC]

## Overview

Chrome OS Skylab Admin is a Google App Engine app (in Go) that supports the following:

### Keep Devices Healthy

This service is trying to keep devices in a healthy, schedulable state at all
times.
Device health can degrade for many reasons (bad test, provisioning failure,
etc...) and this service supports recovering those devices with the following:

* API to manually schedule device repair jobs (invoked via shivas tool)
* Scheduling automated repair jobs for any devices it detects as unhealthy

### Other functionality

* Read recovery version from [configs](http://cs/h/chrome-internal/chromeos/superproject/+/main:infra/config/lab_platform/generated/stable_versions.cfg)
* Provides API to read recovery version from [Datastore](https://pantheon.corp.google.com/datastore/entities;kind=recoveryVersion;ns=__$DEFAULT$__/query/kind?project=chromeos-skylab-bot-fleet)

## Code/Development Setup

For initial setup, follow the [Chrome Infra Go procedures](https://chromium.googlesource.com/infra/infra/+/refs/heads/main/go/README.md)

For environment setup, follow the [Bootstrap
procedures](https://chromium.googlesource.com/infra/infra/+/refs/heads/main/go/README.md#Bootstrap)

## Application Environments

* Production - [cloud project](https://pantheon.corp.google.com/home/dashboard?project=chromeos-skylab-bot-fleet)
* Staging - [cloud project](https://pantheon.corp.google.com/home/dashboard?project=skylab-staging-bot-fleet)
* Local - 'cd go/src/infra/appengine/crosskylabadmin && make dev'

## Test

* Unit testing - 'cd go/src/infra/appengine/crosskylabadmin && make test'
* Functional/integration testing - No automated testing

## Release Procedures

* cd go/src/infra/appengine/crosskylabadmin
* make up-staging && switch-staging (Staging)
* make up-prod && switch-prod (Production)
