# Assumptions

The Current Working Directory is $SRC_ROOT/infra/appengine/findit, i.e. the
directory that contains this file. Please `cd` into it for the commands below to
work.

Note:
1. For Mac, if GoogleAppEngineLauncher is used to run Findit locally, you
    may have to set the field "Extra Flags" under "Launch Settings" with value
   "$SRC_ROOT/infra/appengine/findit/waterfall-backend.yaml
    $SRC_ROOT/infra/appengine/findit/waterfall-frontend.yaml".
2. For Windows, you may have to read the contents of the makefile to learn how
   to run all of the commands manually.

# How to run Findit locally?

From command line, run:
  `make run`

Then open http://localhost:8080 for the home page.

# How to run unit tests for Findit?

From command line, run:
 * `make pytest` to run all tests;
 * `make pytest TEST_GLOB=<path to a sub dir>` to run tests in a sub directory;
 * `make pytest TEST_GLOB=<path to a sub dir>:*<test name>*` to run a given test.


If a code path branch is not tested and no line number is shown in the command
line output, you could check the code coverage report shown in the output.

# How to automatically format python code?

From command line, run:
  `git cl format`

# How to deploy to appengine?

## Staging
Deploy to the staging instance (and make it default):
  `make deploy-findit-staging`

## Production
Deploy to analysis.chromium.org (production):
  `make deploy-findit-prod`

Please use [pantheon] to make the new version default.

NOTE: Deployment to staging/prod requires some extra commands to pull in
dependencies which were natively available in past, but now are not, due to many
changes in infra repo. This is a tech debt, which hasn't been resolved, and
probably will not be resolved any time sooner. The complete set of instructions
to deploy can be found at go/deploy-findit

# Code Structure
* Findit
  * [handlers/](handlers/) contains logic to handle incoming http requests
  * [services/](services/) contains core logic for cron jobs.

[pantheon]: https://pantheon.corp.google.com/appengine/versions?project=findit-for-me&src=ac&versionId=alpha&moduleId=default&pli=1&serviceId=default&versionssize=50
