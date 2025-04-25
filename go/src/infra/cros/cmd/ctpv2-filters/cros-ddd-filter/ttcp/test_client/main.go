// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	context "context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"

	pb "go.chromium.org/chromiumos/config/go/test/api"

	ppjson "go.chromium.org/infra/cros/cmd/ctpv2-filters/cros-ddd-filter/ttcp/libs/pprinter/json"
)

const (
	defaultRootPath = "/tmp/test/ctp-filter"
)

var defaultPort = 8080

var (
	// Server mode params
	addr = flag.String("addr", "0.0.0.0:8080", "the address to connect to")
)

// This is the entry point for the dockerized version of TTCP.
func main() {
	fmt.Println("solver service")
	flag.Parse()
	error := mainInt()
	if error != nil {
		log.Println("ERROR: The application closed due to the error:", error)
	}
}

// Main function that starts the Grpc server.
// The supported command line options are:
//   - `-addr <host:port>` defines the address
func mainInt() error {

	conn, err := grpc.Dial(
		*addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(16*10e6),
		))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	client := pb.NewGenericFilterServiceClient(conn)

	data, err := os.ReadFile("./src/ttcp/solver_service/test_client/data/request.json")
	if err != nil {
		fmt.Println("Error reading file:", err)
		return err
	}

	testRequest := string(data)
	execute(client, testRequest)

	return nil
}

func execute(client pb.GenericFilterServiceClient, request string) {
	testPlan := parseTestPlan(request)

	fmt.Println()
	jsonStr := protojson.MarshalOptions{}.Format(testPlan)
	var jsonValue interface{}
	json.Unmarshal([]byte(jsonStr), &jsonValue)
	log.Printf("Imput:")
	fmt.Println(ppjson.DispValue(jsonValue))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	r, err := client.Execute(ctx, testPlan)
	if err != nil {
		log.Fatalf("could not execute: %v", err)
	}
	fmt.Println()
	fmt.Println()
	jsonStr = protojson.MarshalOptions{}.Format(r)
	json.Unmarshal([]byte(jsonStr), &jsonValue)
	log.Printf("Output:")
	fmt.Println(ppjson.DispValue(jsonValue))
	fmt.Println()
}

func parseTestPlan(source string) *pb.InternalTestplan {
	unmarshalOptions := protojson.UnmarshalOptions{
		AllowPartial:   false,
		DiscardUnknown: false,
	}
	testPlan := pb.InternalTestplan{}
	err := unmarshalOptions.Unmarshal([]byte(source), &testPlan)
	if err != nil {
		log.Fatal(err)
	}
	return &testPlan
}
