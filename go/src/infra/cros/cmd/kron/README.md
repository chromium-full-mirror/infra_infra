<!--
Copyright 2024 The Chromium Authors
Use of this source code is governed by a BSD-style license that can be
found in the LICENSE file.
-->

# Kron

Kron is the rewrite of the SuiteScheduler cron scheduling service. It fully
duplicates the core logic that SuiteScheduler provided and added on some extra
layers of debuggability. Configs for this service are still located in the
infra/config-internal repo where SuiteScheduler originally had stored them.

More information can be found at: go/kron-dd.

## Installation

### CIPD

If you have CIPD set up you can fetch the package from there using:

```bash
cipd install chromiumos/infra/kron/linux-amd64 latest
```

If not you will need to set up CIPD:

```bash
# Create a directory for the CIPD root
mkdir ~/cipd
cd ~/cipd/

# Initialize the CIPD root for package installation
cipd init -force

# Add the export command to your .bashrc
echo "export PATH=\$PATH:$(pwd)" >> ~/.bashrc && source ~/.bashrc

# Install the package
cipd install chromiumos/infra/kron/linux-amd64 latest
```

Once installed use the command by calling:

```bash
kron help
```

### Local

The provided makefile has build instructions for the program. To build the files
locally just run:

```bash
make build
```

This will install the package at the project root and you can run the program
using:

```bash
./kron -help
```

## CLI

Kron is made as a CLI application. To run the program use one of
the below commands to access the project.

Before using any command you'll need to gather your authentication tokens using:

```
kron auth-login
```

This will direct you to a G Cloud browser login screen where you'll sign in with
your google credentials. If you are not a Google full time employee (FTE) it is
unlikely that Kron will work given the need for access to internal files and
resources. Please reach out to the TSE team if you encounter this and must use
Kron for your work.

## Commands

### Configs
---
<br>

```bash
kron configs <flags>
```

The `configs` command is used to search through the SuiteScheduler configs. The
command will take in the user input and will output the configs which match the
criteria. To see all flags, and information about their usage, enter:

```bash
kron help configs
```

#### Filters

When ingesting and searching for configs the application defines two types of
filters, top and bottom level filters.

`Top-level` filters define the set of filters that largely define the config
trigger mechanism, e.g. `NEW_BUILD` or `DAILY`. These filters do not care about
the contents of the configs but rather are reducing the domain of configs that
will be sent to the `bottom-level` filters.

`Bottom-level` filters work on the inner contents of the configs, E.g. `name` or
`board`. The bottom level filters will receive its working set from the
top-level filters. This reduces the amount of expensive filtering that is
performed making the CLI run faster when working through large amounts of
configurations.

### Run
---
<br>

**This command is primarily intended for BuildBucket builder use only as it
has the ability to launch real testing requests. Please reach out to the TSE team
before using.**

```bash
kron run <flags>
```


The run command is the entry point to the core scheduling service. This command
will fetch the configs from storage and run the passed in trigger type. The
supported triggers currently are:

* NEW_BUILD
* TIMED_EVENTS (DAILY, WEEKLY, FORTNIGHTLY, and N_DAYS)
* MULTI_DUT
* NEW_BUILDS_3D

When running locally use the `-test` flag to ensure that we are not launching
mass amounts of staging traffic nor clearing the release image pub/sub.

```
kron run -test <flags>
```
