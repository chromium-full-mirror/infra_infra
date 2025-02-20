// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package main

import (
	"context"
	"fmt"

	lgs "go.chromium.org/luci/common/gcloud/gs"

	"go.chromium.org/infra/cros/internal/gerrit"
	"go.chromium.org/infra/cros/internal/gs"
	"go.chromium.org/infra/cros/internal/manifestutil"
	"go.chromium.org/infra/cros/internal/repo"
)

const (
	chromeExternalHost                = "chromium.googlesource.com"
	chromeInternalHost                = "chrome-internal.googlesource.com"
	manifestInternalProject           = "chromeos/manifest-internal"
	internalManifestVersionsProject   = "chromeos/manifest-versions"
	externalManifestVersionsProject   = "chromiumos/manifest-versions"
	internalBuildspecsGSBucketDefault = "chromeos-manifest-versions"
	externalBuildspecsGSBucketDefault = "chromiumos-manifest-versions"
)

// A list of remotes to be used when the ToT manifest lacks them.
//
// Add remotes to this list when you remove a remote from the ToT manifest while
// old manifests without remote annotations still refer to them.
//
// See b/269194223 for an example where this hack was needed.
var fallbackRemotes = []repo.Remote{
	// weave remote was added in R47-7423.0.0 (crrev.com/i/229188), and removed in
	// R60-9526.0.0 (crrev.com/i/369389).
	{
		Name:   "weave",
		Fetch:  "https://weave.googlesource.com",
		Review: "https://weave-review.googlesource.com",
		Annotations: []repo.Annotation{
			{Name: "public", Value: "true"},
		},
	},
}

// CreateProjectBuildspec creates a public buildspec as outlined in go/per-project-buildspecs.
func createPublicBuildspec(gsClient gs.Client, gerritClient gerrit.Client, buildspec *repo.Manifest, uploadPath lgs.Path, push bool) error {
	remoteReference := buildspec
	anyAnnotations := false
	for _, remote := range buildspec.Remotes {
		if len(remote.Annotations) > 0 {
			anyAnnotations = true
			break
		}
	}

	if !anyAnnotations {
		// If annotations are missing, fall back to downloading the ToT
		// manifest and using that as reference.
		var err error
		remoteReference, err = manifestutil.LoadManifestFromGitilesWithIncludes(
			context.Background(), gerritClient, chromeInternalHost, manifestInternalProject,
			"HEAD", "default.xml")
		if err != nil {
			return err
		}

		// Add fallback remotes if they're missing in the ToT manifest.
		for _, fallbackRemote := range fallbackRemotes {
			if remoteReference.GetRemoteByName(fallbackRemote.Name) == nil {
				remoteReference.Remotes = append(remoteReference.Remotes, fallbackRemote)
			}
		}
	}

	// Look at remotes, filter out non public projects.
	publicRemote := make(map[string]bool, len(buildspec.Remotes))
	var publicRemotes []repo.Remote
	for _, remote := range buildspec.Remotes {
		referenceRemote := remoteReference.GetRemoteByName(remote.Name)
		if referenceRemote == nil {
			return fmt.Errorf("could not get public status for remote %v from reference manifest", remote.Name)
		}

		public, ok := referenceRemote.GetAnnotation("public")
		if !ok {
			return fmt.Errorf("could not get public status for remote %v from reference manifest", remote.Name)
		}
		publicRemote[remote.Name] = ok && (public == "true")
		if remoteReference != buildspec {
			remote.Annotations = referenceRemote.Annotations
		}
		if publicRemote[remote.Name] {
			publicRemotes = append(publicRemotes, remote)
		}
	}

	// Verify that the default is not a private remote.
	defaultRemote := buildspec.Default.RemoteName
	if public, ok := publicRemote[defaultRemote]; !(ok && public) {
		return fmt.Errorf("default remote is private")
	}

	var publicProjects []repo.Project
	for _, project := range buildspec.Projects {
		// Check for the (implicit) default remote or a known public remote.
		if public, ok := publicRemote[project.RemoteName]; project.RemoteName == "" || (ok && public) {
			publicProjects = append(publicProjects, project)
		}
	}
	buildspec.Remotes = publicRemotes
	buildspec.Projects = publicProjects

	// Upload to external buildspec dir.
	if !push {
		LogOut("Dry run, not uploading buildspec to %s...", string(uploadPath))
		return nil
	}
	if err := WriteManifestToGS(gsClient, uploadPath, buildspec); err != nil {
		return err
	}
	LogOut("Uploaded buildspec to %s", string(uploadPath))
	return nil
}

func WriteManifestToGS(gsClient gs.Client, uploadPath lgs.Path, manifest *repo.Manifest) error {
	manifestData, err := manifest.WriteToBytes()
	if err != nil {
		return err
	}
	return gsClient.WriteFileToGS(uploadPath, manifestData)
}
