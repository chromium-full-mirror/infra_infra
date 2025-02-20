// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cloudsdk

import (
	"context"
	"reflect"
	"testing"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"github.com/googleapis/gax-go/v2"

	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/libs/vmlab/api"
)

type mockImageClient struct {
	deleteFunc func() (*compute.Operation, error)
	getFunc    func() (*computepb.Image, error)
	importFunc func() (*compute.Operation, error)
	listFunc   func() *compute.ImageIterator
}

func (m *mockImageClient) Delete(context.Context, *computepb.DeleteImageRequest, ...gax.CallOption) (*compute.Operation, error) {
	return m.deleteFunc()
}

func (m *mockImageClient) Get(ctx context.Context, req *computepb.GetImageRequest, opts ...gax.CallOption) (*computepb.Image, error) {
	return m.getFunc()
}

func (m *mockImageClient) Insert(context.Context, *computepb.InsertImageRequest, ...gax.CallOption) (*compute.Operation, error) {
	return m.importFunc()
}

func (m *mockImageClient) List(ctx context.Context, req *computepb.ListImagesRequest, opts ...gax.CallOption) *compute.ImageIterator {
	return m.listFunc()
}

type mockStorageClient struct {
	existsFunc func(bucket string, object string) bool
}

func (m *mockStorageClient) Exists(ctx context.Context, bucket string, object string) bool {
	return m.existsFunc(bucket, object)
}

func TestDescribeImageError(t *testing.T) {
	imageApi := &cloudsdkImageApi{}
	client := &mockImageClient{getFunc: func() (*computepb.Image, error) {
		return nil, errors.New("404")
	}}
	gceImage := &api.GceImage{
		Name:    "my-image",
		Project: "my-project",
	}

	_, err := imageApi.describeImage(client, gceImage)

	if err == nil {
		t.Errorf("describeImage() expected error, got nil")
	}
	if gceImage.Status != api.GceImage_NOT_FOUND {
		t.Errorf("describeImage() expected status api.GceImage_NOT_FOUND, got %v", gceImage.Status)
	}
}

func TestDescribeImageReady(t *testing.T) {
	imageApi := &cloudsdkImageApi{}
	client := &mockImageClient{getFunc: func() (*computepb.Image, error) {
		status := "READY"
		i := &computepb.Image{
			Status: &status,
		}
		return i, nil
	}}
	gceImage := &api.GceImage{
		Name:    "my-image",
		Project: "my-project",
	}

	gceImage, err := imageApi.describeImage(client, gceImage)

	if err != nil {
		t.Errorf("describeImage() expected nil error, got %v", err)
	}
	if gceImage.Status != api.GceImage_READY {
		t.Errorf("describeImage() expected status api.GceImage_READY, got %v", gceImage.Status)
	}
}

func TestDescribeImagePending(t *testing.T) {
	imageApi := &cloudsdkImageApi{}
	client := &mockImageClient{getFunc: func() (*computepb.Image, error) {
		status := "PENDING"
		i := &computepb.Image{
			Status: &status,
		}
		return i, nil
	}}
	gceImage := &api.GceImage{
		Name:    "my-image",
		Project: "my-project",
	}

	gceImage, err := imageApi.describeImage(client, gceImage)

	if err != nil {
		t.Errorf("describeImage() expected nil error, got %v", err)
	}
	if gceImage.Status != api.GceImage_PENDING {
		t.Errorf("describeImage() expected status api.GceImage_PENDING, got %v", gceImage.Status)
	}
}

func TestDescribeImageNotFound(t *testing.T) {
	imageApi := &cloudsdkImageApi{}
	client := &mockImageClient{getFunc: func() (*computepb.Image, error) {
		return nil, errors.New("googleapi: Error 404: The resource 'projects/chromeos-gce-tests/global/images/staging-betty-arc-r-119-8768649798491452801-15630-0-0-58485-cq' was not found")
	}}
	gceImage := &api.GceImage{
		Name:    "my-image",
		Project: "my-project",
	}

	gceImage, err := imageApi.describeImage(client, gceImage)

	if err != nil {
		t.Errorf("describeImage() expected nil error, got %v", err)
	}
	if gceImage.Status != api.GceImage_NOT_FOUND {
		t.Errorf("describeImage() expected status api.GceImage_NOT_FOUND, got %v", gceImage.Status)
	}
}

func TestDescribeImageDeleting(t *testing.T) {
	imageApi := &cloudsdkImageApi{}
	client := &mockImageClient{getFunc: func() (*computepb.Image, error) {
		status := "DELETING"
		i := &computepb.Image{
			Status: &status,
		}
		return i, nil
	}}
	gceImage := &api.GceImage{
		Name:    "my-image",
		Project: "my-project",
	}

	gceImage, err := imageApi.describeImage(client, gceImage)

	if err != nil {
		t.Errorf("describeImage() expected nil error, got %v", err)
	}
	if gceImage.Status != api.GceImage_DELETING {
		t.Errorf("describeImage() expected status api.GceImage_DELETING, got %v", gceImage.Status)
	}
}

func TestDescribeImageFailed(t *testing.T) {
	imageApi := &cloudsdkImageApi{}
	client := &mockImageClient{getFunc: func() (*computepb.Image, error) {
		status := "FAILED"
		i := &computepb.Image{
			Status: &status,
		}
		return i, nil
	}}
	gceImage := &api.GceImage{
		Name:    "my-image",
		Project: "my-project",
	}

	gceImage, err := imageApi.describeImage(client, gceImage)

	if err != nil {
		t.Errorf("describeImage() expected nil error, got %v", err)
	}
	if gceImage.Status != api.GceImage_FAILED {
		t.Errorf("describeImage() expected status api.GceImage_FAILED, got %v", gceImage.Status)
	}
}

func TestHandleReady(t *testing.T) {
	imageApi := &cloudsdkImageApi{}
	client := &mockImageClient{getFunc: func() (*computepb.Image, error) {
		status := "READY"
		i := &computepb.Image{
			Status: &status,
		}
		return i, nil
	}}
	gceImage := &api.GceImage{
		Name:    "my-image",
		Project: "my-project",
	}

	op, gceImage, err := imageApi.handle(client, nil, gceImage)

	if op != nil {
		t.Errorf("handle() expected nil operation, got %v", op)
	}
	if err != nil {
		t.Errorf("handle() expected nil error, got %v", err)
	}
	if gceImage.Status != api.GceImage_READY {
		t.Errorf("handle() expected status api.GceImage_READY, got %v", gceImage.Status)
	}
}

func TestHandlePending(t *testing.T) {
	imageApi := &cloudsdkImageApi{}
	client := &mockImageClient{
		getFunc: func() (*computepb.Image, error) {
			status := "PENDING"
			i := &computepb.Image{
				Status: &status,
			}
			return i, nil
		},
	}
	gceImage := &api.GceImage{
		Name:    "my-image",
		Project: "my-project",
	}

	_, gceImage, err := imageApi.handle(client, nil, gceImage)

	if err != nil {
		t.Errorf("handle() expected nil error, got %v", err)
	}
	if gceImage.Status != api.GceImage_PENDING {
		t.Errorf("handle() expected status api.GceImage_PENDING, got %v", gceImage.Status)
	}
}

func TestHandleImportImage(t *testing.T) {
	imageApi := &cloudsdkImageApi{}
	expected := &compute.Operation{}
	client := &mockImageClient{
		getFunc: func() (*computepb.Image, error) {
			return nil, nil
		},
		importFunc: func() (*compute.Operation, error) {
			return expected, nil
		},
	}
	gceImage := &api.GceImage{
		Name:    "my-image",
		Project: "my-project",
	}

	op, gceImage, err := imageApi.handle(client, nil, gceImage)

	if op != expected {
		t.Errorf("handle() expected operation %v, got %v", expected, op)
	}
	if err != nil {
		t.Errorf("handle() expected nil error, got %v", err)
	}
	if gceImage.Status != api.GceImage_PENDING {
		t.Errorf("handle() expected status api.GceImage_PENDING, got %v", gceImage.Status)
	}
}

func TestHandleImportImageWithRetries(t *testing.T) {
	imageApi := &cloudsdkImageApi{
		imageQueryInitialRetryBackoff: 0,
		imageQueryMaxRetries:          3,
	}
	expected := &compute.Operation{}
	attempts := 0
	client := &mockImageClient{
		getFunc: func() (*computepb.Image, error) {
			attempts++
			return nil, errors.New("404")
		},
		importFunc: func() (*compute.Operation, error) {
			return expected, nil
		},
	}
	gceImage := &api.GceImage{
		Name:    "my-image",
		Project: "my-project",
	}

	op, gceImage, err := imageApi.handle(client, nil, gceImage)

	if op != expected {
		t.Errorf("handle() expected operation %v, got %v", expected, op)
	}
	if err != nil {
		t.Errorf("handle() expected nil error, got %v", err)
	}
	if gceImage.Status != api.GceImage_PENDING {
		t.Errorf("handle() expected status api.GceImage_PENDING, got %v", gceImage.Status)
	}
	if attempts != imageApi.imageQueryMaxRetries+1 {
		t.Errorf("Expected %d retries, got %d", imageApi.imageQueryMaxRetries, attempts-1)
	}
}

func TestHandleImportError(t *testing.T) {
	imageApi := &cloudsdkImageApi{}
	client := &mockImageClient{
		getFunc: func() (*computepb.Image, error) {
			return nil, nil
		},
		importFunc: func() (*compute.Operation, error) {
			return nil, errors.New("Outage")
		},
	}
	gceImage := &api.GceImage{
		Name:    "my-image",
		Project: "my-project",
	}

	_, gceImage, err := imageApi.handle(client, nil, gceImage)

	if err == nil {
		t.Errorf("handle() expected error, got nil")
	}
	if gceImage.Status != api.GceImage_NOT_FOUND {
		t.Errorf("handle() expected status api.GceImage_NOT_FOUND, got %v", gceImage.Status)
	}
}

func TestHandleImportConflict(t *testing.T) {
	imageApi := &cloudsdkImageApi{}
	client := &mockImageClient{
		getFunc: func() (*computepb.Image, error) {
			return nil, nil
		},
		importFunc: func() (*compute.Operation, error) {
			return nil, errors.New("googleapi: Error 409: The resource 'projects/chromeos-gce-tests/global/images/betty-arc-r-117-8773915992628062017-15565-0-0-85838-snapshot' already exists")
		},
	}
	gceImage := &api.GceImage{
		Name:    "my-image",
		Project: "my-project",
	}

	_, gceImage, err := imageApi.handle(client, nil, gceImage)

	if err != nil {
		t.Errorf("handle() expected nil error, got %v", err)
	}
	if gceImage.Status != api.GceImage_PENDING {
		t.Errorf("handle() expected status api.GceImage_PENDING, got %v", gceImage.Status)
	}
}

func TestDeleteImageNoWait(t *testing.T) {
	imageApi := &cloudsdkImageApi{}
	client := &mockImageClient{
		deleteFunc: func() (*compute.Operation, error) {
			return &compute.Operation{}, nil
		},
	}
	err := imageApi.deleteImage(client, "test", false)
	if err != nil {
		t.Errorf("listImages() unexpected error: %v", err)
	}
}

func TestDeleteImageNoWaitError(t *testing.T) {
	imageApi := &cloudsdkImageApi{}
	client := &mockImageClient{
		deleteFunc: func() (*compute.Operation, error) {
			return nil, errors.New("error")
		},
	}
	err := imageApi.deleteImage(client, "test", false)
	if err == nil {
		t.Errorf("listImages() expected error but got nil")
	}
}

func TestDeleteImageWaitNoOperationError(t *testing.T) {
	imageApi := &cloudsdkImageApi{}
	client := &mockImageClient{
		deleteFunc: func() (*compute.Operation, error) {
			return nil, nil
		},
	}
	err := imageApi.deleteImage(client, "test", true)
	if err == nil {
		t.Errorf("listImages() expected error but got nil")
	}
}

func TestParseBuildPathInvalid(t *testing.T) {
	buildPath := "R108-15164.0.0-71927-8801111609984657185"
	_, err := parseBuildPath(buildPath)
	if err == nil {
		t.Errorf("parseBuildPath() expected error, got nil")
	}
}

func TestParseBuildPathAndConvertNameCq(t *testing.T) {
	buildPath := "amd64-generic-cq/R108-15164.0.1-71927-8801111609984657185"
	info, err := parseBuildPath(buildPath)
	if err != nil {
		t.Fatalf("parseBuildPath() error: %v", err)
	}

	expectedInfo := buildInfo{
		buildType:    "cq",
		board:        "amd64-generic",
		milestone:    "108",
		majorVersion: "15164",
		minorVersion: "0",
		patchNumber:  "1",
		snapshot:     "71927",
		buildNumber:  "8801111609984657185",
	}
	if !reflect.DeepEqual(*info, expectedInfo) {
		t.Errorf("Expected build info: %s, but is actual: %s", expectedInfo, info)
	}

	actualName := getImageName(info, buildPath)
	expectedName := "amd64-generic-108-8801111609984657185-15164-0-1-71927-cq"
	if expectedName != actualName {
		t.Errorf("Expected image name: %s, but is actual: %s", expectedName, actualName)
	}
}

func TestParseBuildPathAndConvertNamePostsubmit(t *testing.T) {
	buildPath := "betty-pi-arc-postsubmit/R113-15376.0.0-79071-8787141177342104481"
	info, err := parseBuildPath(buildPath)
	if err != nil {
		t.Fatalf("parseBuildPath() error: %v", err)
	}

	expectedInfo := buildInfo{
		buildType:    "postsubmit",
		board:        "betty-pi-arc",
		milestone:    "113",
		majorVersion: "15376",
		minorVersion: "0",
		patchNumber:  "0",
		snapshot:     "79071",
		buildNumber:  "8787141177342104481",
	}
	if !reflect.DeepEqual(*info, expectedInfo) {
		t.Errorf("Expected build info: %s, but is actual: %s", expectedInfo, info)
	}

	actualName := getImageName(info, buildPath)
	expectedName := "betty-pi-arc-113-8787141177342104481-15376-0-0-79071-postsubmit"
	if expectedName != actualName {
		t.Errorf("Expected image name: %s, but is actual: %s", expectedName, actualName)
	}
}

func TestParseBuildPathAndConvertNameRelease(t *testing.T) {
	buildPath := "betty-arc-r-release/R108-15178.0.0"
	info, err := parseBuildPath(buildPath)
	if err != nil {
		t.Fatalf("parseBuildPath() error: %v", err)
	}

	expectedInfo := buildInfo{
		buildType:    "release",
		board:        "betty-arc-r",
		milestone:    "108",
		majorVersion: "15178",
		minorVersion: "0",
		patchNumber:  "0",
		snapshot:     "",
		buildNumber:  "",
	}
	if !reflect.DeepEqual(*info, expectedInfo) {
		t.Errorf("Expected build info: %s, but is actual: %s", expectedInfo, info)
	}

	actualName := getImageName(info, buildPath)
	expectedName := "betty-arc-r-108--15178-0-0--release"
	if expectedName != actualName {
		t.Errorf("Expected image name: %s, but is actual: %s", expectedName, actualName)
	}
}

func TestParseBuildPathAndConvertNameLong(t *testing.T) {
	buildPath := "betty-arc-r-postsubmit/R113-15376.99.99-79071-8787141177342104481"
	info, err := parseBuildPath(buildPath)
	if err != nil {
		t.Fatalf("parseBuildPath() error: %v", err)
	}

	expectedInfo := buildInfo{
		buildType:    "postsubmit",
		board:        "betty-arc-r",
		milestone:    "113",
		majorVersion: "15376",
		minorVersion: "99",
		patchNumber:  "99",
		snapshot:     "79071",
		buildNumber:  "8787141177342104481",
	}
	if !reflect.DeepEqual(*info, expectedInfo) {
		t.Errorf("Expected build info: %s, but is actual: %s", expectedInfo, info)
	}

	actualName := getImageName(info, buildPath)
	expectedName := "betty-arc-r-113-8787141177342104481-15376-99-99-79071-postsubmi"
	if expectedName != actualName {
		t.Errorf("Expected image name: %s, but is actual: %s", expectedName, actualName)
	}
}

func TestParseBuildPathAndConvertNameInvalid(t *testing.T) {
	buildPath := "invalid*build#{path}-for_test.gz"
	info, err := parseBuildPath(buildPath)
	if err == nil || info != nil {
		t.Fatalf("parseBuildPath() should not succeed.")
	}

	actualName := getImageName(info, buildPath)
	expectedName := "unknown-b241a61a97da858ca70e7837aa1e209e"
	if expectedName != actualName {
		t.Errorf("Expected image name: %s, but is actual: %s", expectedName, actualName)
	}
}

func TestGetImageLabelsValid(t *testing.T) {
	actual := getImageLabels(&buildInfo{
		buildType: "cq",
		board:     "betty-arc-r",
		milestone: "100",
	})
	expected := map[string]string{
		"created-by": "vmlab",
		"build-type": "cq",
		"board":      "betty-arc-r",
		"milestone":  "100",
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("Expected labels: %v, but is actual: %v", expected, actual)
	}
}

func TestGetImageLabelsInvalid(t *testing.T) {
	actual := getImageLabels(nil)
	expected := map[string]string{
		"created-by": "vmlab",
		"build-type": "unknown",
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("Expected labels: %v, but is actual: %v", expected, actual)
	}
}

func TestParseImageStatus(t *testing.T) {
	expectedResults := map[string]api.GceImage_Status{
		"UNKNOWN":  api.GceImage_UNKNOWN,
		"READY":    api.GceImage_READY,
		"PENDING":  api.GceImage_PENDING,
		"FAILED":   api.GceImage_FAILED,
		"DELETING": api.GceImage_DELETING,
	}
	for status, expected := range expectedResults {
		actual := parseImageStatus(status)
		if expected != actual {
			t.Errorf("Expected status is %s for %s, but is actual %s", expected, status, actual)
		}
	}
}

func TestGetGcsImagePathExistsNonStaging(t *testing.T) {
	expected := "https://storage.googleapis.com/chromeos-image-archive/build/chromiumos_test_image_gce.tar.gz"

	client := &mockStorageClient{existsFunc: func(bucket, object string) bool {
		return bucket == "chromeos-image-archive"
	}}
	ctx := context.Background()
	actual, err := getGcsImagePath(client, "build", ctx)

	if err != nil {
		t.Errorf("getGcsImagePath() expected nil error, got %v", err)
	}
	if actual != expected {
		t.Errorf("getGcsImagePath() expected %s, got %s", expected, actual)
	}
}

func TestGetGcsImagePathExistsStaging(t *testing.T) {
	expected := "https://storage.googleapis.com/staging-chromeos-image-archive/build/chromiumos_test_image_gce.tar.gz"

	client := &mockStorageClient{existsFunc: func(bucket, object string) bool {
		return bucket == "staging-chromeos-image-archive"
	}}
	ctx := context.Background()
	actual, err := getGcsImagePath(client, "build", ctx)

	if err != nil {
		t.Errorf("getGcsImagePath() expected nil error, got %v", err)
	}
	if actual != expected {
		t.Errorf("getGcsImagePath() expected %s, got %s", expected, actual)
	}
}

func TestGetGcsImagePathNotExists(t *testing.T) {
	client := &mockStorageClient{existsFunc: func(bucket, object string) bool {
		return false
	}}
	ctx := context.Background()
	actual, err := getGcsImagePath(client, "build", ctx)

	if err == nil {
		t.Errorf("getGcsImagePath() expected error, got nil")
	}
	if actual != "" {
		t.Errorf("getGcsImagePath() expected empty result, got %s", actual)
	}
}

func TestPollQuitOnError(t *testing.T) {
	expected := 2
	count := 1
	ctx, cancel := context.WithTimeout(context.Background(), 310*time.Millisecond)
	defer cancel()
	f := func(ctx context.Context) (bool, error) {
		count++
		if count == 2 {
			return false, errors.New("error on 2")
		}
		return false, nil
	}
	interval := 100 * time.Millisecond

	err := poll(ctx, f, interval)
	actual := count

	if expected != actual {
		t.Errorf("count expected: %v, actual: %v", expected, actual)
	}
	if err == nil {
		t.Errorf("poll() expected error, got nil")
	}
}

func TestPollQuitOnSuccess(t *testing.T) {
	expected := 3
	count := 1
	// The original value here before 2023-01-18 was 310.
	// This isn't enough of a buffer and the tree closed one time.
	// Set the buffer to 3 seconds for safety.
	ctx, cancel := context.WithTimeout(context.Background(), 3000*time.Millisecond)
	defer cancel()
	f := func(ctx context.Context) (bool, error) {
		count++
		if count == 3 {
			return true, nil
		}
		return false, nil
	}
	interval := 100 * time.Millisecond

	_ = poll(ctx, f, interval)
	actual := count

	if expected != actual {
		t.Errorf("count expected: %v, actual: %v", expected, actual)
	}
}

func TestPollNoDeadlineError(t *testing.T) {
	ctx := context.Background()
	f := func(ctx context.Context) (bool, error) {
		return false, nil
	}
	interval := time.Duration(1)

	err := poll(ctx, f, interval)

	if err == nil {
		t.Errorf("poll() expected error, got nil")
	}
}
