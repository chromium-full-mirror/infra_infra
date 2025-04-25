// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package protoanalyzer

import (
	"io/ioutil"
	"log"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"

	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/protos/protodocs"
)

// LoadDescriptors loads all the descriptors from the paths in ProtoDescriptorPath
// loads them in a protoregisty and returns that registry.
func LoadDescriptors(ProtoDescriptorPath []string) (*protoregistry.Files, error) {
	var protoFiles = new(protoregistry.Files)
	registerProtoFile(protoFiles, &protodocs.File_protos_protodocs_protodocs_proto)
	for _, f := range ProtoDescriptorPath {
		log.Println(f)
		protoFile, err := ioutil.ReadFile(f)
		if err != nil {
			return protoFiles, err
		}

		log.Println(string(protoFile))
		pb_set := new(descriptorpb.FileDescriptorSet)
		if err := proto.Unmarshal(protoFile, pb_set); err != nil {
			log.Println("unmarshal")
			return protoFiles, err
		}

		for i := range pb_set.File {
			file := pb_set.File[i]
			log.Println("loading package:" + *file.Package + " name:" + *file.Name)
			err := registerProtoFileProto(protoFiles, file)
			if err != nil {
				return protoFiles, err
			}

		}
	}
	log.Println("All files are loaded")
	return protoFiles, nil
}

// registerProtoFileProto registers a protoFile proto into the protoregistry protoFiles
func registerProtoFileProto(protoFiles *protoregistry.Files, protoFile *descriptorpb.FileDescriptorProto) error {
	fd, err := protodesc.NewFile(protoFile, protoFiles)
	if err != nil {
		log.Println("  ", err)
		return err
	}
	return registerProtoFile(protoFiles, &fd)
}

// registerProtoFile register a file descriptor into the protoregistry protoFiles
func registerProtoFile(protoFiles *protoregistry.Files, protoFile *protoreflect.FileDescriptor) error {
	err := protoFiles.RegisterFile(*protoFile)
	if err != nil {
		log.Println("  ", err)
		return err
	}
	return nil
}
