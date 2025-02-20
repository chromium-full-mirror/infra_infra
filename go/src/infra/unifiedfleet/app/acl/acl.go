// Copyright 2022 The Chromium OS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package acl

import (
	"regexp"

	"go.chromium.org/infra/unifiedfleet/app/config"
)

// ACL contains a regular expression and mdb groups to map
type ACL struct {
	Match  *regexp.Regexp
	Groups []string
}

// acls is the ACL for the process
var acls []*ACL

// Register registers all the acls from the config
func Register(cfg *config.Config) {
	for _, acl := range cfg.Acls {
		add(acl.Match, acl.Groups)
	}
}

// add adds the given ACL to acls
func add(path string, groups []string) {
	r := regexp.MustCompile(path)
	acls = append(acls, &ACL{
		Match:  r,
		Groups: groups,
	})
}

// Resolve takes a path to match and returns a slice of strings containing the groups
func Resolve(path string) []string {
	var groups []string
	for _, a := range acls {
		if a.Match.MatchString(path) {
			groups = a.Groups
		}
	}
	return groups
}
