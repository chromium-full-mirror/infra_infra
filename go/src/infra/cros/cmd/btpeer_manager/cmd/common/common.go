// Copyright 2023 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package common provides prompts functions for btpeer_manager commands.
package common

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"infra/cros/cmd/btpeer_manager/fileutils"
)

func PromptYesNo(ctx context.Context, message string, defaultAnswer bool) (bool, error) {
	if defaultAnswer {
		message += " (Y/n):"
	} else {
		message += " (y/N):"
	}
	response, err := ContextualPrompt(ctx, message)
	if err != nil {
		return defaultAnswer, err
	}
	response = strings.TrimSpace(response)
	if response == "" {
		return defaultAnswer, nil
	}
	response = strings.ToLower(response)
	return response == "y", nil
}

func PromptForString(ctx context.Context, message string, defaultAnswer string) (string, error) {
	message = fmt.Sprintf("%s (%q):", message, defaultAnswer)
	response, err := ContextualPrompt(ctx, message)
	if err != nil {
		return "", err
	}
	response = strings.TrimSpace(response)
	if response == "" {
		return defaultAnswer, nil
	}
	return response, nil
}

func ContextualPrompt(ctx context.Context, message string) (string, error) {
	fmt.Printf("\n%s ", message)
	inputReader := bufio.NewReader(fileutils.NewContextualReaderWrapper(ctx, os.Stdin))
	input, err := inputReader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read user input for prompt: %w", err)
	}
	fmt.Println()
	return input, nil
}
