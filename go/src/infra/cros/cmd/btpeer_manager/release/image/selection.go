// Copyright 2024 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package image

import (
	"fmt"
	"strings"

	labapi "go.chromium.org/chromiumos/config/go/test/lab/api"
)

// SelectBtpeerImageByUUID returns the first image in config.Images with a matching
// image UUID. Returns non-nil error if no matching images found.
func SelectBtpeerImageByUUID(config *labapi.RaspiosCrosBtpeerImageConfig, imageUUID string) (*labapi.RaspiosCrosBtpeerImageConfig_OSImage, error) {
	if len(config.GetImages()) < 1 {
		return nil, fmt.Errorf("select image by UUID: no images defined in config")
	}
	for _, imageConfig := range config.GetImages() {
		if imageConfig.GetUuid() == imageUUID {
			return imageConfig, nil
		}
	}
	return nil, fmt.Errorf("select image by UUID: no image defined in config with UUID %q", imageUUID)
}

// SelectCurrentBtpeerImage returns the current image, as defined by
// config.CurrentImageUuid. Returns a non-nil error if no matching images were
// found or if config.CurrentImageUuid is empty.
func SelectCurrentBtpeerImage(config *labapi.RaspiosCrosBtpeerImageConfig) (*labapi.RaspiosCrosBtpeerImageConfig_OSImage, error) {
	if config.GetCurrentImageUuid() == "" {
		return nil, fmt.Errorf("select current btpeer image: no current image set")
	}
	imageConfig, err := SelectBtpeerImageByUUID(config, config.CurrentImageUuid)
	if err != nil {
		return nil, fmt.Errorf("select current btpeer image: %w", err)
	}
	return imageConfig, nil
}

// SelectNextBtpeerImage returns the next image, as defined by
// config.NextImageUuid. If config.NextImageUuid is empty, the current image is
// selected instead via SelectCurrentBtpeerImage. Returns a non-nil error if no
// matching images were found.
func SelectNextBtpeerImage(config *labapi.RaspiosCrosBtpeerImageConfig) (*labapi.RaspiosCrosBtpeerImageConfig_OSImage, error) {
	if config.GetNextImageUuid() == "" {
		// Fallback to current.
		imageConfig, err := SelectCurrentBtpeerImage(config)
		if err != nil {
			return nil, fmt.Errorf("select current btpeer image: no next image set and failed to fallback to current image: %w", err)
		}
		return imageConfig, nil
	}
	imageConfig, err := SelectBtpeerImageByUUID(config, config.NextImageUuid)
	if err != nil {
		return nil, fmt.Errorf("select next btpeer image: %w", err)
	}
	return imageConfig, nil
}

// SelectBtpeerImageForDut returns the next image if the dutHostname is in the
// config.NextImageVerificationDutPool (see SelectNextBtpeerImage), or the
// current image if not (see SelectCurrentBtpeerImage).
func SelectBtpeerImageForDut(config *labapi.RaspiosCrosBtpeerImageConfig, dutHostname string) (*labapi.RaspiosCrosBtpeerImageConfig_OSImage, error) {
	useNext := false
	for _, nextDutHostname := range config.GetNextImageVerificationDutPool() {
		if strings.EqualFold(nextDutHostname, dutHostname) {
			useNext = true
			break
		}
	}
	var imageConfig *labapi.RaspiosCrosBtpeerImageConfig_OSImage
	var err error
	if useNext {
		imageConfig, err = SelectNextBtpeerImage(config)
	} else {
		imageConfig, err = SelectCurrentBtpeerImage(config)
	}
	if err != nil {
		return nil, fmt.Errorf("select btpeer image for dut %q: %w", dutHostname, err)
	}
	return imageConfig, nil
}
