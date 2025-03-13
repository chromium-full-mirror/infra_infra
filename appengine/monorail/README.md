# Monorail Issue Tracker Redirect

Monorail used to be the Issue Tracker used by the Chromium project and other
related projects. It was hosted at
[bugs.chromium.org](https://bugs.chromium.org). All projects migrated to the
[Google Issue Tracker](https://issuetracker.google.com/). Migrated issues
automatically redirect to their new locations. This directory contains the code
that dynamically generates these redirects.

# Getting started with Monorail Redirect development

## Testing

To run all tests:

```
make pytest
```

To run a single test:

```
vpython3 -m pytest redirect/test/redirect_test.py::TestRedirectApp::testNoRedirectIssueList
```

## Local Development Server

One-time setup:

```
gcloud init
gcloud config set project monorail-dev
gcloud auth application-default login
```

To run:

```
make serve
```

## Release process

Monorail Redirect is deployed automatically to monorail-dev and
monorail-staging using LUCI CD. To promote to prod:

```
# Assume you have an infra_internal checkout.
cd ../../../data/gae
git new-branch monorail-deploy
./scripts/promote.py --canary --stable --commit monorail
git cl upload
```

Review the CL and land it. LUCI CD will pick up the change and deploy to prod.
You may check on the deployment status using the
[LUCI UI](https://ci.chromium.org/ui/p/infradata-gae/builders/ci/gae-deploy).
