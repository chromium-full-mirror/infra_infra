// Copyright 2022 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// downloader is fleetware cache service tool.
// It serves in between cache server and google storage.
// To start the server:
// 		downloader -address <address:port>
//				   -credential-file <service account credential file>
// After started on the specified address,
// it listens on the specified TCP port.

// The server accepts below requests:
//   - HEAD /download/<bucket>/path/to/file
//     Return only the meta data of file in header.
//   - GET /download/<bucket>/path/to/file
//     Download the file from google storage.
//   - GET /extract/<bucket>/path/to/archive-tar?file=path/to/file
//     Download the archive tar and return specified file.
//   - GET /decompress/<bucket>/path/to/comopressed-file
//     Download the compressed file and return the decompressed data.
//   - GET /mkfs-<FS_type>?file=<bucket>/path/to/file&file=...
//     Download the files specified by parameters and use them to create a file
//     system image in type of <FS_type>.
//     Currently, the FS type supported are: ext2/3/4, squashfs, erofs
package main

import (
	"archive/tar"
	"compress/bzip2"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/pprof"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/ulikunitz/xz"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"

	"infra/libs/otil"
)

var (
	credentialFile       = flag.String("credential-file", "", "credential json file. Example: ./service-credential.json")
	authScopes           = flag.String("auth-scopes", "", "Comma-separated list of OAuth scope for the credential")
	archiveServerAddress = flag.String("address", ":8080", "archive server address with listening port.")
	cacheServerURL       = flag.String("cache-server-url", "http://127.0.0.1:8082", "cache-server url.")
	shutdownGracePeriod  = flag.Duration("shutdown-grace-period", 30*time.Minute, "The time duration allowed for tasks to complete before completely shutdown archive-server.")
	clientRotationPeriod = flag.Duration("client-rotation-period", 24*time.Hour, "The time duration before rotating to new storage client.")
	sourceAddr           = flag.String("use-source-addr", "", "Use the specified source IP addr instead of the one get automatically")
	tsmonEndpoint        = flag.String("tsmon-endpoint", "", "URL (including file://, https://, // pubsub://project/topic) to post monitoring metrics to.")
	tsmonCredentialPath  = flag.String("tsmon-credential", "", "The credentail file for tsmon client")
	traceEndpoint        = flag.String("trace-endpoint", "", "URL (including file://, http://) to post trace logs to.")
	androidBuildAPI      = flag.String("android-build", "", "URL of the Android build API server")
)

type archiveServer struct {
	gsClient       downloadClient
	cacheServerURL string
	httpClient     *http.Client
}

func main() {
	if err := innerMain(); err != nil {
		log.Fatalf("Exiting due to an error: %s", err)
	}
	log.Printf("Exiting successfully")
}

func innerMain() error {
	flag.Parse()
	if a := flag.Args(); len(a) > 0 {
		return fmt.Errorf("unknown command args: %s", a)
	}
	if *clientRotationPeriod < *shutdownGracePeriod {
		return fmt.Errorf("client-rotation-period '%v' cannot be less than shutdown-grace-period '%v'", *clientRotationPeriod, *shutdownGracePeriod)
	}

	ctx := context.Background()

	if err := metricsInit(ctx, *tsmonEndpoint, *tsmonCredentialPath); err != nil {
		log.Printf("metrics init: %s", err)
	}
	defer metricsShutdown(ctx)

	traceInit(ctx, *traceEndpoint)

	hc, err := getHTTPClient(*sourceAddr, http.DefaultClient)
	if err != nil {
		return err
	}
	c := &archiveServer{
		cacheServerURL: *cacheServerURL,
		httpClient:     hc,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/download/", c.downloadHandler)
	mux.HandleFunc("/extract/", c.extractHandler)
	mux.HandleFunc("/decompress/", c.decompressHandler)
	mux.HandleFunc("GET /mkfs-ext2/{$}", c.mkfsHandler)
	mux.HandleFunc("GET /mkfs-ext3/{$}", c.mkfsHandler)
	mux.HandleFunc("GET /mkfs-ext4/{$}", c.mkfsHandler)
	mux.HandleFunc("GET /mkfs-squashfs/{$}", c.mkfsHandler)
	mux.HandleFunc("GET /mkfs-erofs/{$}", c.mkfsHandler)

	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline/", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile/", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol/", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace/", pprof.Trace)

	otelMux := otelhttp.NewHandler(
		mux,
		"caching-backend-downloader",
		otelhttp.WithMessageEvents(otelhttp.ReadEvents, otelhttp.WriteEvents),
	)
	svr := http.Server{Addr: *archiveServerAddress, Handler: otelMux}
	ctx = cancelOnSignals(ctx, &svr, *shutdownGracePeriod)

	opts := []option.ClientOption{option.WithCredentialsFile(*credentialFile)}
	if *authScopes != "" {
		opts = append(opts, option.WithScopes(strings.Split(*authScopes, ",")...))
	}
	if err := c.rotateClient(ctx, *clientRotationPeriod, *shutdownGracePeriod, *androidBuildAPI, opts...); err != nil {
		log.Fatalf("Failed to initiate download client: %s", err)
	}
	defer c.gsClient.close()

	log.Printf("starting archive-server on %s...", *archiveServerAddress)
	if err = svr.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("HTTP server ListenAndServe fatal errror: %v", err)
	}
	<-ctx.Done()
	return err
}

// rotateClient updates client every rotationPeriod. It loads credPath file.
// It will then close the old client after oldClientGracePeriod duration.
func (c *archiveServer) rotateClient(ctx context.Context, rotationPeriod, oldClientGracePeriod time.Duration, androidBuildAPI string, opts ...option.ClientOption) error {
	client, err := newRealClient(ctx, androidBuildAPI, opts...)
	if err != nil {
		return fmt.Errorf("google storage client error: %w", err)
	}
	c.gsClient = client

	go func() {
		var oldClient downloadClient
		t := time.NewTimer(rotationPeriod)
		for {
			select {
			case <-t.C:
				if oldClient == nil {
					gsClient, err := newRealClient(ctx, androidBuildAPI, opts...)
					if err != nil {
						log.Printf("Rotating new client failed, will retry in 10min: %s", err)
						t.Reset(10 * time.Minute)
						continue
					}
					// Update to new client and reset timer for closing old client.
					oldClient = c.gsClient
					c.gsClient = gsClient
					t.Reset(oldClientGracePeriod)
					log.Printf("Rotating to new client succeed")
				} else {
					// Close old client and reset timer for next rotation.
					if err := oldClient.close(); err != nil {
						log.Printf("Error closing old client: %s", err)
					} else {
						log.Printf("Old client closed")
					}
					oldClient = nil
					t.Reset(rotationPeriod)
				}
			case <-ctx.Done():
				if oldClient != nil {
					if err := oldClient.close(); err != nil {
						log.Printf("Error closing old client: %s", err)
					}
				}
				// https://pkg.go.dev/time#Timer.Stop
				if !t.Stop() {
					<-t.C
				}
				return
			}
		}
	}()
	return nil
}

// downloadHandler handles the /download/bucket/path/to/file requests.
// It writes file stat to header for HEAD, GET method.
// It writes file content to body for GET method.
func (c *archiveServer) downloadHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := otil.FuncSpan(r.Context())
	defer func() { otil.EndSpan(span, nil) }()
	startTime := time.Now()

	ctx, cancel := context.WithTimeout(ctx, 60*time.Minute)
	defer cancel()

	md := metricData{}
	defer updateMetrics(ctx, "download", r.Method, &md, startTime)

	id := generateTraceID(r)
	defer func() { log.Printf("%s request completed in %fs", id, time.Since(startTime).Seconds()) }()

	bRange, err := parseRange(r.Header.Get("Range"))
	if err != nil {
		errStr := fmt.Sprintf("%s parseRange error: %s", id, err)
		log.Printf(errStr)
		http.Error(w, errStr, http.StatusBadRequest)
		md.status = http.StatusBadRequest
		return
	}

	log.Printf("%s request started", id)
	gsClient := c.gsClient

	switch r.Method {
	case http.MethodHead:
		_, md, _ = handleDownloadHEAD(ctx, w, r, gsClient, bRange, id)
	case http.MethodGet:
		md = handleDownloadGET(ctx, w, r, gsClient, bRange, id)
	default:
		errStr := fmt.Sprintf("%s unsupported method", id)
		http.Error(w, errStr, http.StatusBadRequest)
		log.Printf(errStr)
		md.status = http.StatusBadRequest
	}
}

// handleDownloadHEAD handles download HEAD request.
// It writes file stat to ResponseWriter.
// It returns storageObject which is used by handleDownloadGET to send file
// content.
func handleDownloadHEAD(ctx context.Context, w http.ResponseWriter, r *http.Request, gsClient downloadClient, br *byteRange, reqID string) (storageObject, metricData, error) {
	objectName, err := parseURL(r.URL.Path)
	if err != nil {
		err := fmt.Errorf("%s parseURL error: %w", reqID, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		log.Printf(err.Error())
		return nil, metricData{status: http.StatusBadRequest}, err
	}

	gsObject := gsClient.getObject(objectName)

	gsAttrs, err := gsObject.Attrs(ctx)
	if err != nil {
		status := deriveHTTPStatusFromError(err)
		err := fmt.Errorf("%s Obj %q: %w", reqID, objectName.path, err)
		http.Error(w, err.Error(), status)
		log.Printf(err.Error())
		return nil, metricData{status: status}, err
	}

	writeHeaderAndStatusOK(gsAttrs, br, w, reqID)
	return gsObject, metricData{status: http.StatusOK}, nil
}

// handleDownloadGET handles download GET request.
// It writes file stat to ResponseWriter header, and content to body.
func handleDownloadGET(ctx context.Context, w http.ResponseWriter, r *http.Request, gsClient downloadClient, br *byteRange, reqID string) metricData {
	gsObject, md, err := handleDownloadHEAD(ctx, w, r, gsClient, br, reqID)
	if err != nil {
		return md
	}

	var rc io.ReadCloser
	if br != nil {
		rc, err = gsObject.NewRangeReader(ctx, br.start, br.length())
	} else {
		rc, err = gsObject.NewReader(ctx)
	}
	if err != nil {
		log.Printf("%s NewReader error: %s", reqID, err)
		return metricData{status: http.StatusInternalServerError}
	}
	defer rc.Close()

	n, err := io.Copy(w, rc)
	status := http.StatusOK
	if err != nil {
		log.Printf("%s copy to body failed at byte %v: %s", reqID, n, err)
		status = http.StatusInternalServerError
	}
	return metricData{status: status, size: n}
}

// writeHeaderAndStatusOK writes various attributes to response header.
func writeHeaderAndStatusOK(objAttr *storage.ObjectAttrs, br *byteRange, w http.ResponseWriter, reqID string) {
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Type", objAttr.ContentType)
	w.Header().Set("Content-Hash-CRC32C", convertCRC32CToString(objAttr.CRC32C))
	// Object may or may not have MD5. https://cloud.google.com/storage/docs/hashes-etags
	if objAttr.MD5 != nil {
		w.Header().Set("Content-Hash-MD5", base64.StdEncoding.EncodeToString(objAttr.MD5))
	}

	if br != nil {
		// end cannot be more than size-1.
		if br.end > objAttr.Size-1 {
			br.updateEnd(objAttr.Size - 1)
			log.Printf("%s update range end to %d", reqID, br.end)
		}
		// Content-Range is required for partial content status.
		w.Header().Set("Content-Range", br.formatContentRange(objAttr.Size))
		w.Header().Set("Content-Length", strconv.FormatInt(br.length(), 10))
		w.WriteHeader(http.StatusPartialContent)
	} else {
		w.Header().Set("Content-Length", strconv.FormatInt(objAttr.Size, 10))
		w.WriteHeader(http.StatusOK)
	}
}

func convertCRC32CToString(i uint32) string {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, i)
	return base64.StdEncoding.EncodeToString(b)
}

// parseURL parses URL.
// It returns the storage object name(bucket and object path).
// Typical path for archive server is '/RPC/bucket/...object-path/'.
// After splitting, the fields would be like
// ["", RPC, bucket, ...object-path].
// Example: url = "/download/release/build/image.tar"
// bucket = "release", objectPath = "build/image.tar"
func parseURL(url string) (*storageObjectName, error) {
	fields := strings.Split(url, "/")
	if len(fields) < 4 {
		return nil, fmt.Errorf("the URL doesn't have all of RPC, bucket and object path")
	}
	if fields[2] == "" {
		return nil, fmt.Errorf("bucket cannot be empty")
	}
	path := strings.Join(fields[3:], "/")
	if path == "" {
		return nil, fmt.Errorf("object cannot be empty")
	}
	return &storageObjectName{bucket: fields[2], path: path}, nil
}

// parseRange parse range value and return range start, end bytes.
// It only support single range bytes=start-end format.
// Not yet support multiple range bytes=start1-end1,start2-end2.
// Empty input string returns nil byteRange.
func parseRange(s string) (*byteRange, error) {
	// s should be bytes=start-end or ""
	if s == "" {
		return nil, nil
	}
	i := strings.Index(s, "-")
	if s[:6] != "bytes=" || i == -1 {
		return nil, fmt.Errorf("%q not in format of 'bytes=<start>-<end>'", s)
	}
	start, errStart := strconv.ParseInt(s[6:i], 10, 64)
	if errStart != nil || start < 0 {
		return nil, fmt.Errorf("start value %q is not a positive integer", s[6:i])
	}
	end, errEnd := strconv.ParseInt(s[i+1:], 10, 64)
	if errEnd != nil || end < 0 {
		return nil, fmt.Errorf("end value %q is not a positive integer", s[i+1:])
	}
	if start > end {
		return nil, fmt.Errorf("start value %d cannot be larger than end value %d", start, end)
	}
	return &byteRange{start: start, end: end}, nil
}

type byteRange struct {
	start, end int64
}

func (r *byteRange) updateEnd(newEnd int64) {
	r.end = newEnd
}

func (r *byteRange) length() int64 {
	return r.end - r.start + 1
}

func (r *byteRange) formatContentRange(totalSize int64) string {
	return fmt.Sprintf("bytes %v-%v/%v", r.start, r.end, totalSize)
}

// extractHandler handles /extract/bucket/path/to/file?file=target_file requests.
// It extracts target_file from tar writes the stat to header,
// the content to body.
func (c *archiveServer) extractHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := otil.FuncSpan(r.Context())
	defer func() { otil.EndSpan(span, nil) }()
	startTime := time.Now()

	id := generateTraceID(r)
	log.Printf("%s request started", id)
	defer func() { log.Printf("%s request completed in %fs", id, time.Since(startTime).Seconds()) }()

	ctx, cancel := context.WithTimeout(ctx, 60*time.Minute)
	defer cancel()

	md := metricData{}
	defer updateMetrics(ctx, "extract", r.Method, &md, startTime)

	switch r.Method {
	case http.MethodHead:
		md = handleExtract(ctx, c.httpClient, w, r, c.cacheServerURL, id, false)
	case http.MethodGet:
		md = handleExtract(ctx, c.httpClient, w, r, c.cacheServerURL, id, true)
	default:
		errStr := fmt.Sprintf("%s unsupported method", id)
		http.Error(w, errStr, http.StatusBadRequest)
		md.status = http.StatusBadRequest
		log.Printf(errStr)
	}
}

// handleExtract handles extract HEAD/GET requests.
// It downloads the tar from cache server(nginx). It then, extracts
// the target file and writes stat to ResponseWriter header.
// If wantBody is true which essentially is GET, it will copy content to
// ResponseWriter body.
func handleExtract(ctx context.Context, c *http.Client, w http.ResponseWriter, r *http.Request, cacheServerURL string, reqID string, wantBody bool) metricData {
	objectName, err := parseURL(r.URL.Path)
	if err != nil {
		errStr := fmt.Sprintf("%s parseURL error: %s", reqID, err)
		http.Error(w, errStr, http.StatusBadRequest)
		log.Printf(errStr)
		return metricData{status: http.StatusBadRequest}
	}

	queryFile := r.URL.Query().Get("file")
	if queryFile == "" {
		errStr := fmt.Sprintf("%s extract file query not specified from %s", reqID, objectName.path)
		http.Error(w, errStr, http.StatusBadRequest)
		log.Printf(errStr)
		return metricData{status: http.StatusBadRequest}
	}

	action := "download"
	if _, ok := compressReaderMap[filepath.Ext(objectName.path)]; ok {
		action = "decompress"
	}
	reqURL := fmt.Sprintf("%s/%s/%s/%s", cacheServerURL, action, objectName.bucket, objectName.path)
	res := downloadURL(ctx, c, w, reqURL, reqID, r)
	if res == nil {
		return metricData{status: http.StatusInternalServerError}
	}
	if res.StatusCode != http.StatusOK {
		return metricData{status: res.StatusCode}
	}
	defer res.Body.Close()

	tarReader, status, err := extractTarAndWriteHeader(ctx, res.Body, queryFile, w)
	if err != nil {
		log.Printf(fmt.Sprintf("%s extractTarAndWriteHeader failed: %s", reqID, err))
		return metricData{status: status}
	}

	md := metricData{status: http.StatusOK}
	if wantBody {
		md.size, err = io.Copy(w, tarReader)
		if err != nil {
			log.Printf("%s copy to body failed at byte %v: %s", reqID, md.size, err)
			md.status = http.StatusInternalServerError
		}
	}
	return md
}

// downloadURL downloads the reqURL and returns the content in response.
// It writes to client header if error occurs or relays non 200 status code
// from upstream.
func downloadURL(ctx context.Context, c *http.Client, w http.ResponseWriter, reqURL string, reqID string, parentReq *http.Request) *http.Response {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		errStr := fmt.Sprintf("%s download request %q: %s", reqID, reqURL, err)
		http.Error(w, errStr, http.StatusInternalServerError)
		log.Printf(errStr)
		return nil
	}

	// Pass down headers needed for customer tracking or E2E tests.
	for _, h := range []string{"X-SWARMING-TASK-ID", "X-BBID", "X-NO-CACHE"} {
		req.Header.Add(h, parentReq.Header.Get(h))
	}

	res, err := c.Do(req)
	if err != nil {
		errStr := fmt.Sprintf("%s download request %q: %s", reqID, reqURL, err)
		http.Error(w, errStr, http.StatusInternalServerError)
		log.Printf(errStr)
		return nil
	}

	if res.StatusCode != http.StatusOK {
		defer res.Body.Close()
		upstreamErr, err := io.ReadAll(res.Body)
		errStr := fmt.Sprintf("%s %s respond %v status: %s", reqID, reqURL, res.StatusCode, upstreamErr)
		if err != nil {
			errStr = fmt.Sprintf("%s failed to read upstream %v response of %q: %s", reqID, res.StatusCode, reqURL, err)
		}
		http.Error(w, errStr, res.StatusCode)
		log.Printf(errStr)
	}
	return res
}

// extractTarAndWriteHeader extracts file from r reader.
// It writes file stat to the header and returns
// the tar reader for GET handling.
func extractTarAndWriteHeader(ctx context.Context, r io.Reader, fileName string, w http.ResponseWriter) (*tar.Reader, int, error) {
	tarReader := tar.NewReader(r)
	for {
		select {
		case <-ctx.Done():
			break
		default:
		}

		header, err := tarReader.Next()
		if err == io.EOF {
			err = fmt.Errorf("tarReader: %q not found in the tar file", fileName)
			http.Error(w, err.Error(), http.StatusNotFound)
			return nil, http.StatusNotFound, err
		}
		if err != nil {
			err = fmt.Errorf("tarReader error: %w", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return nil, http.StatusBadRequest, err
		}

		if header.Typeflag == tar.TypeReg && header.Name == fileName {
			w.Header().Set("Content-Length", strconv.FormatInt(header.Size, 10))
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusOK)
			return tarReader, http.StatusOK, nil
		}
	}
}

// decompressHandler handles the /decompress/bucket/path/to/file requests.
// It decompresses compressed file and returns content to body for GET method.
func (c *archiveServer) decompressHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := otil.FuncSpan(r.Context())
	defer func() { otil.EndSpan(span, nil) }()
	startTime := time.Now()

	id := generateTraceID(r)
	log.Printf("%s request started", id)
	defer func() { log.Printf("%s request completed in %fs", id, time.Since(startTime).Seconds()) }()

	ctx, cancel := context.WithTimeout(ctx, 60*time.Minute)
	defer cancel()

	md := metricData{}
	defer updateMetrics(ctx, "decompress", r.Method, &md, startTime)

	switch r.Method {
	case http.MethodGet:
		md = handleDecompressGET(ctx, c.httpClient, w, r, c.cacheServerURL, id)
	default:
		errStr := fmt.Sprintf("%s unsupported method", id)
		http.Error(w, errStr, http.StatusBadRequest)
		md.status = http.StatusBadRequest
		log.Printf(errStr)
	}
}

type compressReaderFunc func(io.Reader) (io.ReadCloser, error)

func newGZIPReader(r io.Reader) (io.ReadCloser, error) {
	return gzip.NewReader(r)
}

func newBZ2Reader(r io.Reader) (io.ReadCloser, error) {
	return io.NopCloser(bzip2.NewReader(r)), nil
}

func newXZReader(r io.Reader) (io.ReadCloser, error) {
	dReader, err := xz.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("new xz reader: %w", err)
	}
	return io.NopCloser(dReader), nil
}

var compressReaderMap = map[string]compressReaderFunc{
	".gz":  newGZIPReader,
	".tgz": newGZIPReader,
	".bz2": newBZ2Reader,
	".xz":  newXZReader,
}

// handleDecompressGET handles decompress GET method.
// It supports file types in allowedCompressExt.
// Due to the content-size requirement, it decompresses the downloaded file
// into the memory to get the size, then copies content to ResonpseWriter.
func handleDecompressGET(ctx context.Context, c *http.Client, w http.ResponseWriter, r *http.Request, cacheServerURL string, reqID string) metricData {
	objectName, err := parseURL(r.URL.Path)
	if err != nil {
		errStr := fmt.Sprintf("%s parseURL error: %s", reqID, err)
		http.Error(w, errStr, http.StatusBadRequest)
		log.Printf(errStr)
		return metricData{status: http.StatusBadRequest}
	}

	fileExt := filepath.Ext(objectName.path)
	newReader, ok := compressReaderMap[fileExt]
	if !ok {
		errStr := fmt.Sprintf("%s decompress does not support %s extension", reqID, fileExt)
		http.Error(w, errStr, http.StatusBadRequest)
		log.Printf(errStr)
		return metricData{status: http.StatusBadRequest}
	}

	reqURL := fmt.Sprintf("%s/download/%s/%s", cacheServerURL, objectName.bucket, objectName.path)
	res := downloadURL(ctx, c, w, reqURL, reqID, r)
	if res == nil {
		return metricData{status: http.StatusInternalServerError}
	}
	if res.StatusCode != http.StatusOK {
		return metricData{status: res.StatusCode}
	}
	defer res.Body.Close()

	dReader, err := newReader(res.Body)
	if err != nil {
		errStr := fmt.Sprintf("%s newReader error: %s", reqID, err)
		http.Error(w, errStr, http.StatusInternalServerError)
		log.Printf(errStr)
		return metricData{status: http.StatusInternalServerError}
	}
	defer dReader.Close()

	rMem, err := io.ReadAll(dReader)
	if err != nil {
		errStr := fmt.Sprintf("%s ReadAll failed after %v bytes: %s", reqID, len(rMem), err)
		http.Error(w, errStr, http.StatusInternalServerError)
		log.Printf(errStr)
		return metricData{status: http.StatusInternalServerError}
	}

	n, err := decompressWrite(ctx, w, rMem)
	md := metricData{status: http.StatusOK, size: int64(n)}
	if err != nil {
		log.Printf("%s decompressWrite failed: %s", reqID, err)
		md.status = http.StatusInternalServerError
	}
	return md
}

// decompressWrite writes memory buffer to w Response
func decompressWrite(ctx context.Context, w http.ResponseWriter, mem []byte) (int, error) {
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.Itoa(len(mem)))
	w.WriteHeader(http.StatusOK)

	n, err := w.Write(mem)
	if err != nil {
		return n, fmt.Errorf("write to client failed at byte %v: %w", n, err)
	}

	return n, nil
}

// getHTTPClient gets a http client to download intermediate files for
// extraction/decompression from the upstream cache server (not GCS).
func getHTTPClient(sourceAddr string, defaultClient *http.Client) (*http.Client, error) {
	if sourceAddr == "" {
		return defaultClient, nil
	}
	addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:0", sourceAddr))
	if err != nil {
		return nil, fmt.Errorf("get http client bind to %q: %w", sourceAddr, err)
	}
	dialer := &net.Dialer{LocalAddr: addr}
	transport := &http.Transport{DialContext: dialer.DialContext}
	return &http.Client{Transport: transport}, nil
}

// deriveHTTPStatusFromError returns a proper HTTP status code based on the
// error details.
func deriveHTTPStatusFromError(err error) int {
	var e *googleapi.Error
	var status int
	if ok := errors.As(err, &e); ok {
		status = e.Code
	} else if errors.Is(err, storage.ErrBucketNotExist) || errors.Is(err, storage.ErrObjectNotExist) {
		status = http.StatusNotFound
	} else {
		status = http.StatusInternalServerError
	}
	return status
}

// generateTraceID gets the unique id of the request
func generateTraceID(r *http.Request) string {
	id := []string{
		fmt.Sprintf("%s%s", r.Method, r.URL.RequestURI()),
		// The Range header starts with "bytes=", so no need to add a field name.
		r.Header.Get("Range"),
		fmt.Sprintf("swarming_task_id=%s", r.Header.Get("X-SWARMING-TASK-ID")),
		fmt.Sprintf("bbid=%s", r.Header.Get("X-BBID")),
	}

	return strings.Join(id, " ")
}

type metricData struct {
	status int
	size   int64
}
