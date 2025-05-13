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
	injectables map[string]any

	// Cached Injectables map
	Injectables map[string]any
}

func NewInjectableStorage() *InjectableStorage {
	return &InjectableStorage{
		injectables: map[string]any{},
		Injectables: map[string]any{},
	}
}

// LoadInjectables takes the dictionary of injectable objects and converts them
// into json interactable `interface{}`s.
func (storage *InjectableStorage) LoadInjectables() error {
	var allErrs error
	var err error
	storage.Injectables = map[string]any{}

	for key, val := range storage.injectables {
		storage.Injectables[key], err = toInterface(val)
		if err != nil {
			allErrs = errors.Append(allErrs, fmt.Errorf("failed to load injectables, %w", err))
		}
	}
	return allErrs
}

// Get searches through the Storage and returns an error if the object is not found.
func (storage *InjectableStorage) Get(key string) (any, error) {
	splitKey := strings.Split(key, ".")
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
		return stepThroughInterface(storage.Injectables, splitKey)
	}
}

// Set stores any proto workable object into the storage.
func (storage *InjectableStorage) Set(key string, obj any) error {
	var err error
	if storage.isValidType(obj) {
		storage.injectables[key] = obj
	} else {
		err = fmt.Errorf("failed to set %s in storage, %s is not a valid `proto` type", key, reflect.TypeOf(obj))
	}
	return err
}

// LogStorageToBuild writes the json structure of the storage as a log in a step.
func (storage *InjectableStorage) LogStorageToBuild(ctx context.Context, buildState *build.State) {
	_ = storage.LoadInjectables()

	storageLog := buildState.Log("Injectable Storage Contents")
	storageJSON, _ := json.MarshalIndent(storage.Injectables, "", "    ")
	storageStr := string(storageJSON)
	_, err := storageLog.Write([]byte(storageStr))
	if err != nil {
		logging.Infof(ctx, "Failed to write contents of injectable storage, %s", err)
	}
}

// isValidType checks if a type implements ProtoMessage or is a basic, non struct, type.
func (storage *InjectableStorage) isValidType(obj any) bool {
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
func Inject(receiver protoreflect.ProtoMessage, injectionPoint string, storage *InjectableStorage, injectionKey string) (err error) {
	// Catch all thrown exceptions and recover to error instead.
	defer func() {
		if r := recover(); r != nil {
			err = r.(error)
		}
	}()

	receiverMap, err := protoToInterfaceMap(receiver)
	if err != nil {
		return fmt.Errorf("failed to convert %s to map[string]interface{}, %w", getType(receiver), err)
	}

	injectable, err := storage.Get(injectionKey)
	if err != nil {
		return fmt.Errorf("failed to get %s from injectables storage for %s, %w", injectionKey, getType(receiver), err)
	}

	if injectionPoint == "" {
		receiverMap = injectable.(map[string]any)
	} else {
		injectionPointParts := strings.Split(injectionPoint, ".")
		receivingPoint, err := stepThroughInterface(receiverMap, injectionPointParts[0:len(injectionPointParts)-1])
		if err != nil {
			return fmt.Errorf("failed to reach point of injection %s for %s, %w", injectionPoint, getType(receiver), err)
		}

		lastPart := injectionPointParts[len(injectionPointParts)-1]
		err = setValue(receivingPoint, lastPart, injectable)
		if err != nil {
			return fmt.Errorf("failed to inject %s into %s for %s, %w", injectionKey, injectionPoint, getType(receiver), err)
		}
	}

	err = unmarshalInterfaceProtoMapToProto(receiverMap, receiver)
	return
}

// InjectDependencies handles loading the storage's injectables and injecting the dependencies into the receiver.
func InjectDependencies(receiver protoreflect.ProtoMessage, storage *InjectableStorage, deps []*api.DynamicDep) error {
	var allErrs error
	err := storage.LoadInjectables()
	if err != nil {
		allErrs = errors.Append(allErrs, fmt.Errorf("failed to load some injectable, %w", err))
	}
	for _, dep := range deps {
		if err := Inject(receiver, dep.Key, storage, dep.Value); err != nil {
			allErrs = errors.Append(allErrs, fmt.Errorf("failed to inject dependency, %w", err))
		}
	}

	return allErrs
}

// setValue stores the value into the obj at key.
func setValue(obj any, key string, value any) error {
	if keyNum, err := strconv.ParseInt(key, 10, 64); err == nil {
		if isSlice(obj) {
			slice := TranslateSliceToInterface(obj)
			if int(keyNum) < len(slice) {
				obj.([]any)[keyNum] = value
			} else {
				return fmt.Errorf("key %s not found in slice of length %d", key, len(slice))
			}
		} else {
			return fmt.Errorf("expect slice for injecting at %s, found %s", key, getType(obj).String())
		}
	} else {
		if val, ok := obj.(map[string]any)[key]; ok {
			if isSlice(val) && getType(val) != getType(value) {
				slice := TranslateSliceToInterface(val)
				slice = append(slice, value)
				value = slice
			}
		}
		obj.(map[string]any)[key] = value
	}

	return nil
}

// indexAt indexes the object depending on what the index type is.
func indexAt(obj any, index string) (any, error) {
	if indexNum, err := strconv.ParseInt(index, 10, 64); err == nil {
		slice := TranslateSliceToInterface(obj)
		if int(indexNum) < len(slice) {
			return slice[indexNum], nil
		} else {
			return nil, fmt.Errorf("failed to index %s, tried to index %d but length was %d", reflect.TypeOf(obj), indexNum, len(slice))
		}
	} else {
		if val, ok := obj.(map[string]any)[index]; ok {
			return val, nil
		} else {
			return nil, fmt.Errorf("failed to index %s, missing key: %s", reflect.TypeOf(obj), index)
		}
	}
}

// isSlice returns true if obj is of type slice.
func isSlice(obj any) bool {
	return reflect.ValueOf(obj).Kind() == reflect.Slice
}

// getType returns the type of obj as dictated by the reflect library.
func getType(obj any) reflect.Type {
	return reflect.TypeOf(obj)
}

// unmarshalInterfaceProtoMapToProto converts a map[string]interface{}
// back into a provided proto message.
func unmarshalInterfaceProtoMapToProto(protoMap map[string]any, proto protoreflect.ProtoMessage) error {
	jsonBytes, err := json.Marshal(protoMap)
	if err != nil {
		return errors.Annotate(err, "failed to marshal to json").Err()
	}

	unmarshaller := protojson.UnmarshalOptions{
		DiscardUnknown: true,
		AllowPartial:   true,
	}
	err = unmarshaller.Unmarshal(jsonBytes, proto)
	if err != nil {
		return errors.Annotate(err, "failed to unmarshal to proto").Err()
	}

	return nil
}

// Converts any object into an actual interface{} type.
func toInterface(obj any) (any, error) {
	if isSlice(obj) {
		return toInterfaceSlice(obj)
	} else if proto, ok := obj.(protoreflect.ProtoMessage); ok {
		return protoToInterfaceMap(proto)
	} else {
		return obj, nil
	}
}

// Converts slice objects into a slice of interface{}.
func toInterfaceSlice(obj any) ([]any, error) {
	var err error
	interfaces := []any{}

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
func protoToInterfaceMap(proto protoreflect.ProtoMessage) (map[string]any, error) {
	var err error
	var jsonBytes []byte
	objMap := map[string]any{}
	jsonBytes, err = protojson.Marshal(proto)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(jsonBytes, &objMap)
	if err != nil {
		return nil, err
	}

	return objMap, nil
}

// stepThroughInterface uses an array of keys to step through
// the indexes of the provided interface{}.
func stepThroughInterface(obj any, steps []string) (any, error) {
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
func TranslateSliceToInterface(slice any) []any {
	_type := reflect.ValueOf(slice)
	if _type.IsNil() {
		return nil
	}
	if _type.Kind() != reflect.Slice {
		panic(errors.New(fmt.Sprintf("Cannot translate %s objects to []interface{}", _type.Kind())))
	}

	result := make([]any, _type.Len())
	for i := range result {
		result[i] = _type.Index(i).Interface()
	}

	return result
}
