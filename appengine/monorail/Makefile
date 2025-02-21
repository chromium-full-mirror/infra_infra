# Copyright 2016 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# Makefile to simplify some common AppEngine actions.
# Use 'make help' for a list of commands.

DEVID = monorail-dev
STAGEID= monorail-staging
PRODID= monorail-prod

GAE_PY?= vpython3 gae.py
DEV_APPSERVER_FLAGS?= --python_virtualenv_path venv --watcher_ignore_re="(.*/lib|.*/node_modules|.*/venv)"

FRONTEND_MODULES?= default

BRANCH_NAME := $(shell git rev-parse --abbrev-ref HEAD)

PY_DIRS = api,businesslogic,features,framework,project,mrproto,search,services,sitewide,testing,tracker

default: help

help:
	@echo "Available commands:"
	@sed -n '/^[a-zA-Z0-9_.]*:/s/:.*//p' <Makefile

# Commands for running locally using dev_appserver.
# devserver requires an application ID (-A) to be specified.
# We are using `-A monorail-staging` because ml spam code is set up
# to impersonate monorail-staging in the local environment.
serve:
	@echo "---[Starting SDK AppEngine Server]---"
	$(GAE_PY) devserver -A monorail-staging -- $(DEV_APPSERVER_FLAGS) 2>&1

# The _remote commands expose the app on 0.0.0.0, so that it is externally
# accessible by hostname:port, rather than just localhost:port.
serve_remote:
	@echo "---[Starting SDK AppEngine Server]---"
	$(GAE_PY) devserver -A monorail-staging -o -- $(DEV_APPSERVER_FLAGS)

pytest:
	vpython3 -m pytest

pylint:
	pylint --py3k *py {$(PY_DIRS)}{/,/test/}*py

generate_requirements_txt:
	vpython3 -m pip freeze > requirements.txt
	printf "# This file is generated from the .vpython3 spec file.\n# Use \`make generate_requirements_txt\` to update.\n$$(cat requirements.txt)" > requirements.txt

deploy_dev:
	$(eval BRANCH_NAME := $(shell git rev-parse --abbrev-ref HEAD))
	@echo "---[Dev $(DEVID)]---"
	$(GAE_PY) upload --tag $(BRANCH_NAME) -A $(DEVID) $(FRONTEND_MODULES)

# AppEngine apps can be tested locally and in non-default versions upload to
# the main app-id, but it is still sometimes useful to have a completely
# separate app-id.  E.g., for testing inbound email, load testing, or using
# throwaway databases.
deploy_staging:
	@echo "---[Staging $(STAGEID)]---"
	$(GAE_PY) upload -A $(STAGEID) $(FRONTEND_MODULES)

# This is our production server that users actually use.
deploy_prod:
	@echo "---[Deploying prod instance $(PRODID)]---"
	$(GAE_PY) upload -A $(PRODID) $(FRONTEND_MODULES)

# Note that we do not provide a command-line way to make the newly-uploaded
# version the default version. This is for two reasons: a) You should be using
# your browser to confirm that the new version works anyway, so just use the
# console interface to make it the default; and b) If you really want to use
# the command line you can use gae.py directly.
