# Copyright 2016 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.
"""Main program for Monorail Redirect.
"""
import google.appengine.api

from redirect import redirect

app = redirect.GenerateRedirectApp()
app.wsgi_app = google.appengine.api.wrap_wsgi_app(app.wsgi_app)
