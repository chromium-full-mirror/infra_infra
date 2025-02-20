// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package cli

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/maruel/subcommands"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"

	"go.chromium.org/luci/common/data/text"
	"go.chromium.org/luci/common/errors"

	"go.chromium.org/infra/tools/dirmd"
	dirmdpb "go.chromium.org/infra/tools/dirmd/proto"
)

func cmdParse() *subcommands.Command {
	return &subcommands.Command{
		UsageLine: `parse [file1 [file2]...]`,
		ShortDesc: "parse metadata files or content from stdin",
		LongDesc: text.Doc(`
			Parse metadata files.

			The positional arguments must be paths to the files.
			A valid file has a base filename "DIR_METADATA" or "OWNERS".
			The format of its contents correspond to the base name.

			The output format is JSON like below:
			 {
				// The content of file_1 is converted to chrome.dir_meta.Metadata protobuf
				// in JSON format.
				"file_1":{"json":{"monorail":{"component":"Internals\u003eNetwork\u003eDataProxy"}}},
				// dirmd has issue parsing the file.
				"file_2":{"error":"file_2 not exist"},
			 }

			If parsing data from stdin, by default the content should be in DIR_METADATA format.
			Parsing content in OWNERS format, please specify the format, like "dirmd parse -format owners".

			The subcommand returns a non-zero exit code if any of the files is
			invalid.
		`),
		CommandRun: func() subcommands.CommandRun {
			r := &parseRun{}
			r.RegisterBaseFlags()
			r.Flags.StringVar(&r.formatString, "format", "dir-metadata", text.Doc(`
				The format of the input from stdin.
				Valid values: "owners", "dir-metadata".
			`))
			return r
		},
	}
}

type parseRun struct {
	baseCommandRun

	formatString string
}

// parseFiles parses the metadata files and convert them to dirmdpb.Metadata.
func (r *parseRun) parseFiles(args []string) ([]*dirmdpb.Metadata, []string) {
	mds := make([]*dirmdpb.Metadata, len(args))
	errMsgs := make([]string, len(args))
	for i, fileName := range args {
		origFileName := fileName
		var err error
		if fileName, err = canonicalFSPath(fileName); err != nil {
			errMsgs[i] = fmt.Sprintf("%s: failed to canonicalize: %s", origFileName, err)
			continue
		}

		md, err := dirmd.ParseFile(fileName)
		if err != nil {
			errMsgs[i] = fmt.Sprintf("%s: %s", fileName, err)
			continue
		}
		mds[i] = md
	}
	return mds, errMsgs
}

// parseFiles parses the metadata from stdin and convert it to dirmdpb.Metadata.
// It only accepts content in dir_metadata format, contents in owners format will
// fail.
func (r *parseRun) parseStdin() (*dirmdpb.Metadata, string) {
	md := &dirmdpb.Metadata{}
	if r.formatString == "dir-metadata" {
		content, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			return nil, err.Error()
		}

		if err = prototext.Unmarshal(content, md); err != nil {
			return nil, err.Error()
		}
	} else {
		var err error
		if md, _, err = dirmd.ParseOwners(os.Stdin); err != nil {
			return nil, err.Error()
		}
	}

	return md, ""
}

func (r *parseRun) printAsJson(mds []*dirmdpb.Metadata, errMsgs, files []string) int {
	res := make(map[string]map[string]interface{})
	exitCode := 0
	for i := 0; i < len(files); i++ {
		md := mds[i]
		errMsg := errMsgs[i]
		file := files[i]
		switch {
		case md == nil && errMsg == "":
			panic(fmt.Sprintf("impossible: no result on %s", file))
		case md == nil:
			res[file] = map[string]interface{}{"error": errMsg}
			exitCode = 1
		default:
			data, err := protojson.Marshal(md)
			if err != nil {
				res[file] = map[string]interface{}{"error": err.Error()}
				exitCode = 1
			}
			res[file] = map[string]interface{}{"json": json.RawMessage(data)}
		}
	}

	js, err := json.Marshal(res)
	if err != nil {
		fmt.Println(err)
		return 1
	}
	if err = r.writeTextOutput(js); err != nil {
		fmt.Println(err)
		return 1
	}
	return exitCode
}

func (r *parseRun) validateFlags() error {
	if r.formatString != "dir-metadata" && r.formatString != "owners" {
		return errors.Reason("unexpected format %s; expected dir-metadata or owners", r.formatString).Err()
	}
	return nil
}

func (r *parseRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	if err := r.validateFlags(); err != nil {
		fmt.Println(err)
		return 1
	}

	exitCode := 0
	if len(args) > 0 {
		mds, errMsgs := r.parseFiles(args)
		exitCode = r.printAsJson(mds, errMsgs, args)
	} else {
		md, errStr := r.parseStdin()
		exitCode = r.printAsJson([]*dirmdpb.Metadata{md}, []string{errStr}, []string{"stdin"})
	}
	return exitCode
}
