// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"
	"google.golang.org/protobuf/encoding/protojson"

	"go.chromium.org/chromiumos/config/go/build/api"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
)

var ErrBucketNotExist = errors.New("bucket does not exist")
var ErrObjectNotExist = errors.New("object does not exist")

var execLock sync.Mutex

// actionTimeout is the timeout that pertains to reading or writing
// a single file from/to GCS.
const actionTimeout = 60 * time.Second

// Storage metadata for remote file
type GSObject struct {
	Bucket string
	Object string
}

// cleanObjectPath removes the first slash in a path if any. The built-in path
// framework concerns itself with removing trailing only, and as such we do so
// bespoke.
func cleanObjectPath(objectPath string) string {
	if strings.HasPrefix(objectPath, "/") {
		return objectPath[1:]
	}
	return objectPath
}

func ParseGSURL(gsURL string) (GSObject, error) {
	if !strings.HasPrefix(gsURL, "gs://") {
		return GSObject{}, fmt.Errorf("gs url must begin with 'gs://', instead have, %s", gsURL)
	}

	u, err := url.Parse(gsURL)
	if err != nil {
		return GSObject{}, fmt.Errorf("unable to parse url, %w", err)
	}

	// Host corresponds to bucket
	// Path corresponds to object (though we need to remove prepending '/')
	return GSObject{
		Bucket: u.Host,
		Object: cleanObjectPath(u.Path),
	}, nil
}

func GetURLPath(gsURL string) (string, error) {
	if !strings.HasPrefix(gsURL, "gs://") {
		return "", fmt.Errorf("gs url must begin with 'gs://', instead have, %s", gsURL)
	}

	return gsURL[len("gs://"):], nil
}

// DownloadFile downloads a file from a designated gsURL to a given
// path on the local file system. If the bucket does not exist,
// returns ErrBucketNotExist. If the object does not exist,
// returns ErrObjectNotExist.
func DownloadFile(ctx context.Context, client *storage.Client, gsURL, destLocalPath string) error {
	logging.Infof(ctx, "Starting download of %s to %s", gsURL, destLocalPath)
	object, err := ParseGSURL(gsURL)
	if err != nil {
		return fmt.Errorf("unable to parse gs url, %w", err)
	}
	logging.Infof(ctx, "Parsed URL: %s", object)
	err = Read(ctx, client, object, destLocalPath)
	if errors.Is(err, ErrBucketNotExist) {
		return ErrBucketNotExist
	}
	if err == ErrObjectNotExist {
		return ErrObjectNotExist
	}
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}
	return nil
}

func NewStorageClientWithDefaultAccount(ctx context.Context, clientOpts ...option.ClientOption) (*storage.Client, error) {
	client, err := storage.NewClient(ctx, clientOpts...)
	if err != nil {
		return nil, err
	}
	return client, nil
}

// Read downloads a file from GCS to the given local path. If the bucket does
// not exist in GCS, the method returns ErrBucketNotExist. If the object does
// not exist in GCS, the method returns ErrObjectNotExist.
func Read(ctx context.Context, client *storage.Client, gsObject GSObject, destFilePath string) (retErr error) {
	ctx, cancel := context.WithTimeout(ctx, actionTimeout)
	defer cancel()

	reader, err := client.Bucket(gsObject.Bucket).Object(gsObject.Object).NewReader(ctx)
	if errors.Is(err, storage.ErrBucketNotExist) {
		return ErrBucketNotExist
	}
	if errors.Is(err, storage.ErrObjectNotExist) {
		return ErrObjectNotExist
	}
	if err != nil {
		return fmt.Errorf("opening reader: %w", err)
	}
	defer func() {
		if err := reader.Close(); err != nil && retErr == nil {
			retErr = err
		}
	}()

	dst, err := os.OpenFile(destFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("opening destination file %q: %w", destFilePath, err)
	}
	defer func() {
		if err := dst.Close(); err != nil && retErr == nil {
			retErr = err
		}
	}()

	if _, err := io.Copy(dst, reader); err != nil {
		return fmt.Errorf("copying to local file: %w", err)
	}
	return nil
}

// FetchImageData fetches container image metadata from provided gcs path
func FetchImageData(ctx context.Context, board string, gcsPath string) (map[string]*api.ContainerImageInfo, error) {
	if !strings.HasSuffix(gcsPath, ContainerMetadataPath) {
		gcsPath = gcsPath + ContainerMetadataPath
		logging.Infof(ctx, "container metadata path created: %s", gcsPath)
	}

	// fix for b/375974548. Parallel execution call this fucntion, causing bot to get stuck and eventually die.
	execLock.Lock()
	cat := exec.CommandContext(ctx, "gsutil", "cat", gcsPath)

	catOut, err := cat.Output()
	execLock.Unlock()
	if err != nil {
		logging.Infof(ctx, "error while downloading container metadata: %s", err)
		return nil, errors.Annotate(err, "error while downloading container metadata: ").Err()
	}

	metadata := &api.ContainerMetadata{}
	err = protojson.Unmarshal(catOut, metadata)
	if err != nil {
		logging.Infof(ctx, "error while downloading metadata: %s", err)
		return nil, errors.Annotate(err, "unable to unmarshal metadata: ").Err()
	}

	var imagesMap *api.ContainerImageMap

	if len(metadata.Containers) != 1 {
		logging.Infof(ctx, "Unexpected none or multiple conatiner maps in same metadata")
		return nil, fmt.Errorf("unexpected none or multiple conatiner maps in same metadata")

	}
	// Always grab the first map for the list, as the key lookup is unreliable. (b/331283423)
	for _, IM := range metadata.Containers {
		imagesMap = IM
	}

	Containers := make(map[string]*api.ContainerImageInfo)
	for _, image := range imagesMap.GetImages() {
		Containers[image.Name] = &api.ContainerImageInfo{
			Digest:     image.Digest,
			Repository: image.Repository,
			Name:       image.Name,
			Tags:       image.Tags,
		}
	}

	return Containers, nil
}

// DownloadGcsFileToLocal downloads gcs file to local if it doesn't exist.
func DownloadGcsFileToLocal(ctx context.Context, gcsPath string, tempRootDir string) (string, error) {
	urlPath, err := GetURLPath(gcsPath)
	if err != nil {
		logging.Infof(ctx, "error while gettting url path: %s", err)
		return "", err
	}
	localFilePath := path.Join(tempRootDir, urlPath)
	if err := DownloadGcsFileAsLocalFile(ctx, gcsPath, localFilePath); err != nil {
		return "", err
	}
	return localFilePath, nil
}

// DownloadGcsFileAsLocalFile downloads gcs file as specific local file if it doesn't exist.
func DownloadGcsFileAsLocalFile(ctx context.Context, gcsPath string, localFilePath string, clientOpts ...option.ClientOption) error {
	client, err := NewStorageClientWithDefaultAccount(ctx, clientOpts...)
	if err != nil {
		logging.Infof(ctx, "error while creating new storage client: %s", err)
		return err
	}
	if localFilePath == "" {
		return errors.Reason("localFilePath is not defined").Err()
	}
	if err := os.MkdirAll(filepath.Dir(localFilePath), os.ModePerm); err != nil {
		logging.Infof(ctx, "error while making local dir: %s", err)
		return err
	}
	logging.Infof(ctx, "Download gcs file %q as %q", gcsPath, localFilePath)
	if CheckIfFileExists(localFilePath) != nil {
		err = DownloadFile(ctx, client, gcsPath, localFilePath)
		if err != nil {
			logging.Infof(ctx, "error while downloading file: %s", err)
			return err
		}
	} else {
		logging.Infof(ctx, "local gcs file already exists")
	}
	return nil
}

// GetMajorBuildFromGCSPath parses the major build from gcs path
func GetMajorBuildFromGCSPath(gcsPath string) string {
	if gcsPath == "" {
		return ""
	}
	g := strings.Split(gcsPath, "/")
	R := g[len(g)-1]
	Major := strings.Split(R, "-")
	if len(Major) <= 2 {
		return R
	}

	return strings.Join(Major[:2], "-")
}

func IsAndroidUrl(gsURL string) bool {
	return strings.HasPrefix(gsURL, "android-build")
}
