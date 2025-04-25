// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package protoanalyzer

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/protos/protodocs"
)

// AnalyzeFile analyzes a Protobuf package in order to extract the relation of
// the types defined in the package and their documentation. All the documentation
// of the package, messages and fields should be documented with the extensions
// provided in the `protodocs` package
func AnalyzeFile(file protoreflect.FileDescriptor) FileInfo {
	//analyze the file's messages
	messages := map[string]MessageInfo{}
	protoMessages := file.Messages()
	for messageIndex := range protoMessages.Len() {
		message := protoMessages.Get(messageIndex)
		analyzedMessages := AnalyzeMessage(message)
		for name, msg := range analyzedMessages {
			messages[name] = msg
		}
	}

	fileInfo := FileInfo{
		Name:        string(file.Package()),
		Description: proto.GetExtension(file.Options(), protodocs.E_FileDocs).([]string),
		Messages:    messages,
	}
	return fileInfo
}

// AnalyzeMessage analyses a message and its embedded messages. It returns information
// for all those message in a map where the key is the name of the message.
func AnalyzeMessage(message protoreflect.MessageDescriptor) map[string]MessageInfo {
	messages := map[string]MessageInfo{}
	embeddedMessages := message.Messages()
	for eMessageIndex := range embeddedMessages.Len() {
		embeddedMessage := embeddedMessages.Get(eMessageIndex)
		if !embeddedMessage.IsMapEntry() {
			messageInfo := AnalyzeMessage(embeddedMessage)
			for name, msg := range messageInfo {
				messages[name] = msg
			}
		}
	}

	fields := map[string]FieldInfo{}
	for _, f := range AnalyzeFields(message) {
		fields[f.Name] = f
	}

	messageInfo := MessageInfo{
		Name:        string(message.Name()),
		FullName:    string(message.FullName()),
		Description: proto.GetExtension(message.Options(), protodocs.E_MessageDocs).([]string),
		Fields:      fields,
	}
	messages[messageInfo.Name] = messageInfo
	return messages
}

// AnalyzeFields analyses the fields of a message. It all the fields belonging to a OneOf are grouped into
// a single Field. The result is returned as a map where the key is the name of the field and the value
// is the result of the analysuios
func AnalyzeFields(message protoreflect.MessageDescriptor) map[string]FieldInfo {
	oneOfInfoLookup := map[string]OneOfFieldType{}
	fieldInfos := map[string]FieldInfo{}

	oneOfs := message.Oneofs()
	for oneOfIndex := range oneOfs.Len() {
		oneOf := oneOfs.Get(oneOfIndex)
		oneOfName := string(oneOf.Name())
		oneOfDoc := proto.GetExtension(oneOf.Options(), protodocs.E_OneDocs).([]string)

		oneOfType := OneOfFieldType{SubFields: map[string]FieldInfo{}}
		oneOfInfoLookup[oneOfName] = oneOfType

		fieldInfos[oneOfName] = FieldInfo{
			Name:        oneOfName,
			Description: oneOfDoc,
			Type:        oneOfType,
		}
	}

	fields := message.Fields()
	for fieldIndex := range fields.Len() {
		field := fields.Get(fieldIndex)
		fieldName := field.TextName()
		fieldDoc := proto.GetExtension(field.Options(), protodocs.E_Docs).([]string)
		fieldType := ExtractFieldType(field)
		fieldInfo := FieldInfo{
			Name:        fieldName,
			Description: fieldDoc,
			Type:        fieldType,
		}
		oneOf := field.ContainingOneof()
		if oneOf != nil {
			oneOfInfoLookup[string(oneOf.Name())].addSubfield(fieldInfo)
		} else {
			fieldInfos[fieldName] = fieldInfo
		}

	}

	return fieldInfos
}

// extractFieldType returns a string representing the type of the descriptor field.
func ExtractFieldType(field protoreflect.FieldDescriptor) FieldType {
	if field.IsMap() {
		protoValue := field.MapValue()
		var valueType FieldType
		if protoValue.Kind() == protoreflect.MessageKind {
			valueType = ReferenceFieldType{Path: string(protoValue.Message().Name())}
		} else {
			valueType = ScalarFieldType{kind: field.Kind()}
		}
		fieldType := MapField{
			Key:   ScalarFieldType{kind: field.MapKey().Kind()},
			Value: valueType.(MapValueType),
		}
		return fieldType
	}
	var fieldType FieldType
	if field.Kind() == protoreflect.MessageKind {
		fieldType = ReferenceFieldType{Path: string(field.Message().Name())}
	} else {
		fieldType = ScalarFieldType{kind: field.Kind()}
	}
	if field.IsList() {
		fieldType = ArrayField{SubType: fieldType.(ArrayFieldSubSType)}
	}
	return fieldType
}

// fieldInfo contains the details of a protobuf message's field necessairy to
// generate documentation.
type FieldInfo_dep struct {
	// Name of the field
	Name string
	// Path's value is <name_of_oneof>.<field_name> if the field is member of a
	// oneOf block if not path will have the same value as the field's name
	Path string
	// True only if the field is part of a  oneOf
	IsOneOf bool
	// Type name of the field.
	FieldType string
	// Content of proto_docs.oneDoc attribute of the field.
	Description string
}
