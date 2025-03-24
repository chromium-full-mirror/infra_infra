// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type DiskCache struct {
	files       map[string]map[int]string
	maxSize     int64
	currentSize int64
	cacheDir    string
	accessTimes map[string]map[int]int64
}

func NewDiskCache(maxSize int64, cacheDir string) (*DiskCache, error) {
	err := os.MkdirAll(cacheDir, 0775)
	if err != nil {
		return nil, fmt.Errorf("error creating cache directory: %w", err)
	}

	c := &DiskCache{
		files:       make(map[string]map[int]string),
		maxSize:     maxSize,
		currentSize: 0,
		cacheDir:    cacheDir,
		accessTimes: make(map[string]map[int]int64),
	}

	err = c.scanCache()
	if err != nil {
		return nil, fmt.Errorf("error scanning cache directory: %w", err)
	}

	return c, nil
}

func (c *DiskCache) scanCache() error {
	entries, err := os.ReadDir(c.cacheDir)
	if err != nil {
		return fmt.Errorf("error reading cache directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		nameParts := strings.Split(entry.Name(), "__")
		if len(nameParts) < 2 || len(nameParts) > 3 {
			continue
		}
		buildID, err := strconv.Atoi(nameParts[0])
		if err != nil {
			continue
		}
		filename := nameParts[1]

		filePath := filepath.Join(c.cacheDir, entry.Name())
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			return fmt.Errorf("error getting file info: %w", err)
		}
		fileSize := fileInfo.Size()
		if _, exists := c.files[filename]; !exists {
			c.files[filename] = make(map[int]string)
			c.accessTimes[filename] = make(map[int]int64)
		}
		c.files[filename][buildID] = filePath
		// Set access time to the file's last modification time
		c.accessTimes[filename][buildID] = fileInfo.ModTime().Unix()
		c.currentSize += fileSize
	}
	return nil
}

func (c *DiskCache) getUniqueFileName(filename string, buildID int) (string, error) {
	cacheFilename := fmt.Sprintf("%d__%s", buildID, filename)
	targetPath := filepath.Join(c.cacheDir, cacheFilename)

	// Check for filename collisions
	for i := 1; ; i++ {
		_, err := os.Stat(targetPath)
		if os.IsNotExist(err) {
			break
		} else if err != nil {
			return "", fmt.Errorf("error checking for file collision: %w", err)
		}
		cacheFilename = fmt.Sprintf("%d__%d__%s", buildID, i, filename)
		targetPath = filepath.Join(c.cacheDir, cacheFilename)
	}
	return targetPath, nil
}

func (c *DiskCache) CreateFile(filename string, buildID int, expectedSize int64, deferClose bool) (*os.File, error) {
	// Calculate a unique cache file path.
	targetPath, err := c.getUniqueFileName(filename, buildID)
	if err != nil {
		return nil, err
	}

	// Check if a previous version exists
	if _, exists := c.files[filename]; exists {
		if prevPath, exists := c.files[filename][buildID]; exists {
			prevFileInfo, err := os.Stat(prevPath)
			if err != nil {
				return nil, fmt.Errorf("error getting previous file info: %w", err)
			}
			prevFileSize := prevFileInfo.Size()
			c.currentSize -= prevFileSize

			if err := os.Remove(prevPath); err != nil {
				return nil, fmt.Errorf("error deleting previous file: %w", err)
			}
			delete(c.files[filename], buildID)
			delete(c.accessTimes[filename], buildID)
		}
	}

	if _, exists := c.files[filename]; !exists {
		c.files[filename] = make(map[int]string)
		c.accessTimes[filename] = make(map[int]int64)
	}

	// Check if adding the new file exceeds the limit
	if c.currentSize+expectedSize > c.maxSize {
		for c.currentSize+expectedSize > c.maxSize {
			if len(c.files) == 0 {
				return nil, fmt.Errorf("cache size limit exceeded, even after deleting all files")
			}
			if err := c.deleteOldestFile(); err != nil {
				return nil, fmt.Errorf("error deleting oldest file: %w", err)
			}
			if len(c.files) == 0 {
				break
			}
		}
	}

	targetFile, err := c.createFileWithReservation(targetPath, expectedSize)
	if err != nil {
		return nil, fmt.Errorf("error creating file with reservation: %w", err)
	}
	if deferClose {
		defer targetFile.Close()
	}

	c.files[filename][buildID] = targetPath
	c.accessTimes[filename][buildID] = time.Now().Unix()
	c.currentSize += expectedSize
	return targetFile, nil
}

func (c *DiskCache) createFileWithReservation(targetPath string, expectedSize int64) (*os.File, error) {
	targetFile, err := os.Create(targetPath)
	if err != nil {
		return nil, fmt.Errorf("error creating target file: %w", err)
	}

	if err := targetFile.Truncate(expectedSize); err != nil {
		return nil, fmt.Errorf("error truncating file: %w", err)
	}
	return targetFile, nil
}

func (c *DiskCache) AddFile(filename string, buildID int, filePath string) error {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("error getting file info: %w", err)
	}
	fileSize := fileInfo.Size()

	// Calculate a unique cache file path.
	targetPath, err := c.getUniqueFileName(filename, buildID)
	if err != nil {
		return err
	}

	// Check if a previous version exists
	if _, exists := c.files[filename]; exists {
		if prevPath, exists := c.files[filename][buildID]; exists {
			prevFileInfo, err := os.Stat(prevPath)
			if err != nil {
				return fmt.Errorf("error getting previous file info: %w", err)
			}
			prevFileSize := prevFileInfo.Size()
			c.currentSize -= prevFileSize

			if err := os.Remove(prevPath); err != nil {
				return fmt.Errorf("error deleting previous file: %w", err)
			}
			delete(c.files[filename], buildID)
			delete(c.accessTimes[filename], buildID)

		}
	}

	if _, exists := c.files[filename]; !exists {
		c.files[filename] = make(map[int]string)
		c.accessTimes[filename] = make(map[int]int64)
	}

	// Check if adding the new file exceeds the limit
	if c.currentSize+fileSize > c.maxSize {
		for c.currentSize+fileSize > c.maxSize {
			if len(c.files) == 0 {
				return fmt.Errorf("cache size limit exceeded, even after deleting all files")
			}
			if err := c.deleteOldestFile(); err != nil {
				return fmt.Errorf("error deleting oldest file: %w", err)
			}
			if len(c.files) == 0 {
				break
			}
		}
	}

	err = c.moveFile(filePath, targetPath)
	if err != nil {
		return fmt.Errorf("error moving file to cache: %w", err)
	}

	c.files[filename][buildID] = targetPath
	c.accessTimes[filename][buildID] = time.Now().Unix()
	c.currentSize += fileSize
	return nil
}

func (c *DiskCache) moveFile(sourcePath string, targetPath string) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("error opening source file: %w", err)
	}
	defer sourceFile.Close()

	targetFile, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("error creating target file: %w", err)
	}
	defer targetFile.Close()

	_, err = io.Copy(targetFile, sourceFile)
	if err != nil {
		return fmt.Errorf("error copying file: %w", err)
	}
	sourceFile.Close()

	err = os.Remove(sourcePath)
	if err != nil {
		return fmt.Errorf("error removing source file: %w", err)
	}

	return nil
}

func (c *DiskCache) deleteOldestFile() error {
	var allFiles []fileKey
	for filename, buildMap := range c.files {
		for buildID := range buildMap {
			allFiles = append(allFiles, fileKey{
				filename:   filename,
				buildID:    buildID,
				accessTime: c.accessTimes[filename][buildID],
			})
		}
	}

	if len(allFiles) == 0 {
		return nil
	}

	sort.Slice(allFiles, func(i, j int) bool {
		return allFiles[i].accessTime < allFiles[j].accessTime
	})

	oldestFile := allFiles[0]

	filePath := c.files[oldestFile.filename][oldestFile.buildID]
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("error getting file info: %w", err)
	}
	fileSize := fileInfo.Size()
	c.currentSize -= fileSize

	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("error deleting file: %w", err)
	}

	delete(c.files[oldestFile.filename], oldestFile.buildID)
	delete(c.accessTimes[oldestFile.filename], oldestFile.buildID)

	// if the map for the file is now empty, delete it
	if len(c.files[oldestFile.filename]) == 0 {
		delete(c.files, oldestFile.filename)
		delete(c.accessTimes, oldestFile.filename)
	}

	return nil
}

type fileKey struct {
	filename   string
	buildID    int
	accessTime int64
}

func (c *DiskCache) GetFile(filename string, buildID int) (string, bool) {
	if _, exists := c.files[filename]; !exists {
		return "", false
	}
	filePath, exists := c.files[filename][buildID]
	if exists {
		c.accessTimes[filename][buildID] = time.Now().Unix()
	}
	return filePath, exists
}

func (c *DiskCache) GetLatestFile(filename string) (string, bool) {
	if _, exists := c.files[filename]; !exists {
		return "", false
	}

	var latestBuildID int
	var latestFilePath string
	for buildID, filePath := range c.files[filename] {
		if buildID >= latestBuildID {
			latestBuildID = buildID
			latestFilePath = filePath
		}
	}
	if latestFilePath != "" {
		c.accessTimes[filename][latestBuildID] = time.Now().Unix()
	}
	return latestFilePath, latestFilePath != ""
}
