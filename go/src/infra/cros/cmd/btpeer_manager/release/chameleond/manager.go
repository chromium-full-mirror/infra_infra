// Copyright 2023 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package chameleond provides commands for managing chameleond releases for btpeers.
package chameleond

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
	bundleDirStoragePath                           = "chameleond"
	configStoragePathProd                          = "btpeer_chameleond_config_prod.json"
	configStoragePathTest                          = "btpeer_chameleond_config_test.json"
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
	config        *labapi.BluetoothPeerChameleondConfig
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

func (m *Manager) InitializeNewBTPeerChameleondConfig(ctx context.Context, forceOverwriteExisting bool) error {
	if !forceOverwriteExisting {
		configObjectPath, _ := m.ConfigObject()
		if err := m.FetchConfig(ctx); err != nil && !errors.Is(err, storage.ErrObjectNotExist) {
			return fmt.Errorf("failed to check if a config already exists in GCS at %q: %w", configObjectPath, err)
		}
		if m.config != nil {
			return fmt.Errorf("a config already exists in GCS at %q and forceOverwriteExisting is false", configObjectPath)
		}
	}
	m.config = &labapi.BluetoothPeerChameleondConfig{}
	return m.saveConfig(ctx)
}

func (m *Manager) chameleondBundleFileObject(fileName string) (string, *storage.ObjectHandle) {
	objName := m.objectName(fmt.Sprintf("%s/%s", bundleDirStoragePath, fileName))
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

func (m *Manager) Config() *labapi.BluetoothPeerChameleondConfig {
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
	config := &labapi.BluetoothPeerChameleondConfig{}
	if err := protojson.Unmarshal(configJSON, config); err != nil {
		return fmt.Errorf("failed to unmarshal config object %q from GCS as a BluetoothPeerChameleondConfig: %w", configObjectPath, err)
	}
	log.Logger.Printf("Successfully retrieved BluetoothPeerChameleondConfig from %q\n", configObjectPath)
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
	log.Logger.Printf("Saved new BluetoothPeerChameleondConfig to %q:\n%s\n", configObjectPath, configJSON)
	return nil
}

func (m *Manager) UpdateConfig(ctx context.Context, config *labapi.BluetoothPeerChameleondConfig, forceUpdate bool) (*labapi.BluetoothPeerChameleondConfig, error) {
	if err := m.validateConfig(config); err != nil {
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

func (m *Manager) validateConfig(config *labapi.BluetoothPeerChameleondConfig) error {
	bundleCommits := make(map[string]bool)
	bundleMinDutReleaseVersions := make(map[string]bool)
	for i, bundleConfig := range config.Bundles {
		if bundleConfig.ChameleondCommit == "" {
			return fmt.Errorf("Bundles[%d].ChameleondCommit must not be empty", i)
		}
		if bundleConfig.ArchivePath == "" {
			return fmt.Errorf("Bundles[%d].ArchivePath must not be empty", i)
		}
		if bundleConfig.MinDutReleaseVersion == "" {
			return fmt.Errorf("Bundles[%d].MinDutReleaseVersion must not be empty", i)
		}
		if bundleCommits[bundleConfig.ChameleondCommit] {
			return fmt.Errorf("Bundles[%d].ChameleondCommit %q is not unique", i, bundleConfig.ChameleondCommit)
		}
		bundleCommits[bundleConfig.ChameleondCommit] = true
		if bundleMinDutReleaseVersions[bundleConfig.MinDutReleaseVersion] {
			return fmt.Errorf("Bundles[%d].MinDutReleaseVersion %q is not unique", i, bundleConfig.MinDutReleaseVersion)
		}
		bundleMinDutReleaseVersions[bundleConfig.MinDutReleaseVersion] = true
		if _, err := ParseChromeOSReleaseVersion(bundleConfig.MinDutReleaseVersion); err != nil {
			return fmt.Errorf("Bundles[%d].MinDutReleaseVersion %q is invalid: %w", i, bundleConfig.MinDutReleaseVersion, err)
		}
	}
	if !bundleMinDutReleaseVersions["0"] {
		return errors.New("no fallback bundle found with MinDutReleaseVersion \"0\"")
	}
	for i, version := range config.NextDutReleaseVersions {
		if _, err := ParseChromeOSReleaseVersion(version); err != nil {
			return fmt.Errorf("NextDutReleaseVersions[%d] %q is invalid: %w", i, version, err)
		}
	}
	if config.NextChameleondCommit != "" {
		if !bundleCommits[config.NextChameleondCommit] {
			return fmt.Errorf("NextChameleondCommit is set to %q, but there is no matching bundle configured", config.NextChameleondCommit)
		}
		if len(config.NextDutHosts) == 0 {
			log.Logger.Printf("WARNING: NextChameleondCommit is set, but NextDutHosts is empty so the next bundle will never be selected")
		}
		if len(config.NextDutReleaseVersions) == 0 {
			log.Logger.Printf("WARNING: NextChameleondCommit is set, but NextDutReleaseVersions is empty so the next bundle will never be selected")
		}
	}
	return nil
}

func (m *Manager) UploadChameleondBundle(ctx context.Context, chameleondBundleFilePath, chameleondCommit, minDutReleaseVersion string) (*labapi.BluetoothPeerChameleondConfig_ChameleondBundle, error) {
	bundleObjPath, bundleObj := m.chameleondBundleFileObject(filepath.Base(chameleondBundleFilePath))
	log.Logger.Printf("Uploading chameleond bundle from %q to %q", chameleondBundleFilePath, bundleObjPath)
	inFile, err := os.Open(chameleondBundleFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open chamelond bundle file at %q: %w", chameleondBundleFilePath, err)
	}
	inFileReader := fileutils.NewContextualReaderWrapper(ctx, inFile)
	storageWriter := bundleObj.NewWriter(ctx)
	bytesWritten, err := io.Copy(storageWriter, inFileReader)
	if err != nil {
		_ = storageWriter.Close()
		_ = inFile.Close()
		return nil, fmt.Errorf("failed to upload new chameleond bundle file to %q: %w", bundleObjPath, err)
	}
	if err := storageWriter.Close(); err != nil {
		_ = inFile.Close()
		return nil, fmt.Errorf("failed to upload new chameleond bundle file to %q: %w", bundleObjPath, err)
	}
	_ = inFile.Close()
	log.Logger.Printf("Successfully uploaded %d bytes to %q", bytesWritten, bundleObjPath)
	bundleConfig := &labapi.BluetoothPeerChameleondConfig_ChameleondBundle{
		ChameleondCommit:     chameleondCommit,
		MinDutReleaseVersion: minDutReleaseVersion,
		ArchivePath:          bundleObjPath,
	}
	return bundleConfig, nil
}

func (m *Manager) DownloadChameleondBundle(ctx context.Context, bundleConfig *labapi.BluetoothPeerChameleondConfig_ChameleondBundle, dstDir string) (string, error) {
	dstFilePath := filepath.Join(dstDir, filepath.Base(bundleConfig.ArchivePath))
	bundleObj, err := m.localObject(bundleConfig.ArchivePath)
	if err != nil {
		return "", fmt.Errorf("invalid chameleond bundle archive path %q: %w", bundleConfig.ArchivePath, err)
	}
	log.Logger.Printf("Downloading chameleond bundle archive %q\n", bundleConfig.ArchivePath)
	if err := m.downloadObjectToFile(ctx, bundleObj, dstFilePath); err != nil {
		return "", fmt.Errorf("failed to download chameleond bundle %q to %q", bundleConfig.ArchivePath, dstFilePath)
	}
	absDstFilePath, err := filepath.Abs(dstFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute file path of downloaded chameleond bundle relative path %q: %w", dstFilePath, err)
	}
	log.Logger.Printf("Successfully downloaded chameleond bundle to %q\n", absDstFilePath)
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
