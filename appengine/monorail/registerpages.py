# Copyright 2022 The Chromium Authors
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.
"""This file sets up all the urls for monorail pages."""

import flask

import urllib

from components import endpoints_flask
from components import prpc

from features import banspammer
from features import inboundemail
from features import hotlistcreate
from features import savedqueries
from features import userhotlists
from framework import banned
from framework import clientmon
from framework import csp_report
from framework import warmup
from framework import reap
from framework import deleteusers
from framework import excessiveactivity
from framework import teardown
from framework import ts_mon_js
from framework import trimvisitedpages
from project import peopledetail
from project import peoplelist
from project import projectadmin
from project import projectadminadvanced
from project import projectexport
from project import projectsummary
from project import projectupdates
from project import project_constants
from project import redirects
from services import api_svc_v1
from services import cachemanager_svc
from services import client_config_svc
from sitewide import custom_404
from sitewide import hostinghome
from sitewide import moved
from sitewide import userclearbouncing
from sitewide import userupdates
from sitewide import userprofile
from sitewide import projectcreate
from sitewide import usersettings
from sitewide import groupadmin
from sitewide import groupcreate
from sitewide import groupdetail
from sitewide import grouplist
from features import rerankhotlist
from features import hotlistdetails
from features import hotlistissues
from features import hotlistissuescsv
from features import hotlistpeople
from features import dateaction
from features import filterrules
from features import pubsub
from features import notify
from features import hotlistcreate
from features import savedqueries
from features import userhotlists
from features import banspammer
from search import backendnonviewable
from search import backendsearch
from tracker import componentcreate
from tracker import componentdetail
from tracker import fltconversion
from tracker import fieldcreate
from tracker import fielddetail
from tracker import templatecreate
from tracker import templatedetail
from tracker import issueadmin
from tracker import issueadvsearch
from tracker import issueattachment
from tracker import issueattachmenttext
from tracker import issuebulkedit
from tracker import issuedetailezt
from tracker import issueentry
from tracker import issueentryafterlogin
from tracker import issueexport
from tracker import issueoriginal
from tracker import issuereindex
from tracker import issuetips
from tracker import issueimport

from tracker import webcomponentspage

from api import api_routes as api_routes_v0
from api.v3 import api_routes as api_routes_v3


class ServletRegistry(object):

  _PROJECT_NAME_REGEX = project_constants.PROJECT_NAME_PATTERN
  _USERNAME_REGEX = r'[-+\w=.%]+(@([-a-z0-9]+\.)*[a-z0-9]+)?'
  _HOTLIST_ID_NAME_REGEX = r'\d+|[a-zA-Z][-0-9a-zA-Z\.]*'

  def __init__(self):
    self.routes = []

  def _AppendUrlToRoutes(self, rule_tuple, base=''):
    """Register each of the given servlets."""
    for rule in rule_tuple:
      self.routes.append([base + rule[0], rule[1], rule[2]])

  def Register(self, services, flask_instance):
    """Register all the monorail request handlers."""
    # self._RegisterGroupUrls(services)
    # self._RegisterHostingUrl(services)
    # self._RegisterOldHostUrl(services)
    # self._RegisterRedirectProjectUrl()
    self._RegisterCSPUrl()
    # self._RegisterProjectUrls(services, flask_instance)
    # self._RegisterUserUrls(services)
    self._RegisterTaskUrl(services)
    self._RegisterCronUrl(services)
    # self._RegisterBackendUrl(services)
    self._RegisterMONSetUrl(services)
    self._RegisterAHUrl(services)
    # self._RegisterPrpcUrl(services)
    # self._RegisterWebComponentsUrl(services)
    self._RegisterFlaskUrlRules(flask_instance, self.routes)

  def _RegisterFlaskUrlRules(
      self, flask_instance, rule_tuple, removed_prefix=''):
    """Add url rules to a given Flask instance.

    Args:
      flask_instance: The Flask app to add URLs to.
      rule_tuple: List of tuple of path, module and method to call, HTTP method

    Returns:
      The Flask instance.
    """
    for rule in rule_tuple:
      url = rule[0][len(removed_prefix):] if rule[0].startswith(
          removed_prefix) else rule[0]
      flask_instance.add_url_rule(url, view_func=rule[1], methods=rule[2])

    flask_instance.register_error_handler(404, _Handle404)

    return flask_instance

  # pylint: disable=unused-argument
  def _RegisterGroupUrls(self, services):
    _GROUP_URL = [
        (
            '/', grouplist.GroupList(services=services).GetGroupList,
            ['GET']),
        (
            '/<string:viewed_username>/',
            groupdetail.GroupDetail(services=services).GetGroupDetail, ['GET']),
        (
            '/<string:viewed_username>/edit.do',
            groupdetail.GroupDetail(services=services).PostGroupDetail,
            ['POST']),
        (
            '/<string:viewed_username>/groupadmin',
            groupadmin.GroupAdmin(services=services).GetGroupAdmin, ['GET']),
        (
            '/<string:viewed_username>/groupadmin.do',
            groupadmin.GroupAdmin(services=services).PostGroupAdmin, ['POST']),
    ]
    self._AppendUrlToRoutes(_GROUP_URL, '/g')

  # pylint: disable=unused-argument
  def _RegisterHostingUrl(self, service):

    def DefaultToMainPage():
      url = flask.request.host_url
      return flask.redirect(url)

    _HOSTING_URL = [
        (
            '/',
            DefaultToMainPage,
            ['GET'],
        ),
        (
            '/excessiveActivity',
            excessiveactivity.ExcessiveActivity(
                services=service).GetExcessiveActivity, ['GET']),
        (
            '/settings',
            usersettings.UserSettings(services=service).GetUserSetting, ['GET'
                                                                        ]),
        (
            '/settings.do',
            usersettings.UserSettings(services=service).PostUserSetting,
            ['POST']),
        ('/noAccess', banned.Banned(services=service).GetNoAccessPage, ['GET']),
        (
            '/moved', moved.ProjectMoved(services=service).GetProjectMoved,
            ['GET']),
        (
            '/createProject',
            projectcreate.ProjectCreate(services=service).GetCreateProject,
            ['GET']),
        (
            '/createProject.do',
            projectcreate.ProjectCreate(services=service).PostCreateProject,
            ['POST']),
        (
            '/createHotlist',
            hotlistcreate.HotlistCreate(services=service).GetCreateHotlist,
            ['GET']),
        (
            '/createHotlist.do',
            hotlistcreate.HotlistCreate(services=service).PostCreateHotlist,
            ['POST']),
        (
            '/createGroup',
            groupcreate.GroupCreate(services=service).GetGroupCreate, ['GET']),
        (
            '/createGroup.do',
            groupcreate.GroupCreate(services=service).PostGroupCreate, ['POST'
                                                                       ]),
        (
            '/deleteGroup',
            grouplist.GroupList(services=service).GetGroupDelete, ['GET']),
        (
            '/deleteGroup.do',
            grouplist.GroupList(services=service).PostGroupDelete,
            ['POST']),
    ]

    self._AppendUrlToRoutes(_HOSTING_URL, '/hosting')

  def _RegisterOldHostUrl(self, service):
    _OLD_HOSTING_URL = [
        (
            '/hosting_old/',
            hostinghome.HostingHome(services=service).GetOldHostingHome,
            ['GET']),
    ]
    self._AppendUrlToRoutes(_OLD_HOSTING_URL, '')

  def _RegisterRedirectProjectUrl(self):

    def GetRedirectProject():
      url = flask.request.host_url
      return flask.redirect(url)

    _PROJECT_REDIRECT_URL = [
        ('/projects/', GetRedirectProject, ['GET']),
    ]
    self._AppendUrlToRoutes(_PROJECT_REDIRECT_URL, '')

  def _RegisterCSPUrl(self):
    self._AppendUrlToRoutes([('/csp.do', csp_report.postCsp, ['POST'])], '')

  def _RegisterProjectUrls(self, service, flaskapp_project):
    _PROJECT_URLS = [
        (
            '/<string:project_name>/<string:unrecognized>',
            custom_404.ErrorPage(services=service).Get404Page,
            ['GET'],
        ),
        (
            '/<string:project_name>/adminComponents',
            issueadmin.AdminComponents(services=service).GetAdminComponentsPage,
            ['GET']),
        (
            '/<string:project_name>/adminComponents.do',
            issueadmin.AdminComponents(
                services=service).PostAdminComponentsPage, ['POST']),
        (
            '/<string:project_name>/adminIntro',
            projectsummary.ProjectSummary(
                services=service).GetProjectSummaryPage, ['GET']),
        (
            '/<string:project_name>/adminLabels',
            issueadmin.AdminLabels(services=service).GetAdminLabelsPage,
            ['GET']),
        (
            '/<string:project_name>/adminLabels.do',
            issueadmin.AdminLabels(services=service).PostAdminLabelsPage,
            ['POST']),
        (
            '/<string:project_name>/adminRules',
            issueadmin.AdminRules(services=service).GetAdminRulesPage, ['GET']),
        (
            '/<string:project_name>/adminRules.do',
            issueadmin.AdminRules(services=service).PostAdminRulesPage,
            ['POST']),
        (
            '/<string:project_name>/adminStatuses',
            issueadmin.AdminStatuses(services=service).GetAdminStatusesPage,
            ['GET']),
        (
            '/<string:project_name>/adminStatuses.do',
            issueadmin.AdminStatuses(services=service).PostAdminStatusesPage,
            ['POST']),
        (
            '/<string:project_name>/adminTemplates',
            issueadmin.AdminTemplates(services=service).GetAdminTemplatesPage,
            ['GET']),
        (
            '/<string:project_name>/adminTemplates.do',
            issueadmin.AdminTemplates(services=service).PostAdminTemplatesPage,
            ['POST']),
        (
            '/<string:project_name>/adminViews',
            issueadmin.AdminViews(services=service).GetAdminViewsPage, ['GET']),
        (
            '/<string:project_name>/adminViews.do',
            issueadmin.AdminViews(services=service).PostAdminViewsPage,
            ['POST']),
        (
            '/<string:project_name>/admin',
            projectadmin.ProjectAdmin(services=service).GetProjectAdminPage,
            ['GET']),
        (
            '/<string:project_name>/admin.do',
            projectadmin.ProjectAdmin(services=service).PostProjectAdminPage,
            ['POST']),
        (
            '/<string:project_name>/adminAdvanced',
            projectadminadvanced.ProjectAdminAdvanced(
                services=service).GetProjectAdminAdvancedPage, ['GET']),
        (
            '/<string:project_name>/adminAdvanced.do',
            projectadminadvanced.ProjectAdminAdvanced(
                services=service).PostProjectAdminAdvancedPage, ['POST']),
        (
            '/<string:project_name>/components/create',
            componentcreate.ComponentCreate(
                services=service).GetComponentCreatePage, ['GET']),
        (
            '/<string:project_name>/components/create.do',
            componentcreate.ComponentCreate(
                services=service).PostComponentCreatePage, ['POST']),
        (
            '/<string:project_name>/components/detail',
            componentdetail.ComponentDetail(
                services=service).GetComponentDetailPage, ['GET']),
        (
            '/<string:project_name>/components/detail.do',
            componentdetail.ComponentDetail(
                services=service).PostComponentDetailPage, ['POST']),
        (
            '/<string:project_name>/fields/create',
            fieldcreate.FieldCreate(services=service).GetFieldCreate, ['GET']),
        (
            '/<string:project_name>/fields/create.do',
            fieldcreate.FieldCreate(services=service).PostFieldCreate, ['POST'
                                                                       ]),
        (
            '/<string:project_name>/fields/detail',
            fielddetail.FieldDetail(services=service).GetFieldDetail, ['GET']),
        (
            '/<string:project_name>/fields/detail.do',
            fielddetail.FieldDetail(services=service).PostFieldDetail, ['POST'
                                                                       ]),
        (
            '/<string:project_name>/issues/advsearch',
            issueadvsearch.IssueAdvancedSearch(
                services=service).GetIssueAdvSearchPage, ['GET']),
        (
            '/<string:project_name>/issues/advsearch.do',
            issueadvsearch.IssueAdvancedSearch(
                services=service).PostIssueAdvSearchPage, ['POST']),
        (
            '/<string:project_name>/issues/detail',
            webcomponentspage.WebComponentsPage(
                services=service).GetWebComponentsIssueDetail, ['GET']),
        (
            '/<string:project_name>/issues/export',
            issueexport.IssueExport(services=service).GetIssueExport, ['GET']),
        (
            '/<string:project_name>/issues/export/json',
            issueexport.IssueExportJSON(services=service).GetIssueExportJSON,
            ['GET']),
        (
            '/<string:project_name>/issues/export/json.do',
            issueexport.IssueExportJSON(services=service).PostIssueExportJSON,
            ['POST']),
        (
            '/<string:project_name>/issues/import',
            issueimport.IssueImport(services=service).GetIssueImport, ['GET']),
        (
            '/<string:project_name>/issues/import.do',
            issueimport.IssueImport(services=service).PostIssueImport, ['POST'
                                                                       ]),
        (
            '/<string:project_name>/issues/original',
            issueoriginal.IssueOriginal(services=service).GetIssueOriginal,
            ['GET']),
        (
            '/<string:project_name>/issues/entry',
            issueentry.IssueEntry(services=service).GetIssueEntry, ['GET']),
        (
            '/<string:project_name>/issues/entry.do',
            issueentry.IssueEntry(services=service).PostIssueEntry, ['POST']),
        (
            '/<string:project_name>/issues/entry_new',
            webcomponentspage.WebComponentsPage(
                services=service).GetWebComponentsIssueNewEntry, ['GET']),
        (
            '/<string:project_name>/issues/list',
            webcomponentspage.WebComponentsPage(
                services=service).GetWebComponentsIssueList, ['GET']),
        (
            '/<string:project_name>/issues/reindex',
            issuereindex.IssueReindex(services=service).GetIssueReindex,
            ['GET']),
        (
            '/<string:project_name>/issues/reindex.do',
            issuereindex.IssueReindex(services=service).PostIssueReindex,
            ['POST']),
        (
            '/<string:project_name>/issues/detail/list',
            issuedetailezt.FlipperList(services=service).GetFlipperList,
            ['GET']),
        (
            '/<string:project_name>/issues/detail/flipper',
            issuedetailezt.FlipperIndex(services=service).GetFlipperIndex,
            ['GET']),
        (
            '/<string:project_name>/issues/detail/flipper.do',
            issuedetailezt.FlipperIndex(services=service).PostFlipperIndex,
            ['POST']),
        (
            '/<string:project_name>/issues/wizard',
            webcomponentspage.WebComponentsPage(
                services=service).GetWebComponentsIssueWizard, ['GET']),
        (
            '/<string:project_name>/templates/create',
            templatecreate.TemplateCreate(services=service).GetTemplateCreate,
            ['GET']),
        (
            '/<string:project_name>/templates/create.do',
            templatecreate.TemplateCreate(services=service).PostTemplateCreate,
            ['POST']),
        (
            '/<string:project_name>/templates/detail',
            templatedetail.TemplateDetail(services=service).GetTemplateDetail,
            ['GET']),
        (
            '/<string:project_name>/templates/detail.do',
            templatedetail.TemplateDetail(services=service).PostTemplateDetail,
            ['POST']),
        (
            '/<string:project_name>/people/list',
            peoplelist.PeopleList(services=service).GetPeopleListPage, ['GET']),
        (
            '/<string:project_name>/people/list.do',
            peoplelist.PeopleList(services=service).PostPeopleListPage,
            ['POST']),
        (
            '/<string:project_name>/people/detail',
            peopledetail.PeopleDetail(services=service).GetPeopleDetailPage,
            ['GET']),
        (
            '/<string:project_name>/people/detail.do',
            peopledetail.PeopleDetail(services=service).PostPeopleDetailPage,
            ['POST']),
        (
            '/<string:project_name>/projectExport',
            projectexport.ProjectExport(services=service).GetProjectExportPage,
            ['GET']),
        (
            '/<string:project_name>/projectExport/json',
            projectexport.ProjectExportJSON(
                services=service).GetProjectExportJSONPage, ['GET']),
        (
            '/<string:project_name>/projectExport/json.do',
            projectexport.ProjectExportJSON(
                services=service).PostProjectExportJSONPage, ['POST']),
        (
            '/<string:project_name>/updates/list',
            projectupdates.ProjectUpdates(
                services=service).GetProjectUpdatesPage, ['GET']),
        (
            '/<string:project_name>/w/list',
            redirects.WikiRedirect(services=service).GetWikiListRedirect,
            ['GET']),
        (
            '/<string:project_name>/wiki/<string:wiki_page>',
            redirects.WikiRedirect(services=service).GetWikiRedirect, ['GET']),
        (
            '/<string:project_name>/source/<string:source_page>',
            redirects.SourceRedirect(services=service).GetSourceRedirect,
            ['GET']),
        (
            '/<string:project_name>/issues/entryafterlogin',
            issueentryafterlogin.IssueEntryAfterLogin(
                services=service).GetIssueEntryAfterLogin,
            ['GET'],
        ),
        (
            '/<string:project_name>/issues/searchtips',
            issuetips.IssueSearchTips(services=service).GetIssueSearchTips,
            ['GET'],
        ),
        (
            '/<string:project_name>/issues/attachment',
            issueattachment.AttachmentPage(services=service).GetAttachmentPage,
            ['GET'],
        ),
        (
            '/<string:project_name>/issues/attachmentText',
            issueattachmenttext.AttachmentText(
                services=service).GetAttachmentText,
            ['GET'],
        ),
        (
            '/<string:project_name>/issues/bulkedit',
            issuebulkedit.IssueBulkEdit(services=service).GetIssueBulkEdit,
            ['GET']),
        (
            '/<string:project_name>/issues/bulkedit.do',
            issuebulkedit.IssueBulkEdit(services=service).PostIssueBulkEdit,
            ['POST']),
        (
            '/<string:project_name>/issues/detail/next',
            issuedetailezt.FlipperNext(
                services=service).GetFlipperNextRedirectPage, ['GET']),
        (
            '/<string:project_name>/issues/detail/previous',
            issuedetailezt.FlipperPrev(
                services=service).GetFlipperPrevRedirectPage, ['GET']),
    ]
    self._AppendUrlToRoutes(_PROJECT_URLS, '/p')

    # pylint: disable=unused-variable
    @flaskapp_project.route('/p/<string:project_name>/issues/approval')
    @flaskapp_project.route('/p/<string:project_name>/issues/detail_ezt')
    def ProjectRedirectToIssueDetail(project_name):
      host_url = flask.request.host_url
      url = host_url + 'p/' + project_name + '/issues/detail'
      query_string = flask.request.query_string
      if query_string:
        url = '%s?%s' % (url, query_string)
      return flask.redirect(url)

    # pylint: disable=unused-variable
    @flaskapp_project.route('/p/<string:project_name>/issues/list_new')
    @flaskapp_project.route('/p/<string:project_name>/')
    @flaskapp_project.route('/p/<string:project_name>/issues/')
    def ProjectRedirectToIssueList(project_name):
      host_url = flask.request.host_url
      url = host_url + 'p/' + project_name + '/issues/list'
      query_string = urllib.parse.urlencode(flask.request.args)
      if query_string:
        url = '%s?%s' % (url, query_string)
      return flask.redirect(url)

    # pylint: disable=unused-variable
    @flaskapp_project.route('/p/')
    def ProjectRedirectToMainPage():
      url = flask.request.host_url
      return flask.redirect(url)

    # pylint: disable=unused-variable
    @flaskapp_project.route('/p/<string:project_name>/people/')
    def ProjectRedirectToPeopleList(project_name):
      host_url = flask.request.host_url
      url = host_url + 'p/' + project_name + '/people/list'
      return flask.redirect(url)

  def _RegisterUserUrls(self, service):

    def UserRedirectToMainPage():
      url = flask.request.host_url
      return flask.redirect(url)

    _USER_URLS = [
        (
            '/',
            UserRedirectToMainPage,
            ['GET'],
        ),
        (
            '/<string:viewed_username>/queries',
            savedqueries.SavedQueries(services=service).GetSavedQueriesPage,
            ['GET']),
        (
            '/<string:viewed_username>/queries.do',
            savedqueries.SavedQueries(services=service).PostSavedQueriesPage,
            ['Post']),
        (
            '/<string:viewed_username>/hotlists',
            userhotlists.UserHotlists(services=service).GetUserHotlistsPage,
            ['GET']),
        (
            '/<string:viewed_username>/hotlists.do',
            userhotlists.UserHotlists(services=service).PostUserHotlistsPage,
            ['Post']),
        (
            '/<string:viewed_username>/',
            userprofile.UserProfile(services=service).GetUserProfilePage,
            ['GET']),
        (
            '/<string:viewed_username>/edit.do',
            userprofile.UserProfile(services=service).PostUserProfilePage,
            ['POST']),
        (
            '/<string:viewed_username>/ban.do',
            userprofile.BanUser(services=service).PostBanUserPage, ['POST']),
        (
            '/<string:viewed_username>/banSpammer.do',
            banspammer.BanSpammer(services=service).PostBanSpammerPage,
            ['POST']),
        (
            '/<string:viewed_username>/clearBouncing',
            userclearbouncing.UserClearBouncing(
                services=service).GetUserClearBouncingPage, ['GET']),
        (
            '/<string:viewed_username>/clearBouncing.do',
            userclearbouncing.UserClearBouncing(
                services=service).PostUserClearBouncingPage, ['Post']),
        (
            '/<string:viewed_username>/updates/projects',
            userupdates.UserUpdatesProjects(
                services=service).GetUserUpdatesProjectsPage, ['GET']),
        (
            '/<string:viewed_username>/updates/developers',
            userupdates.UserUpdatesDevelopers(
                services=service).GetUserUpdatesDevelopersPage, ['GET']),
        (
            '/<string:viewed_username>/updates',
            userupdates.UserUpdatesIndividual(
                services=service).GetUserUpdatesPage, ['GET']),
        (
            '/<string:viewed_username>/hotlists/<string:hotlist_id>',
            hotlistissues.HotlistIssues(services=service).GetHotlistIssuesPage,
            ['GET']),
        (
            '/<string:viewed_username>/hotlists/<string:hotlist_id>.do',
            hotlistissues.HotlistIssues(services=service).PostHotlistIssuesPage,
            ['POST']),
        (
            '/<string:viewed_username>/hotlists/<string:hotlist_id>/csv',
            hotlistissuescsv.HotlistIssuesCsv(
                services=service).GetHotlistIssuesCsvPage, ['GET']),
        (
            '/<string:viewed_username>/hotlists/<string:hotlist_id>/people',
            hotlistpeople.HotlistPeopleList(
                services=service).GetHotlistPeoplePage, ['GET']),
        (
            '/<string:viewed_username>/hotlists/<string:hotlist_id>/people.do',
            hotlistpeople.HotlistPeopleList(
                services=service).PostHotlistPeoplePage, ['POST']),
        (
            '/<string:viewed_username>/hotlists/<string:hotlist_id>/details',
            hotlistdetails.HotlistDetails(
                services=service).GetHotlistDetailsPage, ['GET']),
        (
            '/<string:viewed_username>/hotlists/<string:hotlist_id>/details.do',
            hotlistdetails.HotlistDetails(
                services=service).PostHotlistDetailsPage, ['POST']),
        (
            '/<string:viewed_username>/hotlists/<string:hotlist_id>/rerank',
            rerankhotlist.RerankHotlistIssue(
                services=service).GetRerankHotlistIssuePage, ['GET']),
        (
            '/<string:viewed_username>/hotlists/<string:hotlist_id>/rerank.do',
            rerankhotlist.RerankHotlistIssue(
                services=service).PostRerankHotlistIssuePage, ['POST']),
    ]

    self._AppendUrlToRoutes(_USER_URLS, '/u')

  # pylint: disable=unused-argument
  def _RegisterTaskUrl(self, service):
    _TASK_URL = [
        (
            '/banSpammer.do',
            banspammer.BanSpammerTask(services=service).PostBanSpammer,
            ['POST']),
        (
            '/sendWipeoutUserListsTask.do',
            deleteusers.SendWipeoutUserListsTask(
                services=service).PostSendWipeoutUserListsTask, ['POST']),
        (
            '/deleteWipeoutUsersTask.do',
            deleteusers.DeleteWipeoutUsersTask(
                services=service).PostDeleteWipeoutUsersTask, ['POST']),
        (
            '/deleteUsersTask.do',
            deleteusers.DeleteUsersTask(services=service).PostDeleteUsersTask,
            ['POST']),
        (
            '/notifyRulesDeleted.do',
            notify.NotifyRulesDeletedTask(
                services=service).PostNotifyRulesDeletedTask, ['POST']),
        (
            '/notifyIssueChange.do',
            notify.NotifyIssueChangeTask(
                services=service).PostNotifyIssueChangeTask, ['POST']),
        (
            '/notifyBlockingChange.do',
            notify.NotifyBlockingChangeTask(
                services=service).PostNotifyBlockingChangeTask, ['POST']),
        (
            '/notifyBulkEdit.do', notify.NotifyBulkChangeTask(
                services=service).PostNotifyBulkChangeTask, ['POST']),
        (
            '/notifyApprovalChange.do',
            notify.NotifyApprovalChangeTask(
                services=service).PostNotifyApprovalChangeTask, ['POST']),
        (
            '/publishPubsubIssueChange.do',
            pubsub.PublishPubsubIssueChangeTask(
                services=service).PostPublishPubsubIssueChangeTask, ['POST']),
        (
            '/issueDateAction.do',
            dateaction.IssueDateActionTask(
                services=service).PostIssueDateActionTask, ['POST']),
        (
            '/fltConversionTask.do',
            fltconversion.FLTConvertTask(services=service).PostFLTConvertTask,
            ['POST']),
        (
            '/outboundEmail.do',
            notify.OutboundEmailTask(services=service).PostOutboundEmailTask,
            ['POST']),
        (
            '/recomputeDerivedFields.do',
            filterrules.RecomputeDerivedFieldsTask(
                services=service).PostRecomputeDerivedFieldsTask, ['POST']),
    ]
    self._AppendUrlToRoutes(_TASK_URL, '/_task')

  # pylint: disable=unused-argument
  def _RegisterCronUrl(self, service):
    _CRON_URL = [
        (
            '/wipeoutSync',
            deleteusers.WipeoutSyncCron(services=service).GetWipeoutSyncCron,
            ['GET']),
        (
            '/reindexQueue',
            filterrules.ReindexQueueCron(services=service).GetReindexQueueCron,
            ['GET']),
        (
            '/dateAction',
            dateaction.DateActionCron(services=service).GetDateActionCron,
            ['GET']),
        (
            '/ramCacheConsolidate',
            cachemanager_svc.RamCacheConsolidate(
                services=service).GetRamCacheConsolidate, ['GET']),
        ('/reap', reap.Reap(services=service).GetReap, ['GET']),
        (
            '/loadApiClientConfigs', client_config_svc.GetLoadApiClientConfigs,
            ['GET']),
        (
            '/trimVisitedPages',
            trimvisitedpages.TrimVisitedPages(
                services=service).GetTrimVisitedPages, ['GET']),
    ]
    self._AppendUrlToRoutes(_CRON_URL, '/_cron')

  # pylint: disable=unused-argument
  def _RegisterBackendUrl(self, service):
    _BACKEND_URL = [
        (
            '/search',
            backendsearch.BackendSearch(services=service).GetBackendSearch,
            ['GET']),
        (
            '/nonviewable',
            backendnonviewable.BackendNonviewable(
                services=service).GetBackendNonviewable, ['GET']),
    ]
    self._AppendUrlToRoutes(_BACKEND_URL, '/_backend')

  # pylint: disable=unused-argument
  def _RegisterMONSetUrl(self, service):
    _MON_URL = [
        (
            '/clientmon.do',
            clientmon.ClientMonitor(services=service).PostClientMonitor,
            ['POST']),
        (
            '/jstsmon.do',
            ts_mon_js.MonorailTSMonJSHandler(
                services=service).PostMonorailTSMonJSHandler,
            ['POST'],
        )
    ]
    self._AppendUrlToRoutes(_MON_URL, '/_')

  def _RegisterAHUrl(self, service):
    _AH_URL = [
        ('/warmup', warmup.Warmup, ['GET']), ('/start', warmup.Start, ['GET']),
        ('/stop', warmup.Stop, ['GET']),
        (
            '/bounce',
            inboundemail.BouncedEmail(services=service).postBouncedEmail,
            ['POST']),
        (
            '/mail/<string:project_addr>',
            inboundemail.InboundEmail(services=service).HandleInboundEmail,
            ['GET', 'POST'])
    ]
    self._AppendUrlToRoutes(_AH_URL, '/_ah')

  def _RegisterPrpcUrl(self, service):
    prpc_server = prpc.FlaskServer(
        allowed_origins=client_config_svc.GetAllowedOriginsSet())
    api_routes_v0.RegisterApiHandlers(prpc_server, service)
    api_routes_v3.RegisterApiHandlers(prpc_server, service)
    routes = prpc_server.get_routes()
    self._AppendUrlToRoutes(routes, '')

  def _RegisterWebComponentsUrl(self, service):
    self.routes.append(
        [
            '/',
            webcomponentspage.ProjectListPage(
                services=service).GetProjectListPage, ['GET']
        ])
    self.routes.append(
        [
            '/hotlists/<path:subpath>',
            webcomponentspage.WebComponentsPage(
                services=service).GetWebComponentsHotlist, ['GET']
        ])
    self.routes.append(
        [
            '/users/<path:subpath>',
            webcomponentspage.WebComponentsPage(
                services=service).GetWebComponentsUser, ['GET']
        ])


def RegisterEndpointsUrls(app):
  api_classes = [api_svc_v1.MonorailApi, api_svc_v1.ClientConfigApi]
  routes = endpoints_flask.api_routes(api_classes, '/_ah/api')
  for rule, endpoint, view_func, methods in routes:
    app.add_url_rule(
        rule, endpoint=endpoint, view_func=view_func, methods=methods)
  app.view_functions['cors_handler'] = endpoints_flask.cors_handler


def RegisterTeardown(app):
  app.teardown_request(teardown.Teardown)


def _Handle404(e):
  return flask.render_template('404.html'), 404
