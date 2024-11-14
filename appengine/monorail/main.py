# Copyright 2016 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.
"""Main program for Monorail.

Monorail is an issue tracking tool that is based on the code.google.com
issue tracker, but it has been ported to Google AppEngine and Google Cloud SQL.
"""
from __future__ import print_function
from __future__ import division
from __future__ import absolute_import

import os
import six

import flask
import google.appengine.api
import google.cloud.logging

# Fix imports before importing gae_ts_mon.
import import_utils

import_utils.FixImports()

import gae_ts_mon

import registerpages
from framework import sorting
from redirect import redirect
from services import service_manager

if os.getenv('GAE_ENV') == 'standard':
  # If this isn't a local server, set up cloud logging.
  client = google.cloud.logging.Client()
  client.setup_logging()

if six.PY3:
  # https://github.com/GoogleCloudPlatform/appengine-python-standard/issues/70
  import functools
  from google.appengine.api import memcache
  unpickler = functools.partial(six.moves.cPickle.Unpickler, encoding='bytes')
  memcache.setup_client(memcache.Client(unpickler=unpickler))

services = service_manager.set_up_services()
sorting.InitializeArtValues(services)

app = flask.Flask(__name__)
app.wsgi_app = google.appengine.api.wrap_wsgi_app(app.wsgi_app)

redirect_app = redirect.GenerateRedirectApp()
app.wsgi_app = redirect.RedirectMiddleware(app.wsgi_app, redirect_app.wsgi_app)

registerpages.ServletRegistry().Register(services, app)
# registerpages.RegisterEndpointsUrls(app)
registerpages.RegisterTeardown(app)

gae_ts_mon.initialize_prod(app)
