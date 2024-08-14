# Copyright 2017 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

from components import auth
from components import config as config_api
from components import endpoints_webapp2
from components import prpc

import webapp2

import notifications

README_MD = (
    'https://chromium.googlesource.com/infra/infra/+/HEAD/'
    'appengine/cr-buildbucket/README.md'
)


class DummyStartHandler(auth.AuthenticatingHandler):  # pragma: no cover
  """Dummy handler for /_ah/start.

  Derived from AuthenticatingHandler to initialize auth db before the first
  request.
  """

  @auth.public
  def get(self):
    pass


class MainHandler(webapp2.RequestHandler):  # pragma: no cover
  """Redirects to README.md."""

  def get(self):
    # `led` fetches the index page to see if Buildbucket is available. It
    # follows HTTP-level redirects. We don't want it to test Gitiles available.
    # Use Javascript-level redirect instead.
    self.response.headers['Content-Type'] = 'text/html; charset=utf-8'
    self.response.write(
        """
      <p>Redirecting to the Buildbucket documentation...</p>
      <script>window.location.replace(%r);</script>
    """ % README_MD
    )


def get_frontend_routes():  # pragma: no cover
  endpoints_services = [
      config_api.ConfigApi,
  ]
  routes = [
      webapp2.Route(r'/_ah/start', DummyStartHandler),
      webapp2.Route(r'/', MainHandler),
  ]
  routes.extend(endpoints_webapp2.api_routes(endpoints_services))
  # /api routes should be removed once clients are hitting /_ah/api.
  routes.extend(
      endpoints_webapp2.api_routes(endpoints_services, base_path='/api')
  )

  prpc_server = prpc.Server()
  prpc_server.add_interceptor(auth.prpc_interceptor)
  routes += prpc_server.get_routes()

  return routes


def get_backend_routes():  # pragma: no cover
  prpc_server = prpc.Server()
  prpc_server.add_interceptor(auth.prpc_interceptor)

  return [  # pragma: no branch
      webapp2.Route(r'/internal/task/buildbucket/notify/<build_id:\d+>',
                    notifications.TaskPublishNotification),
  ] + (prpc_server.get_routes())
