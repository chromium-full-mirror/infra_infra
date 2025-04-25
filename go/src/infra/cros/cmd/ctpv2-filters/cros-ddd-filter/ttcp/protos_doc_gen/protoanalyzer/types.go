// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package protoanalyzer

import (
	"sort"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// FileInfo contains the summarized analysis of a protobuffer package
type FileInfo struct {
	Name        string                 // Name of the package
	Description []string               // Description of the package
	Messages    map[string]MessageInfo //Messages included in the package
}

// GetMessageSortedByName returns a slice of all the messages contained in the
// protobuffer package f in alphabetical order of their Name
func (f FileInfo) GetMessageSortedByName() []MessageInfo {
	sortedMessages := []MessageInfo{}
	for _, message := range f.Messages {
		sortedMessages = append(sortedMessages, message)
	}
	sort.Slice(sortedMessages, func(i, j int) bool {
		return sortedMessages[i].Name < sortedMessages[j].Name
	})
	return sortedMessages
}

// GetMessageSortedByName returns a slice of all the messages contained in the
// protobuffer package f in alphabetical order of their FullName
func (f FileInfo) GetMessageSortedByFullName() []MessageInfo {
	sortedMessages := []MessageInfo{}
	for _, message := range f.Messages {
		sortedMessages = append(sortedMessages, message)
	}
	sort.Slice(sortedMessages, func(i, j int) bool {
		return sortedMessages[i].FullName < sortedMessages[j].FullName
	})
	return sortedMessages
}

// Message contains the summarized analysis of a protobuffer message
type MessageInfo struct {
	Name        string               // Name of the message
	FullName    string               // Full proto path of the message
	Description []string             // Description of the message
	Fields      map[string]FieldInfo // Fields of the message
}

// GetFieldsSortedByName returns a slice of all fields contained in the
// protobuffer message m in alphabetical order of their Names.
func (m MessageInfo) GetFieldsSortedByName() []FieldInfo {
	sortedFields := []FieldInfo{}
	for _, field := range m.Fields {
		sortedFields = append(sortedFields, field)
	}
	sort.Slice(sortedFields, func(i, j int) bool {
		return sortedFields[i].Name < sortedFields[j].Name
	})
	return sortedFields
}

// FieldInfo contains the summarized analysis of a protobufer message field.
type FieldInfo struct {
	Name        string    // Name of the field
	Description []string  // Protodocs.doc attached to the field
	Type        FieldType // Type of the field.
}

// FieldType is the type that denotes any type that can be a protobuffer
// message field type.
type FieldType interface {
	isFieldType()     // isFieldType denotes if a struct is a FieldType
	ToString() string // ToString returns a user friendly string representing the type
}

func (ScalarFieldType) isFieldType()    {}
func (ReferenceFieldType) isFieldType() {}
func (OneOfFieldType) isFieldType()     {}
func (ArrayField) isFieldType()         {}
func (MapField) isFieldType()           {}

// ScalarFieldType is the generic type for double,float,int32,int64,uint32,uint64,sint32,sint64,fixed32,fixed64,sfixed32,sfixed64,bool,string,bytes
type ScalarFieldType struct {
	kind protoreflect.Kind // Defines which specific protobuffer saclar the type is.
}

func (s ScalarFieldType) ToString() string {
	return s.kind.String()
}

// ReferenceFieldType is the type of the fields that contain another protobuffer message.
type ReferenceFieldType struct {
	Path string // Name of the protobuffer message.
}

func (r ReferenceFieldType) ToString() string {
	return r.Path
}

// OneOfFieldType represents a one field in a protobuf message.
// Note: In the protobuf descriptor the fields of a one of are listed among the other fields of the message.
//
//	The protoanalyzer difers by grouping all the fields of a OneOf under the OneOf field. This is more in
//	line with the protobufer syntax.
type OneOfFieldType struct {
	SubFields map[string]FieldInfo
}

// Returns the field defined under the OneOf o in alphabetical order of their name
func (o OneOfFieldType) GetSubFieldsSortedByName() []FieldInfo {
	sortedFields := []FieldInfo{}
	for _, field := range o.SubFields {
		sortedFields = append(sortedFields, field)
	}
	sort.Slice(sortedFields, func(i, j int) bool {
		return sortedFields[i].Name < sortedFields[j].Name
	})
	return sortedFields
}

func (o OneOfFieldType) ToString() string {
	return "OneOf"
}

func (o OneOfFieldType) addSubfield(f FieldInfo) {
	o.SubFields[f.Name] = f
}

// ArrayField represents a repeated field in a protobug message
type ArrayField struct {
	SubType ArrayFieldSubSType
}

// An array can only store scalar types and reference types.
// ArrayFieldSubSType enforces this
type ArrayFieldSubSType interface {
	isArrayFieldSubType()
	ToString() string
}

func (ScalarFieldType) isArrayFieldSubType()    {}
func (ReferenceFieldType) isArrayFieldSubType() {}

func (a ArrayField) ToString() string {
	return a.SubType.ToString() + "[]"
}

// MapField represents a map field in a protobug message
type MapField struct {
	Key   ScalarFieldType // Key can be any scalar except floating point types and bytes
	Value MapValueType
}

// An map can store any value type except another map field.
// MapValueType enforces this
type MapValueType interface {
	isMapValueType()
	ToString() string
}

func (ScalarFieldType) isMapValueType()    {}
func (ReferenceFieldType) isMapValueType() {}
func (OneOfFieldType) isMapValueType()     {}
func (ArrayField) isMapValueType()         {}

func (m MapField) ToString() string {
	return "map[" + m.Key.ToString() + "]" + m.Value.ToString()
}
