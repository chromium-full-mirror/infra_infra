// Copyright 2024 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package image provides commands for managing release images on btpeers.
package image

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"
	"google.golang.org/protobuf/encoding/protojson"

	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"

	"go.chromium.org/infra/cros/cmd/btpeer_manager/fileutils"
	"go.chromium.org/infra/cros/cmd/btpeer_manager/log"
)

const (
	ChromeOSConnectivityTestArtifactsStorageBucket = "chromeos-connectivity-test-artifacts"
	baseStorageDir                                 = "btpeer"
	imageDirStoragePath                            = "raspios-cros-btpeer"
	configStoragePathProd                          = "raspios_cros_btpeer_image_config_prod.json"
	configStoragePathTest                          = "raspios_cros_btpeer_image_config_test.json"
)

var sharedManagerInstance *Manager

type ManagerConfig struct {
	StorageBucketName      string
	UseProdConfig          bool
	GCSCredentialsFilePath string
}

type Manager struct {
	useProdConfig bool
	gcsBucket     *storage.BucketHandle
	config        *labapi.RaspiosCrosBtpeerImageConfig
}

func SharedManagerInstance() *Manager {
	if sharedManagerInstance == nil {
		panic("sharedManagerInstance is nil")
	}
	return sharedManagerInstance
}

func SetSharedManagerInstance(m *Manager) {
	sharedManagerInstance = m
}

func NewManager(ctx context.Context, managerConfig *ManagerConfig) (*Manager, error) {
	m := &Manager{
		useProdConfig: managerConfig.UseProdConfig,
	}
	var err error
	var gcsClient *storage.Client
	if managerConfig.GCSCredentialsFilePath == "" {
		log.Logger.Println("Creating new GCS client using application-default credentials")
		gcsClient, err = storage.NewClient(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to create new GCS client using application default credentials (you need to run 'gcloud auth application-default login' first or use a cred file): %w", err)
		}
	} else {
		log.Logger.Printf("Creating new GCS client using credentials file %q\n", managerConfig.GCSCredentialsFilePath)
		gcsClient, err = storage.NewClient(ctx, option.WithCredentialsFile(managerConfig.GCSCredentialsFilePath))
		if err != nil {
			return nil, fmt.Errorf("failed to create new GCS client with cred file %q: %w", managerConfig.GCSCredentialsFilePath, err)
		}
	}
	m.gcsBucket = gcsClient.Bucket(managerConfig.StorageBucketName)
	return m, nil
}

func (m *Manager) InitializeNewConfig(ctx context.Context, forceOverwriteExisting bool) error {
	if !forceOverwriteExisting {
		configObjectPath, _ := m.ConfigObject()
		if err := m.FetchConfig(ctx); err != nil && !errors.Is(err, storage.ErrObjectNotExist) {
			return fmt.Errorf("failed to check if a config already exists in GCS at %q: %w", configObjectPath, err)
		}
		if m.config != nil {
			return fmt.Errorf("a config already exists in GCS at %q and forceOverwriteExisting is false", configObjectPath)
		}
	}
	m.config = &labapi.RaspiosCrosBtpeerImageConfig{}
	return m.saveConfig(ctx)
}

func (m *Manager) imageFileObject(imageUUID, fileName string) (string, *storage.ObjectHandle) {
	objName := m.objectName(fmt.Sprintf("%s/%s/%s", imageDirStoragePath, imageUUID, fileName))
	obj := m.gcsBucket.Object(objName)
	return m.fullObjectPath(obj), obj
}

func (m *Manager) ConfigObject() (string, *storage.ObjectHandle) {
	var configStoragePath string
	if m.useProdConfig {
		configStoragePath = configStoragePathProd
	} else {
		configStoragePath = configStoragePathTest
	}
	configObj := m.gcsBucket.Object(m.objectName(configStoragePath))
	return m.fullObjectPath(configObj), configObj
}

func (m *Manager) Config() *labapi.RaspiosCrosBtpeerImageConfig {
	return m.config
}

func (m *Manager) fullObjectPath(obj *storage.ObjectHandle) string {
	return fmt.Sprintf("gs://%s/%s", obj.BucketName(), obj.ObjectName())
}

func (m *Manager) objectName(relativePath string) string {
	return fmt.Sprintf("%s/%s", baseStorageDir, relativePath)
}

func (m *Manager) localObject(fullObjectPath string) (*storage.ObjectHandle, error) {
	baseStorageDirPath := m.fullObjectPath(m.gcsBucket.Object(m.objectName("")))
	if fullObjectPath == baseStorageDirPath {
		return nil, fmt.Errorf("invalid object path %q", fullObjectPath)
	}
	if !strings.HasPrefix(fullObjectPath, baseStorageDirPath) {
		return nil, fmt.Errorf("object path %q does not reside in the project storage dir %q", fullObjectPath, baseStorageDirPath)
	}
	relativePath := strings.TrimPrefix(fullObjectPath, baseStorageDirPath)
	return m.gcsBucket.Object(m.objectName(relativePath)), nil
}

func (m *Manager) FetchConfig(ctx context.Context) error {
	configObjectPath, configObj := m.ConfigObject()
	reader, err := configObj.NewReader(ctx)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			return err
		}
		return fmt.Errorf("failed to open config object %q from GCS: %w", configObjectPath, err)
	}
	configJSON, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("failed to read config object %q from GCS: %w", configObjectPath, err)
	}
	if err := reader.Close(); err != nil {
		return fmt.Errorf("failed to read config object %q from GCS: %w", configObjectPath, err)
	}
	config := &labapi.RaspiosCrosBtpeerImageConfig{}
	if err := protojson.Unmarshal(configJSON, config); err != nil {
		return fmt.Errorf("failed to unmarshal config object %q from GCS as a RaspiosCrosBtpeerImageConfig: %w", configObjectPath, err)
	}
	log.Logger.Printf("Successfully retrieved RaspiosCrosBtpeerImageConfig from %q\n", configObjectPath)
	m.config = config
	return nil
}

func (m *Manager) saveConfig(ctx context.Context) error {
	configObjectPath, configObj := m.ConfigObject()
	configJSON, err := fileutils.MarshalPrettyJSON(m.config)
	if err != nil {
		return fmt.Errorf("failed to marshal config to JSON: %w", err)
	}
	writer := configObj.NewWriter(ctx)
	if _, err := writer.Write(configJSON); err != nil {
		return fmt.Errorf("failed to upload new config object %q to GCS: %w", configObjectPath, err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to upload new config object %q to GCS: %w", configObjectPath, err)
	}
	log.Logger.Printf("Saved new RaspiosCrosBtpeerImageConfig to %q:\n%s\n", configObjectPath, configJSON)
	return nil
}

func (m *Manager) UpdateConfig(ctx context.Context, config *labapi.RaspiosCrosBtpeerImageConfig, forceUpdate bool) (*labapi.RaspiosCrosBtpeerImageConfig, error) {
	if err := m.ValidateConfig(config); err != nil {
		err = fmt.Errorf("invalid config: %w", err)
		if forceUpdate {
			log.Logger.Printf("config validation failed, but continuing anyways as forceUpdate is true: %v\n", err)
		} else {
			return nil, err
		}
	}
	m.config = config
	if err := m.saveConfig(ctx); err != nil {
		return nil, fmt.Errorf("failed to update config in storage: %w", err)
	}
	return m.Config(), nil
}

func (m *Manager) ValidateConfig(config *labapi.RaspiosCrosBtpeerImageConfig) error {
	if config == nil {
		return fmt.Errorf("config is nil")
	}
	imageIDs := make(map[string]bool)
	for i, image := range config.GetImages() {
		if image.Uuid == "" {
			return fmt.Errorf("Images[%d].Uuid must not be empty", i)
		}
		if image.Path == "" {
			return fmt.Errorf("Images[%d].Path must not be empty", i)
		}
		if imageIDs[image.Uuid] {
			return fmt.Errorf("Images[%d].Uuid %q is not unique", i, image.Uuid)
		}
		imageIDs[image.Uuid] = true
	}
	if config.GetCurrentImageUuid() != "" && !imageIDs[config.CurrentImageUuid] {
		return fmt.Errorf("CurrentImageUuid is set to %q, but there is no matching image configured", config.CurrentImageUuid)
	}
	if config.GetNextImageUuid() != "" && !imageIDs[config.NextImageUuid] {
		return fmt.Errorf("NextImageUuid is set to %q, but there is no matching image configured", config.NextImageUuid)
	}
	return nil
}

// UploadImage will upload an image file located at imageFilePath and its *.info
// and raspios_cros_btpeer_build_info.json files expected to be in the same
// pi-gen-btpeer deploy dir like so:
//
// pi-gen-btpeer/deploy/
//
//	<base_image_filename>.info
//	image_<base_image_filename>.img(.xz|.gz)?
//	raspios_cros_btpeer_build_info.json
func (m *Manager) UploadImage(ctx context.Context, imageFilePath string) (*labapi.RaspiosCrosBtpeerImageConfig_OSImage, error) {
	log.Logger.Println("Preparing to upload image, build info, and package info files")

	// Identify image file.
	exists, err := fileutils.PathExists(imageFilePath)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("no image file found at %q", imageFilePath)
	}
	log.Logger.Printf("Identified image file as %q\n", imageFilePath)

	// Identify build info file.
	const buildInfoFilename = "raspios_cros_btpeer_build_info.json"
	deployDir := path.Dir(imageFilePath)
	buildInfoFilePath := path.Join(deployDir, buildInfoFilename)
	exists, err = fileutils.PathExists(buildInfoFilePath)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("no image build info file found at %q", buildInfoFilePath)
	}
	log.Logger.Printf("Identified build info file as %q\n", buildInfoFilePath)

	// Identify package info file.
	baseImageFilename := strings.TrimPrefix(strings.Split(path.Base(imageFilePath), ".img")[0], "image_")
	packageInfoFilePath := path.Join(deployDir, baseImageFilename+".info")
	exists, err = fileutils.PathExists(packageInfoFilePath)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("no package info file found at %q", packageInfoFilePath)
	}
	log.Logger.Printf("Identified package info file as %q\n", packageInfoFilePath)

	// Read image UUID from build info file (also validates file format).
	log.Logger.Printf("Reading build info file %q\n", buildInfoFilePath)
	buildInfoFileContents, err := fileutils.ReadFile(ctx, buildInfoFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read build info file %q: %w", buildInfoFilePath, err)
	}
	buildInfo := &labapi.RaspiosCrosBtpeerImageBuildInfo{}
	if err := protojson.Unmarshal(buildInfoFileContents, buildInfo); err != nil {
		return nil, fmt.Errorf("failed to unmarshall build info file %q as a RaspiosCrosBtpeerImageBuildInfo object: %w", buildInfoFilePath, err)
	}
	marshalledJSON, err := fileutils.MarshalPrettyJSON(buildInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to remashall build info file %q as a RaspiosCrosBtpeerImageBuildInfo object: %w", buildInfoFilePath, err)
	}
	log.Logger.Printf("Successfully unmarshalled build info file %q as RaspiosCrosBtpeerImageBuildInfo: %s\n", buildInfoFilePath, string(marshalledJSON))
	if buildInfo.ImageUuid == "" {
		return nil, fmt.Errorf("build info file %q missing required ImageUuid: RaspiosCrosBtpeerImageBuildInfo%s", buildInfoFilePath, string(marshalledJSON))
	}
	log.Logger.Printf("Identified ImageUuid as %q\n", buildInfo.ImageUuid)

	// Upload files to GCS.
	imageObjPath, imageObjHandle := m.imageFileObject(buildInfo.ImageUuid, filepath.Base(imageFilePath))
	if err := m.uploadFile(ctx, imageFilePath, imageObjPath, imageObjHandle); err != nil {
		return nil, fmt.Errorf("upload image: %w", err)
	}
	buildInfoObjPath, buildInfoObjHandle := m.imageFileObject(buildInfo.ImageUuid, filepath.Base(buildInfoFilePath))
	if err := m.uploadFile(ctx, buildInfoFilePath, buildInfoObjPath, buildInfoObjHandle); err != nil {
		return nil, fmt.Errorf("upload image: %w", err)
	}
	packageInfoObjPath, packageInfoObjHandle := m.imageFileObject(buildInfo.ImageUuid, filepath.Base(packageInfoFilePath))
	if err := m.uploadFile(ctx, packageInfoFilePath, packageInfoObjPath, packageInfoObjHandle); err != nil {
		return nil, fmt.Errorf("upload image: %w", err)
	}

	// Return populated image config.
	return &labapi.RaspiosCrosBtpeerImageConfig_OSImage{
		Uuid: buildInfo.ImageUuid,
		Path: imageObjPath,
	}, nil
}

func (m *Manager) uploadFile(ctx context.Context, srcFilePath, objPath string, objHandle *storage.ObjectHandle) error {
	log.Logger.Printf("Uploading file from %q to %q", srcFilePath, objPath)
	inFile, err := os.Open(srcFilePath)
	if err != nil {
		return fmt.Errorf("failed to open file at %q: %w", srcFilePath, err)
	}
	inFileReader := fileutils.NewContextualReaderWrapper(ctx, inFile)
	storageWriter := objHandle.NewWriter(ctx)
	bytesWritten, err := io.Copy(storageWriter, inFileReader)
	if err != nil {
		_ = storageWriter.Close()
		_ = inFile.Close()
		return fmt.Errorf("failed to upload %q to %q: %w", srcFilePath, objPath, err)
	}
	if err := storageWriter.Close(); err != nil {
		_ = inFile.Close()
		return fmt.Errorf("failed to upload %q to %q: %w", srcFilePath, objPath, err)
	}
	_ = inFile.Close()
	log.Logger.Printf("Successfully uploaded %d bytes to %q", bytesWritten, objPath)
	return nil
}

func (m *Manager) DownloadImage(ctx context.Context, imageConfig *labapi.RaspiosCrosBtpeerImageConfig_OSImage, dstDir string) (string, error) {
	dstFilePath := filepath.Join(dstDir, filepath.Base(imageConfig.Path))
	imageObj, err := m.localObject(imageConfig.Path)
	if err != nil {
		return "", fmt.Errorf("invalid image path %q: %w", imageConfig.Path, err)
	}
	log.Logger.Printf("Downloading image %q to %q\n", imageConfig.Path, dstFilePath)
	if err := m.downloadObjectToFile(ctx, imageObj, dstFilePath); err != nil {
		return "", fmt.Errorf("failed to download image %q to %q", imageConfig.Path, dstFilePath)
	}
	absDstFilePath, err := filepath.Abs(dstFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute file path of downloaded image relative path %q: %w", dstFilePath, err)
	}
	log.Logger.Printf("Successfully downloaded image to %q\n", absDstFilePath)
	return absDstFilePath, nil
}

func (m *Manager) downloadObjectToFile(ctx context.Context, obj *storage.ObjectHandle, dstFilePath string) error {
	dstFileDir := path.Dir(dstFilePath)
	if dstFileDir != "" {
		if err := os.MkdirAll(dstFileDir, fs.FileMode(0777)); err != nil {
			return fmt.Errorf("failed to make dirs for file %q: %w", dstFilePath, err)
		}
	}
	dstFile, err := os.Create(dstFilePath)
	if err != nil {
		return fmt.Errorf("failed to create file %q: %w", dstFilePath, err)
	}
	outFileWriter := fileutils.NewContextualWriterWrapper(ctx, dstFile)
	reader, err := obj.NewReader(ctx)
	if err != nil {
		_ = reader.Close()
		_ = dstFile.Close()
		if errors.Is(err, storage.ErrObjectNotExist) {
			return err
		}
		return fmt.Errorf("failed to open GCS object: %w", err)
	}
	if _, err := io.Copy(outFileWriter, reader); err != nil {
		_ = reader.Close()
		_ = dstFile.Close()
		return fmt.Errorf("failed to download GCS object: %w", err)
	}
	if err := reader.Close(); err != nil {
		_ = dstFile.Close()
		return fmt.Errorf("failed to download GCS object: %w", err)
	}
	if err := dstFile.Close(); err != nil {
		return fmt.Errorf("failed to write to file %q: %w", dstFilePath, err)
	}
	return nil
}
