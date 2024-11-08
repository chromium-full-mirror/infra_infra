// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/ulikunitz/xz"
)

func TestDownloadHandler(t *testing.T) {
	t.Parallel()
	gsa := &archiveServer{
		gsClient: &fakeGSClient{
			objects: map[string]*fakeGSObject{
				"bucket/path/to/file": {
					attrs: &storage.ObjectAttrs{
						Size:        99,
						ContentType: "text",
						MD5:         []byte("randomHashString"),
						CRC32C:      uint32(1984),
					},
					content: "this is the content",
				},
			},
		},
	}
	tests := []struct {
		method            string
		url               string
		contentRange      string
		wantStatusCode    int
		wantContentLength int
		wantBody          string
		wantContentType   string
		wantMD5           string
		wantCRC32C        string
	}{
		{
			method:            "GET",
			url:               "/download/bucket/path/to/file",
			wantStatusCode:    200,
			wantContentLength: 99,
			wantBody:          "this is the content",
			wantContentType:   "text",
			wantMD5:           base64.StdEncoding.EncodeToString([]byte("randomHashString")),
			wantCRC32C:        "AAAHwA==",
		},
		{
			method:            "GET",
			url:               "/download/wrong-bucket/path/to/file",
			wantStatusCode:    404,
			wantBody:          "GET/download/wrong-bucket/path/to/file  swarming_task_id= bbid= Obj \"path/to/file\": storage: object doesn't exist\n",
			wantContentLength: -1,
			wantContentType:   "text/plain; charset=utf-8",
		},
		{
			contentRange:      "bytes=0-3",
			method:            "GET",
			url:               "/download/bucket/path/to/file",
			wantStatusCode:    206,
			wantContentLength: 4,
			wantBody:          "this",
			wantContentType:   "text",
			wantMD5:           base64.StdEncoding.EncodeToString([]byte("randomHashString")),
			wantCRC32C:        "AAAHwA==",
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.method+" "+tc.url, func(t *testing.T) {
			t.Parallel()
			r := httptest.NewRequest(tc.method, tc.url, strings.NewReader(""))
			r.Header.Set("Range", tc.contentRange)
			w := httptest.NewRecorder()
			gsa.downloadHandler(w, r)
			got := w.Result()
			if got.StatusCode != tc.wantStatusCode {
				t.Errorf("%s, %s downloadHandler StatusCode = %d, want %d", tc.method, tc.url, got.StatusCode, tc.wantStatusCode)
			}
			if got.ContentLength != int64(tc.wantContentLength) {
				t.Errorf("%s, %s downloadHandler ContentLength = %d, want %d", tc.method, tc.url, got.ContentLength, tc.wantContentLength)
			}
			if gotContentType := got.Header.Get("Content-Type"); gotContentType != tc.wantContentType {
				t.Errorf("%s, %s downloadHandler Content-Type = %q, want %q", tc.method, tc.url, gotContentType, tc.wantContentType)
			}
			if gotMD5 := got.Header.Get("Content-Hash-MD5"); gotMD5 != tc.wantMD5 {
				t.Errorf("%s, %s downloadHandler Content-Hash-MD5 = %q, want %q", tc.method, tc.url, gotMD5, tc.wantMD5)
			}
			if gotCRC32C := got.Header.Get("Content-Hash-CRC32C"); gotCRC32C != tc.wantCRC32C {
				t.Errorf("%s, %s downloadHandler Content-Hash-CRC32C = %q, want %q", tc.method, tc.url, gotCRC32C, tc.wantCRC32C)
			}
			body, err := io.ReadAll(got.Body)
			if err != nil {
				t.Errorf("err = %s, want nil", err.Error())
			}
			if b := string(body); b != tc.wantBody {
				t.Errorf("Body = %q, want %q", b, tc.wantBody)
			}
		})
	}
}

func TestExtracHandler(t *testing.T) {
	t.Parallel()

	fakeObjects := map[string]*fakeGSObject{}
	gsa := &archiveServer{
		gsClient: &fakeGSClient{
			objects: fakeObjects,
		},
		httpClient: http.DefaultClient,
	}

	tarFiles := map[string]map[string]string{
		"bucket2/extract.tar": {
			"f1.txt": "this is bucket2 file1",
			"f2.txt": "this is bucket2 file2",
			"f3.txt": "this is bucket2 file3",
		},
		"bucket/extract.tar": {
			"f1.txt": "this is bucket file1",
			"f2.txt": "this is bucket file2",
			"f3.txt": "this is bucket file3. Hi\nHow\nAre\nYou\n",
		},
	}

	for tarName, tarContent := range tarFiles {
		var buf bytes.Buffer
		tw := tar.NewWriter(&buf)
		defer tw.Close()

		for name, content := range tarContent {
			hdr := &tar.Header{
				Name: name,
				Mode: 0600,
				Size: int64(len(content)),
			}
			if err := tw.WriteHeader(hdr); err != nil {
				t.Fatalf("ExtractHandler error writing header: %s", err)
			}
			if _, err := tw.Write([]byte(content)); err != nil {
				t.Fatalf("ExtractHadnler error writing content: %s", err)
			}
		}
		fakeObjects[tarName] = &fakeGSObject{
			attrs: &storage.ObjectAttrs{
				Size:        int64(len(buf.String())),
				ContentType: "tar",
			},
			content: buf.String(),
		}

		gzTarName := tarName[:len(tarName)-3] + "tgz"
		var zbuf bytes.Buffer
		func() {
			w := gzip.NewWriter(&zbuf)
			defer w.Close()
			if _, err := w.Write(buf.Bytes()); err != nil {
				t.Fatalf("error writing %s content %s: %s", gzTarName, buf.Bytes(), err)
			}
		}()
		fakeObjects[gzTarName] = &fakeGSObject{
			attrs: &storage.ObjectAttrs{
				Size: int64(len(zbuf.String())),
				//ContentType: "tgz",
			},
			content: zbuf.String(),
		}

	}

	mux := http.NewServeMux()
	mux.HandleFunc("/extract/", gsa.extractHandler)
	mux.HandleFunc("/download/", gsa.downloadHandler)
	mux.HandleFunc("/decompress/", gsa.decompressHandler)
	s := httptest.NewServer(mux)
	defer s.Close()
	gsa.cacheServerURL = s.URL

	tests := []struct {
		url               string
		wantStatusCode    int
		wantContentLength int64
		wantBody          string
	}{
		{
			url:               "/extract/bucket/extract.tar?file=f1.txt",
			wantStatusCode:    200,
			wantContentLength: int64(len(tarFiles["bucket/extract.tar"]["f1.txt"])),
			wantBody:          tarFiles["bucket/extract.tar"]["f1.txt"],
		},
		{
			url:               "/extract/bucket/extract.tgz?file=f1.txt",
			wantStatusCode:    200,
			wantContentLength: int64(len(tarFiles["bucket/extract.tar"]["f1.txt"])),
			wantBody:          tarFiles["bucket/extract.tar"]["f1.txt"],
		},
		{
			url:               "/extract/bucket/extract.tar?file=f2.txt",
			wantStatusCode:    200,
			wantContentLength: int64(len(tarFiles["bucket/extract.tar"]["f2.txt"])),
			wantBody:          tarFiles["bucket/extract.tar"]["f2.txt"],
		},
		{
			url:               "/extract/bucket/extract.tgz?file=f2.txt",
			wantStatusCode:    200,
			wantContentLength: int64(len(tarFiles["bucket/extract.tar"]["f2.txt"])),
			wantBody:          tarFiles["bucket/extract.tar"]["f2.txt"],
		},
		{
			url:               "/extract/bucket/extract.tar?file=f3.txt",
			wantStatusCode:    200,
			wantContentLength: int64(len(tarFiles["bucket/extract.tar"]["f3.txt"])),
			wantBody:          tarFiles["bucket/extract.tar"]["f3.txt"],
		},
		{
			url:               "/extract/bucket/extract.tgz?file=f3.txt",
			wantStatusCode:    200,
			wantContentLength: int64(len(tarFiles["bucket/extract.tar"]["f3.txt"])),
			wantBody:          tarFiles["bucket/extract.tar"]["f3.txt"],
		},
		{
			url:               "/extract/bucket2/extract.tar?file=f1.txt",
			wantStatusCode:    200,
			wantContentLength: int64(len(tarFiles["bucket2/extract.tar"]["f1.txt"])),
			wantBody:          tarFiles["bucket2/extract.tar"]["f1.txt"],
		},
		{
			url:               "/extract/bucket2/extract.tgz?file=f1.txt",
			wantStatusCode:    200,
			wantContentLength: int64(len(tarFiles["bucket2/extract.tar"]["f1.txt"])),
			wantBody:          tarFiles["bucket2/extract.tar"]["f1.txt"],
		},
		{
			url:               "/extract/bucket2/extract.tar?file=f2.txt",
			wantStatusCode:    200,
			wantContentLength: int64(len(tarFiles["bucket2/extract.tar"]["f2.txt"])),
			wantBody:          tarFiles["bucket2/extract.tar"]["f2.txt"],
		},
		{
			url:               "/extract/bucket2/extract.tgz?file=f2.txt",
			wantStatusCode:    200,
			wantContentLength: int64(len(tarFiles["bucket2/extract.tar"]["f2.txt"])),
			wantBody:          tarFiles["bucket2/extract.tar"]["f2.txt"],
		},
		{
			url:               "/extract/bucket2/extract.tar?file=f3.txt",
			wantStatusCode:    200,
			wantContentLength: int64(len(tarFiles["bucket2/extract.tar"]["f3.txt"])),
			wantBody:          tarFiles["bucket2/extract.tar"]["f3.txt"],
		},
		{
			url:               "/extract/bucket2/extract.tgz?file=f3.txt",
			wantStatusCode:    200,
			wantContentLength: int64(len(tarFiles["bucket2/extract.tar"]["f3.txt"])),
			wantBody:          tarFiles["bucket2/extract.tar"]["f3.txt"],
		},
	}

	for _, tc := range tests {
		tc := tc
		got, err := http.Get(fmt.Sprintf("%s%s", s.URL, tc.url))
		t.Run(tc.url, func(t *testing.T) {
			t.Parallel()
			if err != nil {
				t.Fatalf("extractHandler http.Get(%s) failed unexpectedly. err=%s", tc.url, err)
			}
			defer got.Body.Close()

			if got.StatusCode != tc.wantStatusCode {
				t.Errorf("extractHandler StatusCode=%v, want %v", got.StatusCode, tc.wantStatusCode)
			}
			if got.ContentLength != tc.wantContentLength {
				t.Errorf("extractHandler ContentLength=%v, want %v", got.ContentLength, tc.wantContentLength)
			}

			gotRead, err := io.ReadAll(got.Body)
			if err != nil {
				t.Fatalf("extractHandler %s read body failed unexpectedly. err=%s", tc.url, err)
			}
			if gotBody := string(gotRead); gotBody != tc.wantBody {
				t.Errorf("extractHanlder Body=%s, want %s", gotBody, tc.wantBody)
			}
		})
	}
}

func TestWriteHeaderAndStatusOK(t *testing.T) {
	t.Parallel()
	objAttrs := &storage.ObjectAttrs{
		ContentType: "fake-type",
		Size:        2048,
		MD5:         []byte("hi there"),
		CRC32C:      uint32(123),
	}
	w := httptest.NewRecorder()
	writeHeaderAndStatusOK(objAttrs, nil, w, "")
	wantResponseWriterHeaders := []struct {
		header string
		want   string
	}{
		{"Content-Type", "fake-type"},
		{"Content-Length", "2048"},
		{"Accept-Ranges", "bytes"},
		{"Content-Hash-MD5", base64.StdEncoding.EncodeToString([]byte("hi there"))},
		{"Content-Hash-CRC32C", convertCRC32CToString(123)},
	}
	for _, tc := range wantResponseWriterHeaders {
		t.Run(tc.header, func(t *testing.T) {
			if got := w.Header().Get(tc.header); got != tc.want {
				t.Errorf("writeHeaderResponse %q=%q, want %q", tc.header, got, tc.want)
			}
		})
	}
}

func TestParseURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		url        string
		wantBucket string
		wantObject string
	}{
		{
			url:        "/download/bucket/path/to/file",
			wantBucket: "bucket",
			wantObject: "path/to/file",
		},
		{
			url:        "/download/fake-bucket/path/to/fake-file.txt",
			wantBucket: "fake-bucket",
			wantObject: "path/to/fake-file.txt",
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.url, func(t *testing.T) {
			t.Parallel()
			got, err := parseURL(tc.url)
			if err != nil {
				t.Fatalf("parseURL(%v) failed unexpectedly; err=%s", tc.url, err)
			}
			if tc.wantBucket != got.bucket || tc.wantObject != got.path {
				t.Errorf("parseURL(%v) = (%q, %q), want (%q, %q)", tc.url, got.bucket, got.path, tc.wantBucket, tc.wantObject)
			}
		})
	}
}

func TestParseURLErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		url        string
		wantErrMsg string
	}{
		{
			url:        "/download/bucket",
			wantErrMsg: "the URL doesn't have all of RPC, bucket and object path",
		},
		{
			url:        "/download/",
			wantErrMsg: "the URL doesn't have all of RPC, bucket and object path",
		},
		{
			url:        "/download/bucket/",
			wantErrMsg: "object cannot be empty",
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.url, func(t *testing.T) {
			t.Parallel()
			_, err := parseURL(tc.url)
			if err == nil || tc.wantErrMsg != err.Error() {
				t.Errorf("parseURL(%v) error got: %v, want %v", tc.url, err, tc.wantErrMsg)
			}
		})
	}
}

func TestDecompressGZIPHandler(t *testing.T) {
	t.Parallel()
	compressFiles := map[string]string{
		"bucket/f1.gz": "this is file1\n",
		"bucket/f2.gz": "this is file2",
	}

	objects := map[string]*fakeGSObject{}
	gsa := &archiveServer{
		gsClient: &fakeGSClient{
			objects: objects,
		},
		httpClient: http.DefaultClient,
	}
	for fName, fContent := range compressFiles {
		var buf bytes.Buffer
		func() {
			w := gzip.NewWriter(&buf)
			defer w.Close()
			if _, err := w.Write([]byte(fContent)); err != nil {
				t.Fatalf("error writing %s content %s: %s", fName, fContent, err)
			}
		}()
		objects[fName] = &fakeGSObject{
			attrs: &storage.ObjectAttrs{
				Size:        int64(len(buf.String())),
				ContentType: "gzip",
			},
			content: buf.String(),
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/decompress/", gsa.decompressHandler)
	mux.HandleFunc("/download/", gsa.downloadHandler)
	s := httptest.NewServer(mux)
	defer s.Close()
	gsa.cacheServerURL = s.URL
	tests := []struct {
		url               string
		wantStatusCode    int
		wantContentLength int64
		wantBody          string
	}{
		{
			url:               "/decompress/bucket/f1.gz",
			wantStatusCode:    200,
			wantContentLength: int64(len(compressFiles["bucket/f1.gz"])),
			wantBody:          compressFiles["bucket/f1.gz"],
		},
	}
	for _, tc := range tests {
		tc := tc
		got, err := http.Get(fmt.Sprintf("%s%s", s.URL, tc.url))
		if err != nil {
			t.Fatalf("decompress gzip http.Get(%s) failed unexpectedly. err=%s", tc.url, err)
		}
		t.Run(tc.url, func(t *testing.T) {
			t.Parallel()
			defer got.Body.Close()
			if got.StatusCode != tc.wantStatusCode {
				t.Errorf("decompress gzip StatusCode=%v, want %v", got.StatusCode, tc.wantStatusCode)
			}
			if got.ContentLength != tc.wantContentLength {
				t.Errorf("decompress gzip ContentLength=%v, want %v", got.ContentLength, tc.wantContentLength)
			}

			gotRead, err := io.ReadAll(got.Body)
			if err != nil {
				t.Fatalf("decompress gzip %s read body failed unexpectedly. err=%s", tc.url, err)
			}
			if gotBody := string(gotRead); gotBody != tc.wantBody {
				t.Errorf("decompress gzip Body=%s, want %s", gotBody, tc.wantBody)
			}
		})
	}
}

func TestDecompressXZHandler(t *testing.T) {
	t.Parallel()
	compressFiles := map[string]string{
		"bucket/f1.xz": "this is file1",
		"bucket/f2.xz": "this is file2",
	}

	objects := map[string]*fakeGSObject{}
	gsa := &archiveServer{
		gsClient: &fakeGSClient{
			objects: objects,
		},
		httpClient: http.DefaultClient,
	}
	for fName, fContent := range compressFiles {
		var buf bytes.Buffer
		func() {
			w, err := xz.NewWriter(&buf)
			defer w.Close()
			if err != nil {
				t.Fatalf("xz NewWriter error: %s", err)
			}
			if _, err := w.Write([]byte(fContent)); err != nil {
				t.Fatalf("writing %s content %s error: %s", fName, fContent, err)
			}
		}()
		objects[fName] = &fakeGSObject{
			attrs: &storage.ObjectAttrs{
				Size:        int64(buf.Len()),
				ContentType: "xz",
			},
			content: buf.String(),
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/decompress/", gsa.decompressHandler)
	mux.HandleFunc("/download/", gsa.downloadHandler)
	s := httptest.NewServer(mux)
	defer s.Close()
	gsa.cacheServerURL = s.URL
	tests := []struct {
		url               string
		wantStatusCode    int
		wantContentLength int64
		wantBody          string
	}{
		{
			url:               "/decompress/bucket/f1.xz",
			wantStatusCode:    200,
			wantContentLength: int64(len(compressFiles["bucket/f1.xz"])),
			wantBody:          compressFiles["bucket/f1.xz"],
		},
	}
	for _, tc := range tests {
		tc := tc
		url := fmt.Sprintf("%s%s", s.URL, tc.url)
		got, err := http.Get(url)
		if err != nil {
			t.Fatalf("decompress xz http.Get(%s) failed unexpectedly. err=%s", url, err)
		}
		t.Run(tc.url, func(t *testing.T) {
			t.Parallel()
			defer got.Body.Close()
			if got.StatusCode != tc.wantStatusCode {
				t.Errorf("decompress xz StatusCode=%v, want %v", got.StatusCode, tc.wantStatusCode)
			}
			if got.ContentLength != tc.wantContentLength {
				t.Errorf("decompress xz ContentLength=%v, want %v", got.ContentLength, tc.wantContentLength)
			}

			gotRead, err := io.ReadAll(got.Body)
			if err != nil {
				t.Fatalf("decompress xz %s read body failed unexpectedly. err=%s", url, err)
			}
			if gotBody := string(gotRead); gotBody != tc.wantBody {
				t.Errorf("decompress xz Body=%s, want %s", gotBody, tc.wantBody)
			}
		})
	}
}

type fakeGSObject struct {
	attrs   *storage.ObjectAttrs
	content string
	state   int
}

const (
	objStateNormal int = iota
	objStateNotExist
)

func (c *fakeGSObject) Attrs(ctx context.Context) (*storage.ObjectAttrs, error) {
	m := map[int]error{
		objStateNotExist: storage.ErrObjectNotExist,
	}
	if err, ok := m[c.state]; ok {
		return nil, err
	}
	return c.attrs, nil
}

func (c *fakeGSObject) NewReader(ctx context.Context) (io.ReadCloser, error) {
	if c.state == objStateNotExist {
		return nil, fmt.Errorf("storage: object doesn't exist")
	}
	return io.NopCloser(strings.NewReader(c.content)), nil
}

func (c *fakeGSObject) NewRangeReader(ctx context.Context, offset, length int64) (io.ReadCloser, error) {
	if c.state == objStateNotExist {
		return nil, fmt.Errorf("storage: object doesn't exist")
	}
	return io.NopCloser(strings.NewReader(c.content[offset : offset+length])), nil
}

type fakeGSClient struct {
	objects map[string]*fakeGSObject
}

func (c *fakeGSClient) getObject(name *storageObjectName) storageObject {
	key := name.bucket + "/" + name.path
	if v, ok := c.objects[key]; ok {
		return v
	}
	return &fakeGSObject{state: objStateNotExist}
}

func (*fakeGSClient) close() error {
	return nil
}

func TestAndroidObjectNonExisting(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(404)
		_, _ = rw.Write([]byte(`{"error": {"code": 404}}`))
	}))
	defer server.Close()
	client := &androidClient{hc: server.Client(), server: server.URL}
	obj := &androidObject{client: client, path: "url"}
	_, err := obj.Attrs(context.Background())
	if err == nil {
		t.Errorf("androidObject.Attrs(non-exist) got nil, want not nil")
	}
}

func TestAndroidObject(t *testing.T) {
	t.Parallel()

	filename := "filename"
	content := "file content"
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		url := req.URL.String()
		if strings.HasSuffix(url, "?alt=media") {
			_, _ = rw.Write([]byte(content))
		} else {
			_, _ = rw.Write([]byte(fmt.Sprintf(`{"name": %q, "size": "%d", "contentType": "application/octet-stream"}`, filename, len(content))))
		}
	}))
	defer server.Close()

	client := &androidClient{hc: server.Client(), server: server.URL}
	obj := &androidObject{client: client, path: "url"}
	ctx := context.Background()
	attr, err := obj.Attrs(ctx)
	if err != nil {
		t.Errorf("androidObject.Attrs() got %s, want nil", err)
	}
	if attr.Name != filename {
		t.Errorf("androidObject.Attrs() = %s, want %q", attr.Name, filename)
	}
	if l := int64(len(content)); attr.Size != l {
		t.Errorf("androidObject.Attrs() = %d, want %q", attr.Size, l)
	}
	rc, err := obj.NewReader(ctx)
	if err != nil {
		t.Errorf("androidObject.NewReader() got %s, want nil", err)
	}
	if c, _ := io.ReadAll(rc); string(c) != content {
		t.Errorf("androidObject.NewReader() = %q, want %q", c, content)
	}
}
