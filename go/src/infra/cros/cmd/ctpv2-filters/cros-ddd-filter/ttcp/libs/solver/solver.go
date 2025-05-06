// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package solver

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"reflect"
	"regexp"
	"strings"

	"github.com/mitchellh/hashstructure/v2"
	"google.golang.org/protobuf/proto"

	ttcpSolver "go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/protos/ttcp/solver"
	ttcpSyntax "go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/protos/ttcp/syntax"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/datasets/swarmingdata"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/errors"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/inventory/croslab"
	"go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/inventory/deviceinfo"
)

// ClassFilter is an enumerated argument of the evalExpression function.
// For semantics of each element of ClassFilter please refer to the
// documentation of evalExpression.
type ClassFilter int

const (
	ClassFilterNone ClassFilter = iota
	ClassFilterOnlyTestable
	ClassFilterOnlyUntestable
	CONTAINERMAX = 2000000
	LARGECLASSES = 1000000
)

func (filter ClassFilter) String() string {
	return []string{"noFilter", "onlyTestable", "onlyUntestable"}[filter]
}

func ParseClassFilter(str string) (ClassFilter, error) {
	val, ok := map[string]ClassFilter{
		"noFilter":       ClassFilterNone,
		"onlyTestable":   ClassFilterOnlyTestable,
		"onlyUntestable": ClassFilterOnlyUntestable}[str]
	if !ok {
		return ClassFilterNone, errors.NewErrorf("Could not parse %s into a ClassFilter enum value", str)
	}
	return val, nil
}

// getClass convers a ClassExpression in a flat class
func getClass(expression *ttcpSyntax.ClassExpression, collection ttcpSyntax.Collection) (*ttcpSyntax.Class, error) {
	switch body := expression.Body.(type) {
	case *ttcpSyntax.ClassExpression_Name:
		cls, ok := collection.Classes[body.Name]
		if !ok {
			return nil, errors.NewErrorf("Class %s is not present in the collection.", body.Name)
		}
		return cls, nil
	case *ttcpSyntax.ClassExpression_Value:
		return body.Value, nil
	default:
		return nil, errors.NewErrorf("The ClassExpression has an expected body type:%v.", body)
	}
}

// filterDevice returns a filtered list of TargetVariant.
// args:
//   - TargetVariant: list to filter.
//   - class: the criteria to use to filter devices.
//   - add:
//   - If true, only the devices that fit the class criteria will be in the returned list.
//   - If false, only the device that do NOT fit the class criteria will be in the returned list.
func filterDevices(deviceInfo []deviceinfo.TargetVariant, class *ttcpSyntax.Class, add bool) []deviceinfo.TargetVariant {
	positiveDevices := applyClass(class.Expression, deviceInfo)
	filterDevices := []deviceinfo.TargetVariant{}
	for _, dev := range deviceInfo {
		_, ok := positiveDevices[dev.Id()]
		if add {
			if ok {
				filterDevices = append(filterDevices, dev)
			} else {
				log.Println("Excluding device:", dev.Id())
			}
		} else {
			if !ok {
				filterDevices = append(filterDevices, dev)
			} else {
				log.Println("Excluding device:", dev.Id())
			}
		}
	}
	return filterDevices
}

// EvalExpression solves a categorory expression:
//   - transforms the expression into a flat category. A flat category is an
//     enumerated category where the expression of each of its classes do not
//     refer to any other class by name
//   - For each flat class it finds in the inventory which devices belong to the
//     class.
//
// Returns:
// Error:
// Parameters:
//   - expression: The CategoryExpression to solve
//   - classFilter:
//     -- ClassFilterNone:           No filtering
//     -- ClassFilterOnlyTestable:   Remove any class that has not device in
//     the inventory
//     -- ClassFilterOnlyUntestable: Remove any class from the SolvedClass that
//     do have a device in the inventory
//   - TargetVariant: Inventory of available devices.
//   - optIn: If not null, the TargetVariant will be filtered down to only the
//     devices that fit the optIn class.
//   - optOut: If not null, the TargetVariant will be filtered out all the devices
//     that do fit the optOut class.
//   - collection: Dataset of predefined classes and categories that expression
//     can reference.
var solvedString = map[string]map[deviceinfo.TargetId]*ExtendedSolvedDevice{}
var solved = map[uint64]map[deviceinfo.TargetId]*ExtendedSolvedDevice{}

func EvalExpression(
	expression ttcpSyntax.CategoryExpression,
	classFilter ClassFilter,
	devicesInfo []deviceinfo.TargetVariant,
	optIn *ttcpSyntax.ClassExpression,
	optOut *ttcpSyntax.ClassExpression,
	collection ttcpSyntax.Collection,
	logger *log.Logger,
	solvedCache map[uint64]map[deviceinfo.TargetId]*ExtendedSolvedDevice,
	useSwarmingInventory bool,
	pool string) (ttcpSolver.SolvedCategory, error) {

	if optIn != nil {
		cls, err := getClass(optIn, collection)
		if err != nil {
			return ttcpSolver.SolvedCategory{}, errors.JoinError(
				"Unable to resolve the opt in class.",
				err)
		}
		devicesInfo = filterDevices(devicesInfo, cls, true)
	}

	if solvedCache != nil {
		solved = solvedCache
	}
	if optOut != nil {
		cls, err := getClass(optOut, collection)
		if err != nil {
			return ttcpSolver.SolvedCategory{}, errors.JoinError(
				"Unable to resolve the opt in class.",
				err)
		}
		devicesInfo = filterDevices(devicesInfo, cls, false)
	}

	// TODO(b:297298647) implement efficient solving that avoids the creation of the classes that do not have a
	//                   solution
	flatten, err := FlattenCategoryExpression(expression, collection, []string{})

	if err != nil {
		return ttcpSolver.SolvedCategory{}, errors.JoinError("Could not flatten expression.", err)
	}

	if len(flatten.Classes) == 0 {
		return ttcpSolver.SolvedCategory{}, errors.NewError("The category needs to have at least one class")
	}

	err = handleLargeExpression(len(flatten.Classes))
	if err != nil {
		return ttcpSolver.SolvedCategory{}, err
	}

	classes := []*ttcpSolver.SolvedClass{}
	for _, cls := range flatten.Classes {
		exp := cls.GetValue()
		classSolution := applyClass(exp.Expression, devicesInfo)
		// Only append if the ClassSolution finds matches. Otherwise we are eating time for nothing practical.
		if len(classSolution) > 0 {
			if useSwarmingInventory {
				classSolution = filterSwarmingDevices(classSolution, logger, pool)
				if len(classSolution) == 0 {
					continue
				}
			}
			eqcName, equivClasses, err := GenerateEqcName(classSolution, expression, devicesInfo, collection)
			if err != nil {
				return ttcpSolver.SolvedCategory{}, err
			}
			classes = append(classes,
				&ttcpSolver.SolvedClass{
					Expression:      exp,
					Targets:         toSolvedTargets(classSolution),
					LegacySolutions: extractBoardModelMap(classSolution),
					Name:            eqcName,
					Dimensions:      equivClasses,
				},
			)
		}
	}
	filteredClasses := []*ttcpSolver.SolvedClass{}
	switch classFilter {
	case ClassFilterNone:
		filteredClasses = classes
	case ClassFilterOnlyTestable:
		for _, cl := range classes {
			if len(cl.Targets) > 0 {
				filteredClasses = append(filteredClasses, cl)
			}
		}
	case ClassFilterOnlyUntestable:
		for _, cl := range classes {
			if len(cl.Targets) == 0 {
				filteredClasses = append(filteredClasses, cl)
			}
		}
	}

	solution := ttcpSolver.SolvedCategory{
		Expression: &expression,
		Classes:    filteredClasses,
	}
	if logger != nil {
		logger.Println("Completed evaling")

	}
	return solution, nil
}

type ExtendedSolvedDevice struct {
	solvedDevice   ttcpSolver.SolvedTarget
	board          string
	model          string
	imageVariant   string
	swarmingLabels []*ttcpSolver.SwarmingLabel
}

func deviceInfoToExtendedSolvedDevice(info deviceinfo.TargetVariant, propertyPath string, propertyValue interface{}) (*ExtendedSolvedDevice, error) {
	board, ok := info.Properties.PropertiesDetails["board"]
	swarmingLabels := []*ttcpSolver.SwarmingLabel{}
	if !ok {
		return nil, errors.NewErrorf("The device is missing the property \"board\". DeviceProperties:%s", info)
	}
	modelVal, err := getModelValue(info)
	if err != nil {
		return &ExtendedSolvedDevice{}, err
	}
	if strings.Contains(propertyPath, "swarming:") {
		propValue, ok := propertyValue.(string)
		if !ok {
			return nil, errors.NewErrorf("Swarming property value must be of type string. DeviceProperties:%s, PropertyValueType:%s", info, reflect.TypeOf(propertyValue))
		}
		if _, ok := info.Properties.PropertiesDetails[propertyPath]; ok {
			splitSlice := strings.Split(propertyPath, "swarming:_")
			// Take the last item from the list
			swarmingLabelName := splitSlice[len(splitSlice)-1]
			// Remove the "swarming:" prefix
			swarmingLabelName = strings.Replace(swarmingLabelName, "swarming:", "", 1)
			if swarmingLabelName != "hwid" {
				swarmingLabels = append(
					swarmingLabels,
					&ttcpSolver.SwarmingLabel{
						Label: swarmingLabelName, Value: propValue,
					})
			}
		}
	}
	return &ExtendedSolvedDevice{
		solvedDevice: ttcpSolver.SolvedTarget{
			DeviceId: info.DeviceId,
			ImageId:  info.BuildTarget.Id(),
		},
		board:          board.Values[0].(string),
		model:          modelVal,
		imageVariant:   info.BuildTarget.GetVariant(),
		swarmingLabels: swarmingLabels,
	}, nil

}

func toSolvedTargets(extendedDevicesInfo map[deviceinfo.TargetId]*ExtendedSolvedDevice) map[string]*ttcpSolver.SolvedTarget {
	results := map[string]*ttcpSolver.SolvedTarget{}
	for k, v := range extendedDevicesInfo {
		results[string(k)] = &ttcpSolver.SolvedTarget{
			DeviceId: v.solvedDevice.DeviceId,
			ImageId:  v.solvedDevice.ImageId,
			Info: &ttcpSolver.SolvedTargetInfo{
				Board:          v.board,
				Model:          v.model,
				ImageVariant:   v.imageVariant,
				SwarmingLabels: v.swarmingLabels,
			},
		}
	}
	return results
}

func extractBoardModelMap(extendedDevicesInfo map[deviceinfo.TargetId]*ExtendedSolvedDevice) []*ttcpSolver.LegacyTarget {
	boardToModels := map[string]map[string]interface{}{}
	for _, solvedDevice := range extendedDevicesInfo {
		models, ok := boardToModels[solvedDevice.board]
		if ok {
			models[solvedDevice.model] = nil
		} else {
			models = map[string]interface{}{solvedDevice.model: nil}
			boardToModels[solvedDevice.board] = models
		}
	}
	results := []*ttcpSolver.LegacyTarget{}
	for board, models := range boardToModels {
		modelList := []string{}
		for model := range models {
			modelList = append(modelList, model)
		}
		results = append(results,
			&ttcpSolver.LegacyTarget{
				Board:  board,
				Models: modelList,
			},
		)
	}
	return results
}

func applyClass(exp *ttcpSyntax.Expression, deviceInfo []deviceinfo.TargetVariant) map[deviceinfo.TargetId]*ExtendedSolvedDevice {
	sh, _ := proto.Marshal(exp)
	expHash := string(sh)
	v, hwExists := solvedString[expHash]
	if hwExists {
		return v
	}

	var solution = map[deviceinfo.TargetId]*ExtendedSolvedDevice{}
	switch operator := exp.Operator.(type) {
	case *ttcpSyntax.Expression_True:
		solution = applyTrue(deviceInfo)
	case *ttcpSyntax.Expression_Or:
		solution = applyOr(operator.Or, deviceInfo)
	case *ttcpSyntax.Expression_And:
		solution = applyAnd(operator.And, deviceInfo)
	case *ttcpSyntax.Expression_Not:
		solution = applyNot(operator.Not, deviceInfo)
	case *ttcpSyntax.Expression_Property:
		solution = applyPropertyCondition(operator.Property, deviceInfo)
	default:
		log.Fatalf("applyClass is not implemented for the expression type:%T", operator)
	}
	solvedString[expHash] = solution
	return solution
}

func applyTrue(deviceInfo []deviceinfo.TargetVariant) map[deviceinfo.TargetId]*ExtendedSolvedDevice {
	result := map[deviceinfo.TargetId]*ExtendedSolvedDevice{}
	for _, info := range deviceInfo {
		solvedDevice, err := deviceInfoToExtendedSolvedDevice(info, "", nil)
		if err != nil {
			log.Fatal(err)
		}
		result[info.Id()] = solvedDevice
	}
	return result
}

func applyOr(operator *ttcpSyntax.Or, deviceInfo []deviceinfo.TargetVariant) map[deviceinfo.TargetId]*ExtendedSolvedDevice {
	result := map[deviceinfo.TargetId]*ExtendedSolvedDevice{}
	if len(operator.SubExpressions) == 0 {
		return result
	}
	solvedSubExpressions := []map[deviceinfo.TargetId]*ExtendedSolvedDevice{}
	for _, subExp := range operator.SubExpressions {
		solvedSubExpressions = append(solvedSubExpressions, applyClass(subExp, deviceInfo))
	}

	for id, solvedDevice := range solvedSubExpressions[0] {
		result[id] = solvedDevice
	}
	for _, nextSubExpression := range solvedSubExpressions[1:] {
		for id, solvedDevice := range nextSubExpression {
			_, alreadyPresent := result[id]
			if !alreadyPresent {
				result[id] = solvedDevice
			} else {
				extendedSolvedDevices := []*ExtendedSolvedDevice{solvedDevice, result[id]}
				mergedSolvedDevice, err := mergeSolvedDevices(extendedSolvedDevices)
				if err != nil {
					log.Fatal(err)
				}
				result[id] = mergedSolvedDevice
			}
		}
	}
	return result
}

func applyAnd(operator *ttcpSyntax.And, deviceInfo []deviceinfo.TargetVariant) map[deviceinfo.TargetId]*ExtendedSolvedDevice {
	result := map[deviceinfo.TargetId]*ExtendedSolvedDevice{}
	if len(operator.SubExpressions) == 0 {
		return result
	}

	solvedSubExpressions := []map[deviceinfo.TargetId]*ExtendedSolvedDevice{}
	for _, subExp := range operator.SubExpressions {
		solvedSubExpressions = append(solvedSubExpressions, applyClass(subExp, deviceInfo))
	}
	for id, solvedDevice := range solvedSubExpressions[0] {
		result[id] = solvedDevice
	}
	for _, nextSubExpression := range solvedSubExpressions[1:] {
		newResult := map[deviceinfo.TargetId]*ExtendedSolvedDevice{}
		for id, solvedDevice := range nextSubExpression {
			_, alreadyPresent := result[id]
			if alreadyPresent {
				extendedSolvedDevices := []*ExtendedSolvedDevice{solvedDevice, result[id]}
				mergedSolvedDevice, err := mergeSolvedDevices(extendedSolvedDevices)
				if err != nil {
					log.Fatal(err)
				}
				newResult[id] = mergedSolvedDevice
			}
		}
		result = newResult
	}
	return result
}

func applyNot(operator *ttcpSyntax.Not, deviceInfo []deviceinfo.TargetVariant) map[deviceinfo.TargetId]*ExtendedSolvedDevice {
	solvedSubExpression := applyClass(operator.SubExpression, deviceInfo)
	result := map[deviceinfo.TargetId]*ExtendedSolvedDevice{}
	for _, info := range deviceInfo {
		_, present := solvedSubExpression[info.Id()]
		if !present {
			solvedDevice, err := deviceInfoToExtendedSolvedDevice(info, "", nil)
			if err != nil {
				log.Fatal(err)
			}
			result[info.Id()] = solvedDevice
		}
	}
	return result
}

func applyPropertyCondition(condition *ttcpSyntax.Condition, deviceInfo []deviceinfo.TargetVariant) map[deviceinfo.TargetId]*ExtendedSolvedDevice {
	switch typedCondition := condition.Condition.(type) {
	case *ttcpSyntax.Condition_Present:
		return applyPropertyConditionPresent(condition.PropertyPath, typedCondition.Present, deviceInfo)
	case *ttcpSyntax.Condition_StrEqual:
		return applyPropertyConditionStrEqual(condition.PropertyPath, typedCondition.StrEqual, deviceInfo)
	case *ttcpSyntax.Condition_StrRegexMatch:
		return applyPropertyConditionStrRegexMatch(condition.PropertyPath, typedCondition.StrRegexMatch, deviceInfo)
	case *ttcpSyntax.Condition_StrInSet:
		return applyPropertyConditionStrSet(condition.PropertyPath, typedCondition.StrInSet.Values, deviceInfo)
	case *ttcpSyntax.Condition_IntEqual:
		return applyPropertyConditionIntEqual(condition.PropertyPath, typedCondition.IntEqual, deviceInfo)
	case *ttcpSyntax.Condition_IntInSet:
		return applyPropertyConditionIntSet(condition.PropertyPath, typedCondition.IntInSet.Values, deviceInfo)
	case *ttcpSyntax.Condition_IntLess:
		return applyPropertyConditionIntLess(condition.PropertyPath, typedCondition.IntLess, deviceInfo)
	case *ttcpSyntax.Condition_IntLessOrEqual:
		return applyPropertyConditionIntLessOrEqual(condition.PropertyPath, typedCondition.IntLessOrEqual, deviceInfo)
	case *ttcpSyntax.Condition_IntGreater:
		return applyPropertyConditionIntGreater(condition.PropertyPath, typedCondition.IntGreater, deviceInfo)
	case *ttcpSyntax.Condition_IntGreaterOrEqual:
		return applyPropertyConditionIntGreaterOrEqual(condition.PropertyPath, typedCondition.IntGreaterOrEqual, deviceInfo)
	default:
		log.Fatalf("applyClass is not implemented for the expression type:%T", typedCondition)
	}
	return map[deviceinfo.TargetId]*ExtendedSolvedDevice{}
}

func applyPropertyConditionPresent(propertyPath string, conditionPresent bool, deviceInfo []deviceinfo.TargetVariant) map[deviceinfo.TargetId]*ExtendedSolvedDevice {
	result := map[deviceinfo.TargetId]*ExtendedSolvedDevice{}
	for _, info := range deviceInfo {
		_, present := info.Properties.PropertiesDetails[propertyPath]
		if present == conditionPresent {
			solvedDevice, err := deviceInfoToExtendedSolvedDevice(info, propertyPath, nil)
			if err != nil {
				log.Fatal(err)
			}
			result[info.Id()] = solvedDevice
		}
	}
	return result
}

func applyPropertyConditionStrEqual(propertyPath string, conditionValue string, deviceInfo []deviceinfo.TargetVariant) map[deviceinfo.TargetId]*ExtendedSolvedDevice {
	result := map[deviceinfo.TargetId]*ExtendedSolvedDevice{}
	for _, info := range deviceInfo {
		Values := info.Properties.PropertiesDetails[propertyPath].Values
		for _, value := range Values {
			if _, ok := value.(string); ok && value == conditionValue {
				solvedDevice, err := deviceInfoToExtendedSolvedDevice(info, propertyPath, value)
				if err != nil {
					log.Fatal(err)
				}
				result[info.Id()] = solvedDevice
				break
			}
		}
	}
	return result
}

func applyPropertyConditionStrRegexMatch(propertyPath string, conditionValue string, deviceInfo []deviceinfo.TargetVariant) map[deviceinfo.TargetId]*ExtendedSolvedDevice {
	result := map[deviceinfo.TargetId]*ExtendedSolvedDevice{}
	for _, info := range deviceInfo {
		Values := info.Properties.PropertiesDetails[propertyPath].Values
		for _, value := range Values {
			if _, ok := value.(string); ok {
				if matched, _ := regexp.MatchString(conditionValue, value.(string)); matched {
					solvedDevice, err := deviceInfoToExtendedSolvedDevice(info, propertyPath, value)
					if err != nil {
						log.Fatal(err)
					}
					result[info.Id()] = solvedDevice
					break
				}
			}
		}
	}
	return result
}

func applyPropertyConditionStrSet(propertyPath string, conditionValues []string, deviceInfo []deviceinfo.TargetVariant) map[deviceinfo.TargetId]*ExtendedSolvedDevice {
	valuesMap := map[string]interface{}{}
	for _, s := range conditionValues {
		valuesMap[s] = nil
	}
	result := map[deviceinfo.TargetId]*ExtendedSolvedDevice{}
	for _, info := range deviceInfo {
		Values := info.Properties.PropertiesDetails[propertyPath].Values
		for _, value := range Values {
			typedValue, ok := value.(string)
			if !ok {
				continue
			}
			if _, present := valuesMap[typedValue]; present {
				solvedDevice, err := deviceInfoToExtendedSolvedDevice(info, propertyPath, typedValue)
				if err != nil {
					log.Fatal(err)
				}
				result[info.Id()] = solvedDevice
				break
			}
		}
	}
	return result
}

func applyPropertyConditionIntEqual(propertyPath string, conditionValue int64, deviceInfo []deviceinfo.TargetVariant) map[deviceinfo.TargetId]*ExtendedSolvedDevice {
	result := map[deviceinfo.TargetId]*ExtendedSolvedDevice{}
	for _, info := range deviceInfo {
		Values := info.Properties.PropertiesDetails[propertyPath].Values
		for _, value := range Values {
			typedValue, ok := value.(int64)
			if ok && typedValue < conditionValue {
				solvedDevice, err := deviceInfoToExtendedSolvedDevice(info, propertyPath, value)
				if err != nil {
					log.Fatal(err)
				}
				result[info.Id()] = solvedDevice
				break
			}
		}
	}
	return result
}

func applyPropertyConditionIntLess(propertyPath string, conditionValue int64, deviceInfo []deviceinfo.TargetVariant) map[deviceinfo.TargetId]*ExtendedSolvedDevice {
	result := map[deviceinfo.TargetId]*ExtendedSolvedDevice{}
	for _, info := range deviceInfo {
		Values := info.Properties.PropertiesDetails[propertyPath].Values
		for _, value := range Values {
			typedValue, ok := value.(int64)
			if ok && typedValue < conditionValue {
				solvedDevice, err := deviceInfoToExtendedSolvedDevice(info, propertyPath, value)
				if err != nil {
					log.Fatal(err)
				}
				result[info.Id()] = solvedDevice
				break
			}
		}
	}
	return result
}

func applyPropertyConditionIntLessOrEqual(propertyPath string, conditionValue int64, deviceInfo []deviceinfo.TargetVariant) map[deviceinfo.TargetId]*ExtendedSolvedDevice {
	result := map[deviceinfo.TargetId]*ExtendedSolvedDevice{}
	for _, info := range deviceInfo {
		Values := info.Properties.PropertiesDetails[propertyPath].Values
		for _, value := range Values {
			typedValue, ok := value.(int64)
			if ok && typedValue <= conditionValue {
				solvedDevice, err := deviceInfoToExtendedSolvedDevice(info, propertyPath, value)
				if err != nil {
					log.Fatal(err)
				}
				result[info.Id()] = solvedDevice
				break
			}
		}
	}
	return result
}

func applyPropertyConditionIntGreater(propertyPath string, conditionValue int64, deviceInfo []deviceinfo.TargetVariant) map[deviceinfo.TargetId]*ExtendedSolvedDevice {
	result := map[deviceinfo.TargetId]*ExtendedSolvedDevice{}
	for _, info := range deviceInfo {
		Values := info.Properties.PropertiesDetails[propertyPath].Values
		for _, value := range Values {
			typedValue, ok := value.(int64)
			if ok && typedValue > conditionValue {
				solvedDevice, err := deviceInfoToExtendedSolvedDevice(info, propertyPath, value)
				if err != nil {
					log.Fatal(err)
				}
				result[info.Id()] = solvedDevice
				break
			}
		}
	}
	return result
}

func applyPropertyConditionIntGreaterOrEqual(propertyPath string, conditionValue int64, deviceInfo []deviceinfo.TargetVariant) map[deviceinfo.TargetId]*ExtendedSolvedDevice {
	result := map[deviceinfo.TargetId]*ExtendedSolvedDevice{}
	for _, info := range deviceInfo {
		Values := info.Properties.PropertiesDetails[propertyPath].Values
		for _, value := range Values {
			typedValue, ok := value.(int64)
			if ok && typedValue > conditionValue {
				solvedDevice, err := deviceInfoToExtendedSolvedDevice(info, propertyPath, value)
				if err != nil {
					log.Fatal(err)
				}
				result[info.Id()] = solvedDevice
				break
			}
		}
	}
	return result
}

func applyPropertyConditionIntSet(propertyPath string, conditionValues []int64, deviceInfo []deviceinfo.TargetVariant) map[deviceinfo.TargetId]*ExtendedSolvedDevice {
	valuesMap := map[int64]interface{}{}
	for _, s := range conditionValues {
		valuesMap[s] = nil
	}
	result := map[deviceinfo.TargetId]*ExtendedSolvedDevice{}
	for _, info := range deviceInfo {
		Values := info.Properties.PropertiesDetails[propertyPath].Values
		for _, value := range Values {
			typedValue, ok := value.(int64)
			if !ok {
				continue
			}
			if _, present := valuesMap[typedValue]; present {
				solvedDevice, err := deviceInfoToExtendedSolvedDevice(info, propertyPath, value)
				if err != nil {
					log.Fatal(err)
				}
				result[info.Id()] = solvedDevice
				break
			}
		}
	}
	return result
}

func handleLargeExpression(count int) error {
	if count > LARGECLASSES {
		errString := fmt.Sprintf("Number of solved expressions is very large: %v. Every 100k will take 2 seconds on a P920 to solve.", count)
		// Set text in red; then reset the color to white.
		fmt.Println(fmt.Sprintf("\x1b[%dm%s\x1b[0m", 31, errString))
		fmt.Printf("Your estimated solution time is %v minutes\n", (count / 45000 / 60))
		fmt.Println("If your seeing this; consider re-thinking your expression.")

		if count > CONTAINERMAX {
			if _, err := os.Stat("/usr/local/container"); err == nil {
				log.Println("Expression is too large for the service to solve. Exiting.")
				return fmt.Errorf("Expression too large for solving in the container with: %v number of expressions (Max 500K acceptable.)", count)
			}
			if StringPrompt() != "y" {
				return fmt.Errorf("Exiting")
			}
		}
	}
	return nil

}

func StringPrompt() string {
	var s string
	r := bufio.NewReader(os.Stdin)
	for {
		fmt.Fprint(os.Stderr, "Do you wish to continue? (y/n): ")
		s, _ = r.ReadString('\n')
		if s != "" {
			break
		}
	}
	ans := strings.TrimSpace(strings.ToLower(s))
	if ans != "y" && ans != "n" {
		fmt.Println("confirm with explicit y/n")
		return StringPrompt()
	}
	return ans
}

func mergeSolvedDevices(devices []*ExtendedSolvedDevice) (*ExtendedSolvedDevice, error) {
	mergedLabels := []*ttcpSolver.SwarmingLabel{}
	if len(devices) == 0 {
		return nil, errors.NewError("Cannot merge empty array of solved devices.")
	}
	deviceId := devices[0].solvedDevice.DeviceId
	imageId := devices[0].solvedDevice.ImageId
	for _, device := range devices {
		if deviceId != device.solvedDevice.DeviceId || imageId != device.solvedDevice.ImageId {
			return nil, errors.NewError("Cannot merge solved devices with different device or image ids.")
		}
		mergedLabels = append(mergedLabels, device.swarmingLabels[:]...)
	}
	return &ExtendedSolvedDevice{
		solvedDevice:   devices[0].solvedDevice,
		board:          devices[0].board,
		model:          devices[0].model,
		imageVariant:   devices[0].imageVariant,
		swarmingLabels: mergedLabels,
	}, nil
}

func getModelValue(info deviceinfo.TargetVariant) (string, error) {
	model, ok := info.Properties.PropertiesDetails["dlm:model"]
	if !ok {
		if model, ok = info.Properties.PropertiesDetails["project"]; !ok {
			return "", errors.NewErrorf("The device is missing the property \"project\". DeviceProperties:%s", info)
		}
	}
	modelVal, err := model.GetSingleStringValue()
	if err != nil {
		return "", err
	}
	return strings.ToLower(modelVal), nil
}

func filterSwarmingDevices(
	extendedDevicesInfo map[deviceinfo.TargetId]*ExtendedSolvedDevice,
	logger *log.Logger,
	pool string) map[deviceinfo.TargetId]*ExtendedSolvedDevice {

	filteredDevs := map[deviceinfo.TargetId]*ExtendedSolvedDevice{}
	swarmData := getInventoryByHWID(pool, logger)

	for targetId, deviceInfo := range extendedDevicesInfo {
		// Add to filtered list and proceed to next iteration when no swarming labels exist on target
		if len(deviceInfo.swarmingLabels) == 0 {
			filteredDevs[targetId] = deviceInfo
			continue
		}
		deviceId := deviceInfo.solvedDevice.DeviceId
		// Skip iteration when HWID does not exist in the inventory
		if _, ok := swarmData[deviceId]; !ok {
			continue
		}
		// Iterate through inventory devices and find matching swarming sets on the target device
		for _, swarmDevice := range swarmData[deviceId] {
			swarmPropVals := swarmDevice.GetFlatPropValMap()
			// Flag for found inventory device
			foundDevice := false
			for i, targetLabel := range deviceInfo.swarmingLabels {
				labelValId := targetLabel.Label + ":" + targetLabel.Value
				// Stop search on current inventory device compare as soon as label set doesn't match
				if _, ok := swarmPropVals[labelValId]; !ok {
					break
				}
				// At the last label match the of inventory dev compare,
				// add to filtered list and set flag to stop search
				if i == len(deviceInfo.swarmingLabels)-1 {
					filteredDevs[targetId] = deviceInfo
					foundDevice = true
				}
			}
			// Stop comparing on inventory devices if at least one device matches
			if foundDevice {
				break
			}
		}
	}
	return filteredDevs
}

func getInventoryByHWID(pool string, logger *log.Logger) map[string][]swarmingdata.SwarmingdataEntry {
	// Get the lab data which is cached if data on the pool has been queried
	swarmDataResc, err := croslab.GetInventory(pool, logger)
	if err != nil {
		if logger != nil {
			logger.Println("Could not retrieve swarming inventory:", err)
		}
		log.Fatal("Could not retrieve swarming inventory:", err)
	}

	// Transform the lab data into a map of with key HWID
	swarmData, err := swarmDataResc.ParseAsMapPerHwid()
	if err != nil {
		log.Fatal(errors.ApiError(err))
	}
	return swarmData
}

func GetEqcExpressionHash(eqcClass ttcpSolver.SolvedClass) uint64 {
	exp := eqcClass.GetExpression()
	eqcHash, err := hashstructure.Hash(exp, hashstructure.FormatV2, nil)
	if err != nil {
		log.Println(fmt.Sprintf("error while creating hash for eqc: %s", err))
	}
	return eqcHash
}
