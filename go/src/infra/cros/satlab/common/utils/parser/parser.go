// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package parser

import (
	"regexp"

	"go.chromium.org/infra/cros/satlab/common/utils/errors"
)

var deployRe = regexp.MustCompile(`Follow the deploy job at (?P<URL>(?:(?:https?):\/\/)?[\w/\-?=%.]+\.[\w/\-&?=%.]+)`)

// RarseDeployURL parse the deploy URL from data.
func ParseDeployURL(s string) (string, error) {
	if !deployRe.MatchString(s) {
		return "", errors.NotMatch
	}

	matches := deployRe.FindStringSubmatch(s)
	URLIndex := deployRe.SubexpIndex("URL")

	return matches[URLIndex], nil
}
