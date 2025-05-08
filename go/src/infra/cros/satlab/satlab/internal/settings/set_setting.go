// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package settings

import (
	"fmt"
	"reflect"
	"strconv"

	"github.com/maruel/subcommands"
)

// SetSetting is the command to change user settings.
var SetSetting = &subcommands.Command{
	UsageLine: "set",
	ShortDesc: "set user setting",
	LongDesc:  "Set user setting",
	CommandRun: func() subcommands.CommandRun {
		return &setSetting{}
	},
}

// setSetting struct contains the arguments needed to run SetSetting.
type setSetting struct {
	subcommands.CommandRunBase
}

// Run is what is called when a user inputs the setSetting command.
func (c *setSetting) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := c.innerRun(a, args, env); err != nil {
		fmt.Fprintf(a.GetErr(), "%s: %s\n", a.GetName(), err)
		return 1
	}
	return 0
}

// innerRun contains business logic.
func (c *setSetting) innerRun(a subcommands.Application, args []string, env subcommands.Env) error {
	if len(args) != 2 {
		return fmt.Errorf("incorrect arguments - expected key and value")
	}
	key, value := args[0], args[1]

	settings, err := readSettingsFromFile()
	if err != nil {
		return fmt.Errorf("unmarshalling JSON: %w", err)
	}

	newValue, err := parseValue(settings[key], value)
	if err != nil {
		return err
	}

	fmt.Println("Restart your device for the changes to take effect.")
	return settings.updateSetting(key, newValue)
}

func parseValue(originalValue any, newValue string) (any, error) {
	switch reflect.TypeOf(originalValue).Kind() {
	case reflect.Bool:
		return strconv.ParseBool(newValue)
	case reflect.Int:
		return strconv.Atoi(newValue)
	case reflect.Float64:
		return strconv.ParseFloat(newValue, 64)
	case reflect.String:
		return newValue, nil
	default:
		return nil, fmt.Errorf("unsupported type for key '%v'", originalValue)
	}
}

// updateSetting updates an existing setting based on the provided key and value.
func (c Settings) updateSetting(key string, newValue any) error {
	c[key] = newValue

	if err := writeSettingsToFile(c); err != nil {
		return fmt.Errorf("unexpected error occurred while updating the value")
	}

	return nil
}
