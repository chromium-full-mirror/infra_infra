// Copyright 2016 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package handler

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"google.golang.org/appengine"
	"google.golang.org/grpc"

	"go.chromium.org/luci/common/clock"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/gae/service/datastore"
	"go.chromium.org/luci/gae/service/info"
	"go.chromium.org/luci/server/auth/xsrf"
	"go.chromium.org/luci/server/caching"
	"go.chromium.org/luci/server/router"

	"go.chromium.org/infra/appengine/sheriff-o-matic/som/model"
	monorailv3 "go.chromium.org/infra/monorailv2/api/v3/api_proto"
)

const (
	annotationsCacheKey = "annotation-metadata"
	// annotations will expire after this amount of time
	annotationExpiration = time.Hour * 24 * 10
	// maxMonorailQuerySize is the maximum number of bugs per monorail query.
	maxMonorailQuerySize = 100
)

// AnnotationsIssueClient is for testing purpose
type AnnotationsIssueClient interface {
	SearchIssues(context.Context, *monorailv3.SearchIssuesRequest, ...grpc.CallOption) (*monorailv3.SearchIssuesResponse, error)
}

// AnnotationHandler handles annotation-related requests.
type AnnotationHandler struct {
}

// MonorailBugData wrap around monorailv3.Issue to send to frontend.
type MonorailBugData struct {
	BugID     string `json:"id,omitempty"`
	ProjectID string `json:"projectId,omitempty"`
	Summary   string `json:"summary,omitempty"`
	Status    string `json:"status,omitempty"`
}

// AnnotationResponse ... The Annotation object extended with cached bug data.
type AnnotationResponse struct {
	model.Annotation
	BugData map[string]MonorailBugData `json:"bug_data"`
}

var metadataCache = caching.RegisterLRUCache[string, []*MonorailBugData](5)

func convertAnnotationsNonGroupingToAnnotations(annotationsNonGrouping []*model.AnnotationNonGrouping, annotations *[]*model.Annotation) {
	*annotations = make([]*model.Annotation, len(annotationsNonGrouping))
	for i, annotationNonGrouping := range annotationsNonGrouping {
		tmp := model.Annotation(*annotationNonGrouping)
		(*annotations)[i] = &tmp
	}
}

func convertAnnotationsToAnnotationsNonGrouping(annotations []*model.Annotation) []*model.AnnotationNonGrouping {
	annotationsNonGrouping := make([]*model.AnnotationNonGrouping, len(annotations))
	for i, annotation := range annotations {
		tmp := model.AnnotationNonGrouping(*annotation)
		annotationsNonGrouping[i] = &tmp
	}
	return annotationsNonGrouping
}

func datastoreGetAnnotation(c context.Context, annotation *model.Annotation) error {
	annotationNonGrouping := model.AnnotationNonGrouping(*annotation)
	err := datastore.Get(c, &annotationNonGrouping)
	if err != nil {
		return err
	}
	*annotation = model.Annotation(annotationNonGrouping)
	return nil
}

func datastorePutAnnotation(c context.Context, annotation *model.Annotation) error {
	annotations := []*model.Annotation{annotation}
	return datastorePutAnnotations(c, annotations)
}

func datastorePutAnnotations(c context.Context, annotations []*model.Annotation) error {
	annotationsNonGrouping := convertAnnotationsToAnnotationsNonGrouping(annotations)
	return datastore.Put(c, annotationsNonGrouping)
}

func datastoreCreateAnnotationQuery() *datastore.Query {
	return datastore.NewQuery("AnnotationNonGrouping")
}

func datastoreGetAnnotationsByQuery(c context.Context, annotations *[]*model.Annotation, q *datastore.Query) error {
	annotationsNonGrouping := []*model.AnnotationNonGrouping{}
	err := datastore.GetAll(c, q, &annotationsNonGrouping)
	if err != nil {
		return err
	}
	convertAnnotationsNonGroupingToAnnotations(annotationsNonGrouping, annotations)
	return nil
}

func datastoreDeleteAnnotations(c context.Context, annotations []*model.Annotation) error {
	annotationsNonGrouping := convertAnnotationsToAnnotationsNonGrouping(annotations)
	return datastore.Delete(c, annotationsNonGrouping)
}

// Convert data from model.Annotation type to AnnotationResponse type by populating monorail data.
func makeAnnotationResponse(annotations *model.Annotation, meta []*MonorailBugData) *AnnotationResponse {
	bugs := make(map[string]MonorailBugData)
	for _, b := range annotations.Bugs {
		for _, mbd := range meta {
			if b.BugID == mbd.BugID && b.ProjectID == mbd.ProjectID {
				bugs[b.BugID] = *mbd
				break
			}
		}
	}
	return &AnnotationResponse{*annotations, bugs}
}

func filterAnnotations(annotations []*model.Annotation, activeKeys map[string]interface{}) []*model.Annotation {
	ret := []*model.Annotation{}
	groups := map[string]interface{}{}

	// Process annotations not belonging to a group
	for _, a := range annotations {
		if _, ok := activeKeys[a.Key]; ok {
			ret = append(ret, a)
			if a.GroupID != "" {
				groups[a.GroupID] = nil
			}
		}
	}

	// Process annotations belonging to a group
	for _, a := range annotations {
		if _, ok := groups[a.Key]; ok {
			ret = append(ret, a)
		}
	}
	return ret
}

// GetAnnotationsHandler retrieves a set of annotations.
func (ah *AnnotationHandler) GetAnnotationsHandler(ctx *router.Context, activeKeys map[string]interface{}) {
	c, w, p := ctx.Request.Context(), ctx.Writer, ctx.Params

	tree := p.ByName("tree")

	q := datastoreCreateAnnotationQuery()

	if tree != "" {
		q = q.Ancestor(datastore.MakeKey(c, "Tree", tree))
	}

	annotations := []*model.Annotation{}
	datastoreGetAnnotationsByQuery(c, &annotations, q)

	annotations = filterAnnotations(annotations, activeKeys)

	meta := []*MonorailBugData{}

	response := make([]*AnnotationResponse, len(annotations))
	for i, a := range annotations {
		response[i] = makeAnnotationResponse(a, meta)
	}

	data, err := json.Marshal(response)
	if err != nil {
		errStatus(c, w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

type postRequest struct {
	XSRFToken string           `json:"xsrf_token"`
	Data      *json.RawMessage `json:"data"`
}

// PostAnnotationsHandler handles updates to annotations.
func (ah *AnnotationHandler) PostAnnotationsHandler(ctx *router.Context) {
	c, w, r, p := ctx.Request.Context(), ctx.Writer, ctx.Request, ctx.Params

	tree := p.ByName("tree")
	action := p.ByName("action")
	if action != "add" && action != "remove" {
		errStatus(c, w, http.StatusBadRequest, "unrecognized annotation action")
		return
	}

	req := &postRequest{}
	err := json.NewDecoder(r.Body).Decode(req)
	if err != nil {
		errStatus(c, w, http.StatusBadRequest, fmt.Sprintf("while decoding request: %s", err))
		return
	}

	if err = xsrf.Check(c, req.XSRFToken); err != nil {
		errStatus(c, w, http.StatusForbidden, err.Error())
		return
	}

	// Extract the annotation key from the otherwise unparsed body.
	rawJSON := struct{ Key string }{}
	if err = json.Unmarshal([]byte(*req.Data), &rawJSON); err != nil {
		errStatus(c, w, http.StatusBadRequest, fmt.Sprintf("while decoding request: %s", err))
	}

	key := rawJSON.Key

	annotation := &model.Annotation{
		Tree:      datastore.MakeKey(c, "Tree", tree),
		KeyDigest: fmt.Sprintf("%x", sha1.Sum([]byte(key))),
		Key:       key,
	}

	err = datastoreGetAnnotation(c, annotation)
	if action == "remove" && err != nil {
		logging.Errorf(c, "while getting %s: %s", key, err)
		errStatus(c, w, http.StatusNotFound, fmt.Sprintf("Annotation %s not found", key))
		return
	}

	if info.AppID(c) != "" && info.AppID(c) != "app" {
		c = appengine.WithContext(c, r)
	}
	// The annotation probably doesn't exist if we're adding something.
	data := bytes.NewReader([]byte(*req.Data))
	if action == "add" {
		_, err = annotation.Add(c, data)
	} else if action == "remove" {
		_, err = annotation.Remove(c, data)
	}

	if err != nil {
		errStatus(c, w, http.StatusBadRequest, err.Error())
		return
	}

	err = r.Body.Close()
	if err != nil {
		errStatus(c, w, http.StatusInternalServerError, err.Error())
		return
	}

	err = datastorePutAnnotation(c, annotation)
	if err != nil {
		errStatus(c, w, http.StatusInternalServerError, err.Error())
		return
	}

	var m []*MonorailBugData
	annotationResp := makeAnnotationResponse(annotation, m)

	resp, err := json.Marshal(annotationResp)
	if err != nil {
		errStatus(c, w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(resp)
}

// FlushOldAnnotationsHandler culls obsolete annotations from the datastore.
// TODO (crbug.com/1079068): Perhaps we want to revisit flush annotation logic.
func FlushOldAnnotationsHandler(ctx context.Context) error {
	numDeleted, err := flushOldAnnotations(ctx)
	if err != nil {
		return err
	}
	logging.Debugf(ctx, "deleted %d annotations", numDeleted)
	return nil
}

func flushOldAnnotations(c context.Context) (int, error) {
	q := datastoreCreateAnnotationQuery()
	q = q.Lt("ModificationTime", clock.Get(c).Now().Add(-annotationExpiration))
	q = q.KeysOnly(true)

	results := []*model.Annotation{}
	err := datastoreGetAnnotationsByQuery(c, &results, q)
	if err != nil {
		return 0, fmt.Errorf("while fetching annotations to delete: %s", err)
	}

	for _, ann := range results {
		logging.Debugf(c, "Deleting %#v\n", ann)
	}

	err = datastoreDeleteAnnotations(c, results)
	if err != nil {
		return 0, fmt.Errorf("while deleting annotations: %s", err)
	}

	return len(results), nil
}
