// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"

	"go.chromium.org/chromiumos/config/go/test/api"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/luciexe/build"
)

// InjectableStorage is an object that dictates how to interact with
// the dictionary of injectable objects for dependency injection.
type InjectableStorage struct {
	injectables map[string]interface{}

	// Cached Injectables map
	Injectables map[string]interface{}
}

func NewInjectableStorage() *InjectableStorage {
	return &InjectableStorage{
		injectables: map[string]interface{}{},
		Injectables: map[string]interface{}{},
	}
}

// LoadInjectables takes the dictionary of injectable objects and converts them
// into json interactable `interface{}`s.
func (storage *InjectableStorage) LoadInjectables() error {
	var allErrs error
	var err error
	storage.Injectables = map[string]interface{}{}

	for key, val := range storage.injectables {
		storage.Injectables[key], err = toInterface(val)
		if err != nil {
			allErrs = errors.Append(allErrs, fmt.Errorf("Failed to load injectables, %s", err))
		}
	}
	return allErrs
}

// Get searches through the Storage and returns an error if the object is not found.
func (storage *InjectableStorage) Get(key string) (interface{}, error) {
	split_key := strings.Split(key, ".")
	switch {
	// OS Environment variables aren't stored directly in the injectables dictionary.
	case strings.HasPrefix(key, "env-"):
		return os.Getenv(strings.TrimPrefix(key, "env-")), nil
	case strings.HasPrefix(key, "BOOL="):
		return boolHandler(strings.TrimPrefix(key, "BOOL=")), nil
	case strings.HasPrefix(key, "FMT="):
		return fmtHandler(storage, strings.TrimPrefix(key, "FMT=")), nil
	case strings.HasPrefix(key, "JSON="):
		return jsonHandler(storage, strings.TrimPrefix(key, "JSON="))
	case AnyRegex.MatchString(key):
		return anyHandler(storage, key)
	case key == "NIL":
		return nil, nil

	default:
		return stepThroughInterface(storage.Injectables, split_key)
	}
}

// Set stores any proto workable object into the storage.
func (storage *InjectableStorage) Set(key string, obj interface{}) error {
	var err error
	if storage.isValidType(obj) {
		storage.injectables[key] = obj
	} else {
		err = fmt.Errorf("Failed to set %s in storage, %s is not a valid `proto` type", key, reflect.TypeOf(obj))
	}
	return err
}

// LogStorageToStep writes the json structure of the storage as a log in a step.
func (storage *InjectableStorage) LogStorageToBuild(ctx context.Context, buildState *build.State) {
	_ = storage.LoadInjectables()

	storageLog := buildState.Log("Injectable Storage Contents")
	storageJson, _ := json.MarshalIndent(storage.Injectables, "", "    ")
	storageStr := string(storageJson)
	_, err := storageLog.Write([]byte(storageStr))
	if err != nil {
		logging.Infof(ctx, "Failed to write contents of injectable storage, %s", err)
	}
}

// isValidType checks if a type implements ProtoMessage or is a basic, non struct, type.
func (storage *InjectableStorage) isValidType(obj interface{}) bool {
	protoType := reflect.TypeOf((*protoreflect.ProtoMessage)(nil)).Elem()
	if reflect.TypeOf(obj).Kind() == reflect.Pointer || reflect.TypeOf(obj).Kind() == reflect.Struct {
		if reflect.TypeOf(obj).Implements(protoType) {
			return true
		}
	} else {
		if isSlice(obj) {
			valid := true
			slice := TranslateSliceToInterface(obj)
			for _, obj_ := range slice {
				valid = valid && storage.isValidType(obj_)
			}
			return valid
		}

		return true
	}

	return false
}

// Inject implements the logic for which Dependency Injection is based on.
// Dependency injection is the process in which an incoming ProtoMessage can
// have a placeholder replaced with a value found within some provided
// map[string]interface{} which contains all the exposed values for dependency
// injection.
//
// Functionality:
// * Keys are split by "."
// * Keys should be camelCase
// * Keys can be a string or a number. Numbers would represent indexing an array.
// * If the injection_point is an array and the injectable is also an array of the same type, then it will override the array.
// * If the injection_point is an array and the injectable is not an array of the same type, then it will append into the array.
//
// Example:
//
//	message Example {
//		IpEndpoint endpoint = 1;
//	}
//
//	injectables := map[string]interface{}
//	injectables["example_endpoint"] = IpEndpoint {
//		address: "localhost",
//		port: 12345
//	}
//
//	receiver := Example {endpoint: {address: "localhost"}}
//
//	Inject(receiever, "endpoint.port", injectables, "example_endpoint.port")
func Inject(receiver protoreflect.ProtoMessage, injection_point string, storage *InjectableStorage, injection_key string) (err error) {
	// Catch all thrown exceptions and recover to error instead.
	defer func() {
		if r := recover(); r != nil {
			err = r.(error)
		}
	}()

	receiver_map, err := protoToInterfaceMap(receiver)
	if err != nil {
		return fmt.Errorf("Failed to convert %s to map[string]interface{}, %s", getType(receiver), err)
	}

	injectable, err := storage.Get(injection_key)
	if err != nil {
		return fmt.Errorf("Failed to get %s from injectables storage for %s, %s", injection_key, getType(receiver), err)
	}

	if injection_point == "" {
		receiver_map = injectable.(map[string]interface{})
	} else {
		injection_point_parts := strings.Split(injection_point, ".")
		receiving_point, err := stepThroughInterface(receiver_map, injection_point_parts[0:len(injection_point_parts)-1])
		if err != nil {
			return fmt.Errorf("Failed to reach point of injection %s for %s, %s", injection_point, getType(receiver), err)
		}

		last_part := injection_point_parts[len(injection_point_parts)-1]
		err = setValue(receiving_point, last_part, injectable)
		if err != nil {
			return fmt.Errorf("Failed to inject %s into %s for %s, %s", injection_key, injection_point, getType(receiver), err)
		}
	}

	err = unmarshalInterfaceProtoMapToProto(receiver_map, receiver)
	return
}

// InjectDependencies handles loading the storage's injectables and injecting the dependencies into the receiver.
func InjectDependencies(receiver protoreflect.ProtoMessage, storage *InjectableStorage, deps []*api.DynamicDep) error {
	var allErrs error
	err := storage.LoadInjectables()
	if err != nil {
		allErrs = errors.Append(allErrs, fmt.Errorf("Failed to load some injectables, %s", err))
	}
	for _, dep := range deps {
		if err := Inject(receiver, dep.Key, storage, dep.Value); err != nil {
			allErrs = errors.Append(allErrs, fmt.Errorf("Failed to inject dependency, %s", err))
		}
	}

	return allErrs
}

// setValue stores the value into the obj at key.
func setValue(obj interface{}, key string, value interface{}) error {
	if key_num, err := strconv.ParseInt(key, 10, 64); err == nil {
		if isSlice(obj) {
			slice := TranslateSliceToInterface(obj)
			if int(key_num) < len(slice) {
				obj.([]interface{})[key_num] = value
			} else {
				return fmt.Errorf("Key %s not found in slice of length %d", key, len(slice))
			}
		} else {
			return fmt.Errorf("Expect slice for injecting at %s, found %s", key, getType(obj).String())
		}
	} else {
		if val, ok := obj.(map[string]interface{})[key]; ok {
			if isSlice(val) && getType(val) != getType(value) {
				slice := TranslateSliceToInterface(val)
				slice = append(slice, value)
				value = slice
			}
		}
		obj.(map[string]interface{})[key] = value
	}

	return nil
}

// indexAt indexes the object depending on what the index type is.
func indexAt(obj interface{}, index string) (interface{}, error) {
	if index_num, err := strconv.ParseInt(index, 10, 64); err == nil {
		slice := TranslateSliceToInterface(obj)
		if int(index_num) < len(slice) {
			return slice[index_num], nil
		} else {
			return nil, fmt.Errorf("Failed to index %s, tried to index %d but length was %d", reflect.TypeOf(obj), index_num, len(slice))
		}
	} else {
		if val, ok := obj.(map[string]interface{})[index]; ok {
			return val, nil
		} else {
			return nil, fmt.Errorf("Failed to index %s, missing key: %s", reflect.TypeOf(obj), index)
		}
	}
}

// isSlice returns true if obj is of type slice.
func isSlice(obj interface{}) bool {
	return reflect.ValueOf(obj).Kind() == reflect.Slice
}

// getType returns the type of obj as dictated by the reflect library.
func getType(obj interface{}) reflect.Type {
	return reflect.TypeOf(obj)
}

// unmarshalInterfaceProtoMapToProto converts a map[string]interface{}
// back into a provided proto message.
func unmarshalInterfaceProtoMapToProto(proto_map map[string]interface{}, proto protoreflect.ProtoMessage) error {
	json_bytes, err := json.Marshal(proto_map)
	if err != nil {
		return errors.Annotate(err, "failed to marshal to json").Err()
	}

	unmarshaller := protojson.UnmarshalOptions{
		DiscardUnknown: true,
		AllowPartial:   true,
	}
	err = unmarshaller.Unmarshal(json_bytes, proto)
	if err != nil {
		return errors.Annotate(err, "failed to unmarshal to proto").Err()
	}

	return nil
}

// Converts any object into an actual interface{} type.
func toInterface(obj interface{}) (interface{}, error) {
	if isSlice(obj) {
		return toInterfaceSlice(obj)
	} else if proto, ok := obj.(protoreflect.ProtoMessage); ok {
		return protoToInterfaceMap(proto)
	} else {
		return obj, nil
	}
}

// Converts slice objects into a slice of interface{}.
func toInterfaceSlice(obj interface{}) ([]interface{}, error) {
	var err error
	interfaces := []interface{}{}

	if !isSlice(obj) {
		return nil, fmt.Errorf("function `toInterfaceSlice` expected a slice, received %s", reflect.ValueOf(obj).Kind())
	}

	slice := TranslateSliceToInterface(obj)
	for _, _obj := range slice {
		_interface, err := toInterface(_obj)
		if err != nil {
			return nil, err
		}
		interfaces = append(interfaces, _interface)
	}

	return interfaces, err
}

// Coverts protos into a map[string]interface{} by marshaling and unmarshaling through json.
func protoToInterfaceMap(proto protoreflect.ProtoMessage) (map[string]interface{}, error) {
	var err error
	var json_bytes []byte
	obj_map := map[string]interface{}{}
	json_bytes, err = protojson.Marshal(proto)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(json_bytes, &obj_map)
	if err != nil {
		return nil, err
	}

	return obj_map, nil
}

// stepThroughInterface uses an array of keys to step through
// the indexes of the provided interface{}.
func stepThroughInterface(obj interface{}, steps []string) (interface{}, error) {
	var err error
	for _, step := range steps {
		if step == "" {
			break
		}
		obj, err = indexAt(obj, step)
		if err != nil {
			return nil, err
		}
	}

	return obj, nil
}

// TranslateSliceToInterface expects a slice object in.
// The slice then gets forcefully casted into []interface{}
// which is a generic slice form that can be interacted with
// by dependency injection.
func TranslateSliceToInterface(slice interface{}) []interface{} {
	_type := reflect.ValueOf(slice)
	if _type.IsNil() {
		return nil
	}
	if _type.Kind() != reflect.Slice {
		panic(errors.New(fmt.Sprintf("Cannot translate %s objects to []interface{}", _type.Kind())))
	}

	result := make([]interface{}, _type.Len())
	for i := range result {
		result[i] = _type.Index(i).Interface()
	}

	return result
}
