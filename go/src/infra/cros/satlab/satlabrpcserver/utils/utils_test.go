// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package utils

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func createFilesHelper(t *testing.T) {
	for i := range 5 {
		fp, err := os.Create(fmt.Sprintf("./temp/%d.data", i))
		if err != nil {
			t.Fatalf("can not create file")
		}

		if _, err = fp.WriteString(fmt.Sprintf("content: %d", i)); err != nil {
			t.Fatalf("can write content")
		}

		fp.Close()
	}
}

func Test_TarFolder(t *testing.T) {
	t.Parallel()

	err := os.Mkdir("./temp", 0755)
	if err != nil {
		t.Fatalf("can not create temp folder")
	}
	// clean up the directory
	defer func() {
		os.RemoveAll("./temp")
		os.Remove("./temp")
	}()

	createFilesHelper(t)

	out := "out.tar.gz"
	err = TarGz("./temp", out)
	defer os.Remove(out)
	if err != nil {
		t.Errorf("can not tar the files")
	}

	f, err := os.Open(out)
	gr, err := gzip.NewReader(f)
	if err != nil {
		t.Errorf("can not create gzip reader")
	}
	tr := tar.NewReader(gr)

	files := []string{}

	for {
		n, err := tr.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			t.Errorf("can not read the header")
			break
		}

		files = append(files, n.Name)
	}

	expected := []string{
		"./temp/0.data",
		"./temp/1.data",
		"./temp/2.data",
		"./temp/3.data",
		"./temp/4.data",
	}

	s := func(a, b string) bool {
		return a < b
	}
	if diff := cmp.Diff(files, expected, cmpopts.SortSlices(s)); diff != "" {
		t.Errorf("unexpected error")
	}
}
