// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package db

import (
	"fmt"
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

func loadProjectToBoardIndex(projectsFile string) map[string]string {
	data, err := os.ReadFile(projectsFile)
	if err != nil {
		log.Fatal("could not read hwid model to board map:", projectsFile)
	}

	parsed_data := map[any]any{}

	err = yaml.Unmarshal(data, &parsed_data)
	if err != nil {
		log.Fatal("Could not parse the file ", projectsFile, " as yaml:", err)
	}
	return extractProjectToBoard(parsed_data)
}

func extractProjectToBoard(parsed_data map[any]any) map[string]string {
	result := map[string]string{}

	for project, details := range parsed_data {
		projectName := project.(string)
		if details == nil {
			fmt.Println("empty details for project ", project)
			continue
		}
		if detailsMap, ok := details.(map[string]any); ok {
			if boardValue, ok := detailsMap["board"]; ok {
				if boardName, ok := boardValue.(string); ok {
					result[projectName] = boardName
				} else {
					fmt.Println("board is a string:", detailsMap)
				}
			} else {
				fmt.Println("no board entry in project index details:", details)
			}
		} else {
			fmt.Println("details is not a map but instead:", details)
		}

	}
	return result
}
