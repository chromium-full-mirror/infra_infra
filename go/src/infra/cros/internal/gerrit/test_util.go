// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package gerrit

import (
	"fmt"

	"github.com/golang/mock/gomock"

	gitilespb "go.chromium.org/luci/common/proto/gitiles"
)

// Native gomock.Eq does not work on protos.
type DownloadFileRequestMatcher struct {
	req *gitilespb.DownloadFileRequest
}

func (m DownloadFileRequestMatcher) Matches(x any) bool {
	req, ok := x.(*gitilespb.DownloadFileRequest)
	if !ok {
		return false
	}
	return m.req.GetProject() == req.GetProject() &&
		m.req.GetPath() == req.GetPath() &&
		m.req.GetCommittish() == req.GetCommittish()
}

func (m DownloadFileRequestMatcher) String() string {
	return fmt.Sprintf("project: %s, path: %s, committish: %s",
		m.req.GetProject(), m.req.GetPath(), m.req.GetCommittish())
}

func DownloadFileRequestEq(req *gitilespb.DownloadFileRequest) gomock.Matcher {
	return DownloadFileRequestMatcher{req}
}

type RefsRequestMatcher struct {
	req *gitilespb.RefsRequest
}

func (m RefsRequestMatcher) Matches(x any) bool {
	req, ok := x.(*gitilespb.RefsRequest)
	if !ok {
		return false
	}
	return m.req.GetProject() == req.GetProject() &&
		m.req.GetRefsPath() == req.GetRefsPath()
}

func (m RefsRequestMatcher) String() string {
	return fmt.Sprintf("project: %s, refs path: %s",
		m.req.GetProject(), m.req.GetRefsPath())
}

func RefsRequestEq(req *gitilespb.RefsRequest) gomock.Matcher {
	return RefsRequestMatcher{req}
}

type ListFilesRequestMatcher struct {
	req *gitilespb.ListFilesRequest
}

func (m ListFilesRequestMatcher) Matches(x any) bool {
	req, ok := x.(*gitilespb.ListFilesRequest)
	if !ok {
		return false
	}
	return m.req.GetProject() == req.GetProject() &&
		m.req.GetPath() == req.GetPath() &&
		m.req.GetCommittish() == req.GetCommittish()
}

func (m ListFilesRequestMatcher) String() string {
	return fmt.Sprintf("project: %s, path: %s, committish: %s",
		m.req.GetProject(), m.req.GetPath(), m.req.GetCommittish())
}

func ListFilesRequestEq(req *gitilespb.ListFilesRequest) gomock.Matcher {
	return ListFilesRequestMatcher{req}
}
