// Copyright 2025 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"go.chromium.org/luci/server"
	"go.chromium.org/luci/server/cron"
	"go.chromium.org/luci/server/module"
	"go.chromium.org/luci/server/router"

	"go.chromium.org/infra/fleet_automation_governor/internal/dataframe"
)

func main() {
	modules := []module.Module{
		cron.NewModuleFromFlags(),
	}
	server.Main(nil, modules, func(srv *server.Server) error {
		srv.Routes.POST("/run-task", nil, func(c *router.Context) {
			runTaskHandler(srv.Context, c.Writer, c.Request)
		})
		return nil
	})
}

// handles the /run-task endpoint.
func runTaskHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	// TODO (guocb): placeholder data. Remove them in the real implementation.
	t := dataframe.NewFlatTable([]string{"id", "name"})
	t.AddRow(dataframe.NewRowFromMap(map[string]any{"id": 1, "name": "foo"}))
	t.AddRow(dataframe.NewRowFromMap(map[string]any{"id": 2, "name": "bar"}))

	// Marshal the result to JSON.
	responseJSON, err := json.Marshal(t)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error marshalling JSON response: %v", err), http.StatusInternalServerError)
		return
	}
	// Set the content type and write the response.
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(responseJSON)
}
