// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"log"
	"os"
	"path"

	"go.chromium.org/luci/auth"

	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/devscripts/devhelpers"
)

func main() {
	currentPath, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}
	localDevPath := path.Join(path.Dir(currentPath), "local")
	datasetsPath := path.Join(path.Dir(currentPath), "datasets")
	log.Println(localDevPath)
	log.Println(datasetsPath)

	err = devhelpers.PrepareCrosDDD(context.Background(), localDevPath, auth.InteractiveLogin)
	if err != nil {
		log.Fatal(err)
	}
}
