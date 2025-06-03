// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package builders

import (
	"strings"

	"go.chromium.org/chromiumos/config/go/test/api"
)

// CommonContainerVolumes provides the volumes
// that will be provided to each container request.
func CommonContainerVolumes() []string {
	return []string{
		"/creds:/creds",
	}
}

// ContainerBuilder constructs a container request
// with the default assumption of using a generic template.
type ContainerBuilder struct {
	// Inputs
	ContainerId          string
	ContainerImageKey    string
	ContainerImagePath   string
	ContainerArtifactDir string
	Cmd                  string

	// Optional Inputs
	ContainerTemplate *api.Template
	DynamicDeps       []*api.DynamicDep
	AdditionalVolumes []string
	Envs              []string
	Network           string
}

// NewContainerBuilder initializes.
func NewContainerBuilder(contId, imageKey, imagePath, artifactDir, cmd string) *ContainerBuilder {
	return &ContainerBuilder{
		ContainerId:          contId,
		ContainerImageKey:    imageKey,
		ContainerImagePath:   imagePath,
		ContainerArtifactDir: artifactDir,
		Cmd:                  cmd,
		DynamicDeps:          []*api.DynamicDep{},
		AdditionalVolumes:    []string{},
	}
}

// SetCustomTemplate overwrites the assumed generic template in place for
// a well defined cros-tool-runner container template.
func (builder *ContainerBuilder) SetCustomTemplate(template *api.Template) {
	builder.ContainerTemplate = template
}

func (builder *ContainerBuilder) SetDynamicDeps(deps map[string]string) {
	for k, v := range deps {
		builder.DynamicDeps = append(builder.DynamicDeps, &api.DynamicDep{
			Key:   k,
			Value: v,
		})
	}
}

func (builder *ContainerBuilder) SetAdditionalVolumes(volumes []string) {
	builder.AdditionalVolumes = append(builder.AdditionalVolumes, volumes...)
}

func (builder *ContainerBuilder) SetEnvs(envs []string) {
	builder.Envs = envs
}

func (builder *ContainerBuilder) AppendEnv(envs ...string) {
	builder.Envs = append(builder.Envs, envs...)
}

// Build constructs the container request from the information
// provided to the ContainerBuilder.
func (builder *ContainerBuilder) Build() *api.ContainerRequest {
	if builder.ContainerTemplate == nil {
		cmds := strings.Split(builder.Cmd, " ")
		binaryName := ""
		binaryArgs := []string{}
		if len(cmds) > 0 {
			binaryName = cmds[0]
		}
		if len(cmds) > 1 {
			binaryArgs = cmds[1:]
		}
		builder.ContainerTemplate = &api.Template{
			Container: &api.Template_Generic{
				Generic: &api.GenericTemplate{
					BinaryName:        binaryName,
					BinaryArgs:        binaryArgs,
					DockerArtifactDir: builder.ContainerArtifactDir,
					AdditionalVolumes: append(builder.AdditionalVolumes, CommonContainerVolumes()...),
					Env:               builder.Envs,
				},
			},
		}
	}
	return &api.ContainerRequest{
		DynamicIdentifier:  builder.ContainerId,
		Container:          builder.ContainerTemplate,
		ContainerImageKey:  builder.ContainerImageKey,
		ContainerImagePath: builder.ContainerImagePath,
		DynamicDeps:        builder.DynamicDeps,
		Network:            builder.Network,
	}
}
