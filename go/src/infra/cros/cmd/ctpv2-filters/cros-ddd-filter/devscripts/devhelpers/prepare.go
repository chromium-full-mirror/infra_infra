// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package devhelpers

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/common/api/gerrit"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/hardcoded/chromeinfra"

	"go.chromium.org/infra/cros/cmd/common_lib/common"
	buildmetada "go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/datasets/buildmetadata"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/datasets/dlmmetadata"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/datasets/hwid/db"
	ttcpclassescategorygenerator "go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/ttcp_classes_category_generator"
)

const (
	ChromeosHwidPath            = "https://chrome-internal.googlesource.com/chromeos/chromeos-hwid/+archive/refs/heads/main.tar.gz"
	ClassesAndCategoriesPath    = "named_ttcp_classes_and_categories_dataset.zip"
	ClassesAndCategoriesDocPath = "named_ttcp_classes_and_categories_docs.zip"
	BuildMetadataPath           = "cros-ddd/generated/build_metadata.jsonproto"
	DlmDevicesPath              = "cros-ddd/generated/dlm_devices.json"
)

func PrepareCrosDDD(ctx context.Context, dir string, loginMode auth.LoginMode) error {
	authOpts := authorize()

	if err := fetchChromeosHwid(ctx, dir, authOpts, loginMode); err != nil {
		return err
	}

	if err := createTtcpClassesAndCategories(ctx, dir); err != nil {
		return err
	}

	return nil
}

func authorize() *auth.Options {
	opts := chromeinfra.DefaultAuthOptions()
	// Introduce gerrit scopes for accessing the config files
	gerritScopes := []string{
		gerrit.OAuthScope,
		auth.OAuthScopeEmail,
		auth.OAuthScopeIAM,
		// This scope is needed to access an internal repo. It does not mean
		// that the user is authenticated but it is the scope that is needed.
		"https://www.googleapis.com/auth/gerritcodereview",
	}
	opts.Scopes = append(opts.Scopes, gerritScopes...)

	return &opts
}

func fetchChromeosHwid(ctx context.Context, dir string, authOpts *auth.Options, loginMode auth.LoginMode) error {
	tarBytes, err := common.FetchTarFromInternalURL(ChromeosHwidPath, authOpts, loginMode)
	if err != nil {
		return fmt.Errorf("failed to fetch chromeos-hwid, %s", err)
	}

	dir = path.Join(dir, "chromeos-hwid")
	if err = os.MkdirAll(dir, common.DirPermission); err != nil {
		return err
	}

	buffer := bytes.NewBuffer(tarBytes)
	gr, err := gzip.NewReader(buffer)
	if err != nil {
		return fmt.Errorf("failed to make gzip reader, %s", err)
	}

	tr := tar.NewReader(gr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to extract chromeos-hwid, %s", err)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.Mkdir(path.Join(dir, header.Name), common.DirPermission); err != nil {
				return fmt.Errorf("failed to make directory while extracting chromeos-hwid, %s", err)
			}
		case tar.TypeReg:
			file, err := os.Create(path.Join(dir, header.Name))
			if err != nil {
				return fmt.Errorf("failed to make file while extracting chromeos-hwid, %s", err)
			}
			if _, err := io.Copy(file, tr); err != nil {
				return fmt.Errorf("failed to write to file while extracting chromeos-hwid, %s", err)
			}
			file.Close()
		default:
			logging.Infof(ctx, "unknown type: %s for %s", header.Typeflag, header.Name)
		}
	}

	return nil
}

func createTtcpClassesAndCategories(ctx context.Context, dir string) error {
	DbPaths, err := filepath.Glob(path.Join(dir, "chromeos-hwid", "v3", "*.internal"))
	if err != nil {
		return fmt.Errorf("failed to find v3 internal files, %s", err)
	}
	ttcpclassescategorygenerator.CreateTtcpClasessAndCategories(&db.HwidDbResources{
		DescriptorsPaths: DbPaths,
		ProjectIndexPath: path.Join(dir, "chromeos-hwid", "projects.yaml"),
	}, &buildmetada.BuildMetadataResources{
		Path: path.Join(dir, BuildMetadataPath),
	}, &dlmmetadata.DlmResources{
		Path: path.Join(dir, DlmDevicesPath),
	}, path.Join(dir, ClassesAndCategoriesPath), path.Join(dir, ClassesAndCategoriesDocPath))

	return nil
}
