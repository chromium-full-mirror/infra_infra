// Copyright 2016 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package model

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"google.golang.org/appengine"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.chromium.org/luci/common/clock"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/gae/service/datastore"
	"go.chromium.org/luci/server/auth"

	"go.chromium.org/infra/appengine/sheriff-o-matic/som/model/gen"
)

const (
	bqDatasetID = "events"
	bqTableID   = "annotations"
	// AlertKeySeparator separates fields in AlertJSONNonGrouping Key.
	AlertKeySeparator = "$!"
)

// Tree is a tree which sheriff-o-matic receives and groups alerts for.
type Tree struct {
	Name                       string   `gae:"$id" json:"name"`
	DisplayName                string   `json:"display_name"`
	AlertStreams               []string `json:"alert_streams,omitempty"`
	BugQueueLabel              string   `json:"bug_queue_label,omitempty"`
	HelpLink                   string   `json:"help_link,omitempty"`
	GerritProject              string   `json:"gerrit_project,omitempty"`
	GerritInstance             string   `json:"gerrit_instance,omitempty"`
	DefaultMonorailProjectName string   `json:"default_monorail_project_name,omitempty"`
	BuildBucketProjectFilter   string   `json:"bb_project_filter"`
	HotlistID                  string   `json:"hotlist_id,omitempty"`
}

// BuildBucketTree holds "tree" information about buildbucket builders.
type BuildBucketTree struct {
	TreeName     string         `json:"tree_name"`
	TreeBuilders []*TreeBuilder `json:"tree_builders"`
}

// TreeBuilder is a builder that belongs to a particular tree or trees.
type TreeBuilder struct {
	Project  string   // buildbucket project name
	Bucket   string   // buildbucket bucket name
	Builders []string // buildbucket builder name
}

// AlertJSON is the JSON blob of an alert for a tree.
// TODO(crbug.com/1043371): Remove this when we disable automatic grouping.
type AlertJSON struct {
	ID           string         `gae:"$id" json:"-"`
	Tree         *datastore.Key `gae:"$parent"`
	Date         time.Time
	Contents     []byte `gae:",noindex"`
	Resolved     bool
	AutoResolved bool
	ResolvedDate time.Time
}

// AlertJSONNonGrouping is the JSON blob of an alert for a tree.
type AlertJSONNonGrouping struct {
	ID           string         `gae:"$id" json:"-"`
	Tree         *datastore.Key `gae:"$parent"`
	Date         time.Time
	Contents     []byte `gae:",noindex"`
	Resolved     bool
	AutoResolved bool
	ResolvedDate time.Time
}

// ResolveRequest is the format of the request to resolve alerts.
type ResolveRequest struct {
	Keys     []string `json:"keys"`
	Resolved bool     `json:"resolved"`
}

// ResolveResponse is the format of the response to resolve alerts.
type ResolveResponse struct {
	Tree     string   `json:"tree"`
	Keys     []string `json:"keys"`
	Resolved bool     `json:"resolved"`
}

// Annotation is any information sheriffs want to annotate an alert with. For
// example, a bug where the cause of the alert is being solved.
// TODO(crbug.com/1043371): Remove this when we disable automatic grouping.
type Annotation struct {
	Tree             *datastore.Key `gae:"$parent"`
	KeyDigest        string         `gae:"$id"`
	Key              string         `gae:",noindex" json:"key"`
	Bugs             []MonorailBug  `gae:",noindex" json:"bugs"`
	Comments         []Comment      `gae:",noindex" json:"comments"`
	SnoozeTime       int            `json:"snoozeTime"`
	GroupID          string         `gae:",noindex" json:"group_id"`
	ModificationTime time.Time
}

// AnnotationNonGrouping is any information sheriffs want to annotate an alert with. For
// example, a bug where the cause of the alert is being solved.
type AnnotationNonGrouping struct {
	Tree             *datastore.Key `gae:"$parent"`
	KeyDigest        string         `gae:"$id"`
	Key              string         `gae:",noindex" json:"key"`
	Bugs             []MonorailBug  `gae:",noindex" json:"bugs"`
	Comments         []Comment      `gae:",noindex" json:"comments"`
	SnoozeTime       int            `json:"snoozeTime"`
	GroupID          string         `gae:",noindex" json:"group_id"`
	ModificationTime time.Time
}

// MonorailBug stores data to differentiate bugs by projects.
type MonorailBug struct {
	BugID     string `json:"id"`        // This should match monorail.Issue.id
	ProjectID string `json:"projectId"` // This should match monorail.Issue.projectId
}

// Comment is the format for the data in the Comments property of an Annotation
type Comment struct {
	Text string    `json:"text"`
	User string    `json:"user"`
	Time time.Time `json:"time"`
}

type annotationAdd struct {
	Time     int           `json:"snoozeTime"`
	Bugs     []MonorailBug `json:"bugs"`
	Comments []string      `json:"comments"`
	GroupID  string        `json:"group_id"`
}

type annotationRemove struct {
	Time     bool          `json:"snoozeTime"`
	Bugs     []MonorailBug `json:"bugs"`
	Comments []int         `json:"comments"`
	GroupID  bool          `json:"group_id"`
}

func appendToBugList(knownBugs []MonorailBug, newBug MonorailBug) []MonorailBug {
	for _, knownBug := range knownBugs {
		if knownBug.BugID == newBug.BugID && knownBug.ProjectID == newBug.ProjectID {
			return knownBugs
		}
	}
	return append(knownBugs, newBug)
}

func removeFromBugList(knownBugs []MonorailBug, bugToRemove MonorailBug) []MonorailBug {
	for i, knownBug := range knownBugs {
		if knownBug.BugID == bugToRemove.BugID && knownBug.ProjectID == bugToRemove.ProjectID {
			return append(knownBugs[:i], knownBugs[i+1:]...)
		}
	}
	return knownBugs
}

// GetStepName retrieves step name for an alert.
func (a *AlertJSONNonGrouping) GetStepName() (string, error) {
	// ID is of format  tree:project:bucket:builder:step:firstFailure
	fields := strings.Split(a.ID, AlertKeySeparator)
	if len(fields) != 6 {
		return "", fmt.Errorf("invalid alert id %q", a.ID)
	}
	return fields[4], nil
}

// GenerateKeyDigest generates KeyDigest field from key for annotations.
func GenerateKeyDigest(key string) string {
	return fmt.Sprintf("%x", sha1.Sum([]byte(key)))
}

// GetStepName retrieves step name for an annotation.
func (a *Annotation) GetStepName() (string, error) {
	// Key is of format  tree.step_name
	treePrefix := a.Tree.StringID() + "."
	if !strings.HasPrefix(a.Key, treePrefix) {
		return "", fmt.Errorf("invalid Key: %q", a.Key)
	}
	return a.Key[len(treePrefix):], nil
}

// IsGroupAnnotation returns whether an annotation is a group annotation or a normal annotation
func (a *Annotation) IsGroupAnnotation() bool {
	pattern := "[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}"
	matched, err := regexp.Match(pattern, []byte(a.Key))
	if err != nil {
		return false
	}
	return matched
}

// Add adds some data to an annotation. Returns true if a refresh of annotation
// metadata (currently monorail data) is required, and any errors encountered.
func (a *Annotation) Add(c context.Context, r io.Reader) (bool, error) {
	change := &annotationAdd{}
	needRefresh := false

	err := json.NewDecoder(r).Decode(change)
	if err != nil {
		return needRefresh, err
	}

	modified := false

	if change.Time != 0 {
		a.SnoozeTime = change.Time
		modified = true
	}

	// Check if changed bugs are new, and append to annotation Bugs list.
	if change.Bugs != nil {
		oldBugsCount := len(a.Bugs)
		for _, newBug := range change.Bugs {
			a.Bugs = appendToBugList(a.Bugs, newBug)
		}
		if oldBugsCount != len(a.Bugs) {
			needRefresh = true
			modified = true
		}
	}

	user := auth.CurrentIdentity(c)
	commentTime := clock.Now(c)
	if change.Comments != nil {
		comments := make([]Comment, len(change.Comments))
		for i, c := range change.Comments {
			comments[i].Text = c
			comments[i].User = user.Email()
			comments[i].Time = commentTime
		}

		a.Comments = append(a.Comments, comments...)
		modified = true
	}

	if change.GroupID != "" {
		a.GroupID = change.GroupID
		modified = true
	}

	if modified {
		a.ModificationTime = clock.Now(c).UTC()
	}

	return needRefresh, nil
}

func intToTimestamp(s int) (*timestamppb.Timestamp, error) {
	if s == 0 {
		return nil, fmt.Errorf("cannot convert 0 to timestamp.Timestamp")
	}

	ret := timestamppb.New(time.Unix(int64(s/1000), 0))
	return ret, ret.CheckValid()
}

// Remove removes some data to an annotation. Returns if a refreshe of annotation
// metadata (currently monorail data) is required, and any errors encountered.
func (a *Annotation) Remove(c context.Context, r io.Reader) (bool, error) {
	change := &annotationRemove{}

	err := json.NewDecoder(r).Decode(change)
	if err != nil {
		return false, err
	}

	modified := false

	if change.Time {
		a.SnoozeTime = 0
		modified = true
	}

	if change.Bugs != nil {
		for _, bug := range change.Bugs {
			a.Bugs = removeFromBugList(a.Bugs, bug)
		}
		modified = true
	}

	// Client passes in a list of comment indices to delete.
	deletedComments := []Comment{}
	for _, i := range change.Comments {
		if i < 0 || i >= len(a.Comments) {
			return false, errors.New("invalid comment index")
		}
		deletedComments = append(deletedComments, a.Comments[i])
		a.Comments = append(a.Comments[:i], a.Comments[i+1:]...)
		modified = true
	}

	if change.GroupID {
		a.GroupID = ""
		modified = true
	}

	if modified {
		a.ModificationTime = clock.Now(c).UTC()
	}

	return false, nil
}

func createAnnotationEvent(ctx context.Context, a *Annotation, operation gen.SOMAnnotationEvent_OperationType) *gen.SOMAnnotationEvent {

	evt := &gen.SOMAnnotationEvent{
		AlertKeyDigest: a.KeyDigest,
		AlertKey:       a.Key,
		RequestId:      appengine.RequestID(ctx),
		Operation:      operation,
	}

	mt := timestamppb.New(a.ModificationTime)
	if err := mt.CheckValid(); err == nil {
		evt.Timestamp = mt
		evt.ModificationTime = mt
	}

	for _, c := range a.Comments {
		ct := timestamppb.New(c.Time)
		if err := ct.CheckValid(); err == nil {
			evt.Comments = append(evt.Comments, &gen.SOMAnnotationEvent_Comment{
				Text: c.Text,
				Time: ct,
			})
		} else {
			logging.Errorf(ctx, "error getting timestamp proto: %v", err)
		}
	}

	return evt
}
