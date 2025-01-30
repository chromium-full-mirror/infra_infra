// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

import (
	"reflect"
	"testing"

	testapi "go.chromium.org/chromiumos/config/go/test/api"
)

func TestTranslateRequestCmd(t *testing.T) {
	ta := "resultdb_settings=eyJiYXNkMTJjMjEzLyJ9 " +
		"tast_expr_b64=U1RVQl9TVFJJTkdfVE9fUlVOX1RBU1RfVEVTVFM= " +
		"maybemissingvars_b64=dWlcLihnYWlhUG9vbERlX3R5cGV8c2VydmVyKQ== " +
		"shard_method=hash " +
		"retries=1 " +
		"exe_rel_path=out/Release/chrome " +
		"tast_expr_file=out/Release/bin/chrome_all_tast_tests.filter " +
		"tast_expr_key=default " +
		"max_run_sec=7200 " +
		"run_private_tests=False"

	ctpReq := &testapi.CTPRequest{
		SuiteRequest: &testapi.SuiteRequest{
			TestArgs: ta,
		},
	}
	execMd := executionMetadata(ctpReq, false)
	expectedTestargs := []*testapi.Arg{
		{Flag: "is_partner_run", Value: "false"},
		{Flag: "resultdb_settings", Value: "eyJiYXNkMTJjMjEzLyJ9"},
		{Flag: "tast_expr_b64", Value: "U1RVQl9TVFJJTkdfVE9fUlVOX1RBU1RfVEVTVFM="},
		{Flag: "maybemissingvars_b64", Value: "dWlcLihnYWlhUG9vbERlX3R5cGV8c2VydmVyKQ=="},
		{Flag: "shard_method", Value: "hash"},
		{Flag: "retries", Value: "1"},
		{Flag: "exe_rel_path", Value: "out/Release/chrome"},
		{Flag: "tast_expr_file", Value: "out/Release/bin/chrome_all_tast_tests.filter"},
		{Flag: "tast_expr_key", Value: "default"},
		{Flag: "max_run_sec", Value: "7200"},
		{Flag: "run_private_tests", Value: "False"},
		{Flag: "is_al_run", Value: "false"},
	}

	if len(execMd.Args) != len(expectedTestargs) {
		t.Errorf("execMd.Args do not match expectedTestargs.\nGot: %#v\nExpected: %#v", execMd.Args, expectedTestargs)
	}

	if !reflect.DeepEqual(execMd.Args, expectedTestargs) {
		t.Errorf("execMd.Args do not match expectedTestargs.\nGot: %#v\nExpected: %#v", execMd.Args, expectedTestargs)
	}

}
