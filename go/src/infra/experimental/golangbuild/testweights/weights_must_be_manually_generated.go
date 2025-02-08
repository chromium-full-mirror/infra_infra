// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Weights must be manually generated. Use a build tag to hide this generate
// directive from casual uses of go generate ./... .

// See https://chromium-review.googlesource.com/c/infra/infra/+/6244749 for details.

//go:build computeweights

package testweights

//go:generate go run computeweights.go
