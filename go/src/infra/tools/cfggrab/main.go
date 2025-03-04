// Copyright 2021 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Command cfggrab fetches some <name>.cfg from all LUCI project configs.
//
// By default prints all fetched configs to stdout, prefixing each printed
// line with the project name for easier grepping.
//
// If `-outputDir-dir` is given, stores the fetched files into per-project
// <outputDir-dir>/<project>.cfg.
//
// Usage:
//
//	luci-auth login
//	cfggrab cr-buildbucket.cfg | grep "service_account"
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sync"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/reflect/protoreflect"

	"go.chromium.org/luci/auth"
	bbpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/errors"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/common/logging/gologger"
	configpb "go.chromium.org/luci/common/proto/config"
	realmspb "go.chromium.org/luci/common/proto/realms"
	"go.chromium.org/luci/common/sync/parallel"
	"go.chromium.org/luci/config"
	"go.chromium.org/luci/config/cfgclient"
	"go.chromium.org/luci/config/impl/remote"
	cvpb "go.chromium.org/luci/cv/api/config/v2"
	"go.chromium.org/luci/hardcoded/chromeinfra"
	logdogpb "go.chromium.org/luci/logdog/api/config/svcconfig"
	notifypb "go.chromium.org/luci/luci_notify/api/config"
	milopb "go.chromium.org/luci/milo/proto/projectconfig"
	schedulerpb "go.chromium.org/luci/scheduler/appengine/messages"
)

var outputDir = flag.String("output-dir", "-", "Where to store fetched files or - to print them to stdout for grepping")
var configService = flag.String("config-service-host", chromeinfra.ConfigServiceHost, "Hostname of LUCI Config service to query")
var convertJSON = flag.Bool("convert-json", false, ("Convert fetched files to JSONPB (best-effort)." +
	" If output-dir is -, wraps messages in {\"proj\": ..., \"data\": ...}."))
var isServiceCfg = flag.Bool("service-cfg", false, "Treat the config name a service-name/file and load it as service config")

// quick and dirty map for known config file names to the proto message they
// contain.
var messageMap map[string]protoreflect.Message = map[string]protoreflect.Message{
	"commit-queue.cfg":       (*cvpb.Config)(nil).ProtoReflect(),
	"cr-buildbucket-dev.cfg": (*bbpb.BuildbucketCfg)(nil).ProtoReflect(),
	"cr-buildbucket.cfg":     (*bbpb.BuildbucketCfg)(nil).ProtoReflect(),
	"luci-logdog-dev.cfg":    (*logdogpb.ProjectConfig)(nil).ProtoReflect(),
	"luci-logdog.cfg":        (*logdogpb.ProjectConfig)(nil).ProtoReflect(),
	"luci-milo-dev.cfg":      (*milopb.Project)(nil).ProtoReflect(),
	"luci-milo.cfg":          (*milopb.Project)(nil).ProtoReflect(),
	"luci-notify-dev.cfg":    (*notifypb.ProjectConfig)(nil).ProtoReflect(),
	"luci-notify.cfg":        (*notifypb.ProjectConfig)(nil).ProtoReflect(),
	"luci-scheduler-dev.cfg": (*schedulerpb.ProjectConfig)(nil).ProtoReflect(),
	"luci-scheduler.cfg":     (*schedulerpb.ProjectConfig)(nil).ProtoReflect(),
	"project.cfg":            (*configpb.ProjectCfg)(nil).ProtoReflect(),
	"projects.cfg":           (*configpb.ProjectsCfg)(nil).ProtoReflect(),
	"realms-dev.cfg":         (*realmspb.RealmsCfg)(nil).ProtoReflect(),
	"realms.cfg":             (*realmspb.RealmsCfg)(nil).ProtoReflect(),
}

var stdoutLock sync.Mutex

type source struct {
	cfgSet config.Set
	path   string
}

func (s source) String() string {
	return fmt.Sprintf("%s/%s", s.cfgSet, s.path)
}

func main() {
	flag.Parse()
	if len(flag.Args()) != 1 {
		fmt.Fprintf(os.Stderr, "Expecting one positional argument with the name of the file to fetch.\n")
		os.Exit(2)
	}

	toLoad := flag.Arg(0)

	var msg protoreflect.Message
	if *convertJSON {
		var ok bool
		fileName := path.Base(toLoad)
		if msg, ok = messageMap[fileName]; !ok {
			fmt.Fprintf(os.Stderr, "Cannot convert %q to JSON: unknown file", toLoad)
			os.Exit(2)
		}
	}

	ctx := context.Background()
	ctx = gologger.StdConfig.Use(ctx)

	if err := mainImpl(ctx, toLoad, *outputDir, msg, *isServiceCfg); err != nil {
		errors.Log(ctx, err)
		os.Exit(1)
	}

}

func mainImpl(ctx context.Context, toLoad, outputDir string, msg protoreflect.Message, isServiceCfg bool) error {
	if outputDir != "-" {
		if err := os.MkdirAll(outputDir, 0777); err != nil {
			errors.Log(ctx, err)
			os.Exit(1)
		}
	}

	ctx, err := withConfigClient(ctx)
	if err != nil {
		return err
	}

	var sources []source
	if isServiceCfg {
		service, toLoad := path.Split(toLoad)
		sources = []source{{config.Set("services/" + path.Clean(service)), toLoad}}
	} else {
		sources, err = gatherProjectSources(ctx, toLoad)
	}
	if err != nil {
		return err
	}

	return processSources(ctx, sources, processConfig(outputDir, msg))
}

func withConfigClient(ctx context.Context) (context.Context, error) {
	host := *configService
	authOpts := chromeinfra.DefaultAuthOptions()
	authOpts.UseIDTokens = true
	authOpts.Audience = "https://" + host

	creds, err := auth.NewAuthenticator(ctx, auth.SilentLogin, authOpts).PerRPCCredentials()
	if err != nil {
		return nil, errors.Annotate(err, "failed to get credentials to access %s", host).Err()
	}
	iface, err := remote.New(ctx, remote.Options{
		Host:      host,
		Creds:     creds,
		UserAgent: "cfggrab",
	})
	if err != nil {
		return nil, errors.Annotate(err, "failed to create config interface").Err()
	}
	return cfgclient.Use(ctx, iface), nil
}

func gatherProjectSources(ctx context.Context, fileName string) ([]source, error) {
	projects, err := cfgclient.ProjectsWithConfig(ctx, fileName)
	if err != nil {
		return nil, err
	}
	ret := make([]source, len(projects))
	for i, proj := range projects {
		ret[i] = source{
			config.Set("projects/" + proj),
			fileName,
		}
	}
	return ret, err
}

func processSources(ctx context.Context, sources []source, cb func(ctx context.Context, source source, raw []byte) error) error {
	return parallel.FanOutIn(func(work chan<- func() error) {
		for _, source := range sources {
			work <- func() error {
				var blob []byte
				err := cfgclient.Get(ctx, source.cfgSet, source.path, cfgclient.Bytes(&blob), nil)
				if err != nil {
					return err
				}

				if err = cb(ctx, source, blob); err != nil {
					logging.Errorf(ctx, "Failed when processing %s/%s: %s", source.cfgSet, source.path, err)
				}
				return err
			}
		}
	})
}

func processConfig(outputDir string, msg protoreflect.Message) func(context.Context, source, []byte) error {
	return func(ctx context.Context, source source, raw []byte) (err error) {
		if msg != nil {
			decoded := msg.New().Interface()
			if err = prototext.Unmarshal(raw, decoded); err != nil {
				return errors.Annotate(err, "decoding textpb").Err()
			}
			raw, err = protojson.MarshalOptions{
				Multiline:     true,
				Indent:        "  ",
				UseProtoNames: true,
			}.Marshal(decoded)
			if err != nil {
				return errors.Annotate(err, "encoding jsonpb").Err()
			}
		}

		if outputDir != "-" {
			stdoutLock.Lock()
			defer stdoutLock.Unlock()
			dest := filepath.Join(outputDir, source.String())
			fmt.Printf("writing %s\n", dest)
			if err = os.MkdirAll(filepath.Dir(dest), 0777); err == nil {
				err = os.WriteFile(dest, raw, 0666)
			}
			return
		}

		stdoutLock.Lock()
		defer stdoutLock.Unlock()
		if msg == nil {
			// raw
			for _, line := range bytes.Split(raw, []byte{'\n'}) {
				fmt.Printf("%s: %s\n", source.cfgSet, line)
			}
		} else {
			// json
			fmt.Printf(`{"source": {"configSet": %q, "path": %q}, "data": %s}`, source.cfgSet, source.path, raw)
			fmt.Println()
		}
		return nil
	}
}
