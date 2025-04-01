// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"context"
	"fmt"
	"log"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"

	"go.chromium.org/luci/common/logging"
)

const (
	// models
	Gemini15Flash = "gemini-1.5-flash"
	Gemini20Flash = "gemini-2.0-flash"
	// CTPContext is the context for model
	CTPContext = "CTP orchestrates tests on versioned board/model OS images using hermetic test runners. It receives requests specifying test suites and target images as scheduling targets."
	// CTPTopLevelPrompt is prompt used to identify/summarize issue for CTP execution
	CTPTopLevelPrompt = "Gemini, CTP received a test request to run suites on specific boards/models OS images (defined by versions). Test runners execute these hermetic images. Given test request and failure error, summarize the issue (<100 words) and suggest a fix."
	// SuiteFailureSummaryPrompt is prompt for summarizing failure reason for one suite execution
	SuiteFailureSummaryPrompt = "Gemini, above is input request and execution summary for test suite. Explain any failure messages present in the processing of this suite_request and connect them to the details in the suite_request(<200 words)."
)

// AISummarize takes a context text and a prompt and returns a generative ai summary.
func AISummarize(ctx context.Context, context string, prompt string, apiKey string) (string, error) {
	// TODO: generate API_KEY for TSE project and store in Secrets Manager
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		log.Fatal(err)
		return "", err
	}
	defer client.Close()

	// TODO: Allow use of other and more recent models reather than just one.
	model := client.GenerativeModel(Gemini20Flash)
	resp, err := model.GenerateContent(ctx, genai.Text("context: \n"+context+"\n"+"prompt: \n"+prompt))
	if err != nil {
		log.Fatal(err)
		return "", err
	}
	response := ""

	for _, cand := range resp.Candidates {
		if cand.Content != nil {
			for _, part := range cand.Content.Parts {
				logging.Infof(ctx, "GenAI response, %s", part)
				response += fmt.Sprintf("%v", part)
			}
		}
	}
	return response, nil

}
