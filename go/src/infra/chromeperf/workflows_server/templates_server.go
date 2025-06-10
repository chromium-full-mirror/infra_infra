// Copyright 2021 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/prototext"

	configProto "go.chromium.org/luci/common/proto/config"
	"go.chromium.org/luci/config"

	"go.chromium.org/infra/chromeperf/workflows"
	"go.chromium.org/infra/chromeperf/workflows_server/proto"
)

// Configuration path we're looking for to support project-defined templates.
const workflowTemplatesFile = "workflow-templates.cfg"
const configSetName = "services/chromeperf-workflow-templates"

type workflowTemplatesServer struct {
	workflows.UnimplementedWorkflowTemplatesServer
	luciConfigClient config.Interface
}

func (*workflowTemplatesServer) ValidateConfig(ctx context.Context, req *configProto.ValidationRequestMessage) (*configProto.ValidationResponseMessage, error) {
	c := proto.WorkflowTemplatesConfig{}
	if err := prototext.Unmarshal(req.Content, &c); err != nil {
		// TODO(dberris): Provide richer error messages for debuggability.
		return nil, status.Errorf(codes.Internal, "Failed unmarshaling config; err: %v", err)
	}
	return &configProto.ValidationResponseMessage{}, nil
}

func (s *workflowTemplatesServer) ListWorkflowTemplates(ctx context.Context, req *workflows.ListWorkflowTemplatesRequest) (*workflows.ListWorkflowTemplatesResponse, error) {
	// TODO(dberris): Use a Redis cache for getting configurations?
	// Get a list of configurations.
	configs, err := s.luciConfigClient.GetConfig(ctx, configSetName, workflowTemplatesFile, false)
	if err != nil {
		// TODO(dberris): Provide richer error messages for debuggability.
		return nil, status.Errorf(codes.Internal, "Failed fetching configuration; err: %v", err)
	}
	c := proto.WorkflowTemplatesConfig{}
	if err := prototext.Unmarshal([]byte(configs.Content), &c); err != nil {
		// TODO(dberris): Provide richer error messages for debuggability.
		return nil, status.Errorf(codes.Internal, "Failed unmarshaling config; err: %v", err)
	}
	resp := &workflows.ListWorkflowTemplatesResponse{}
	resp.WorkflowTemplates = append(resp.WorkflowTemplates, c.Templates...)
	return resp, nil
}

func (s *workflowTemplatesServer) GetWorkflowTemplate(ctx context.Context, req *workflows.GetWorkflowTemplateRequest) (*workflows.WorkflowTemplate, error) {
	// TODO(dberris): Use a Redis cache ofr getting configurations?
	// Get the list of templates.
	configs, err := s.luciConfigClient.GetConfig(ctx, configSetName, workflowTemplatesFile, false)
	if err != nil {
		// TODO(dberris): Provide richer error messages for debuggability.
		return nil, status.Errorf(codes.Internal, "Failed fetching configuration; err: %v", err)
	}
	c := proto.WorkflowTemplatesConfig{}
	if err := prototext.Unmarshal([]byte(configs.Content), &c); err != nil {
		// TODO(dberris): Provide richer error messages for debuggability.
		return nil, status.Errorf(codes.Internal, "Failed unmarshaling config; err: %v", err)
	}
	for _, t := range c.Templates {
		qualName := "/workflow-template/" + t.Name
		if strings.Compare(qualName, req.Name) == 0 {
			return t, nil
		}
	}
	return nil, status.Errorf(codes.NotFound, "Template not found: %s", req.Name)
}

func main() {
}
