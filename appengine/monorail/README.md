# Monorail Issue Tracker Redirect

Monorail used to be the Issue Tracker used by the Chromium project and other
related projects. It was hosted at
[bugs.chromium.org](https://bugs.chromium.org). All projects migrated to the
[Google Issue Tracker](https://issuetracker.google.com/). Migrated issues
automatically redirect to their new locations. This directory contains the code
that dynamically generates these redirects.

# Getting started with Monorail Redirect development

## Testing

```
make pytest
```

To run a single test:

```
vpython3 test.py services/test/issue_svc_test.py::IssueServiceTest::testUpdateIssues_Normal
```

## Release process

See: [Monorail Deployment](http://go/monorail-deploy)
