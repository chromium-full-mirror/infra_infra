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
	ModelGemini15Flash = "gemini-1.5-flash"
)

// AISummarize takes a context text and a prompt and returns a generative ai summary.
func AISummarize(ctx context.Context, context string, prompt string) (string, error) {
	// TODO: generate API_KEY for TSE project and store in Secrets Manager
	client, err := genai.NewClient(ctx, option.WithAPIKey("API_KEY"))
	if err != nil {
		log.Fatal(err)
		return "", err
	}
	defer client.Close()

	// TODO: Allow use of other and more recent models reather than just one.
	model := client.GenerativeModel(ModelGemini15Flash)
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
