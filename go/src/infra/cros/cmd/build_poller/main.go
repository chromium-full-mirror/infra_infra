package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/maruel/subcommands"
	"google.golang.org/protobuf/encoding/protojson"

	"go.chromium.org/luci/auth"
	"go.chromium.org/luci/auth/client/authcli"
	bbpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/common/cli"
	"go.chromium.org/luci/common/data/text"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/common/logging/gologger"
	"go.chromium.org/luci/grpc/prpc"
	"go.chromium.org/luci/hardcoded/chromeinfra"

	"go.chromium.org/infra/cros/lib/buildbucket"
)

var logCfg = gologger.LoggerConfig{
	Out: os.Stderr,
}

func cmdCollect(authOpts auth.Options) *subcommands.Command {
	return &subcommands.Command{
		UsageLine: "collect -outputprop <outputprop> -json <path> <BUILD ID> [<BUILD ID>...]",
		ShortDesc: "polls until a set of builds has completed or set an output property",
		LongDesc: text.Doc(`
		Polls until a set of builds has completed or set an output property. This can be useful
		if we want to poll until a set of builds has reached a certain state, but is not necessarily
		complete, e.g. a set of builders has published an image.

		At completion, outputs the Build protos of all builds in newline-delimited JSON.

		Note that the value of outputprop isn't checked, just the fact that it is set on the output
		properties of the build.
		`),
		CommandRun: func() subcommands.CommandRun {
			r := &collectRun{}

			r.authFlags = authcli.Flags{}
			r.authFlags.Register(r.GetFlags(), authOpts)

			r.logLevel = logging.Info
			r.Flags.Var(&r.logLevel, "loglevel", text.Doc(`
			Log level, valid options are "debug", "info", "warning", "error". Default is "info".
			`))

			r.Flags.StringVar(&r.host, "host", "cr-buildbucket.appspot.com", "Buildbucket host to use.")
			r.Flags.StringVar(&r.outputProperty, "outputprop", "", "Output property to poll for, required.")
			r.Flags.DurationVar(&r.interval, "interval", 60*time.Second, "Duration to wait between calls to Buildbucket.")
			r.Flags.StringVar(&r.json, "json", "", `Path to write Build protos to, in newline-delimited jsonpb, required. If set to "-" write to stdout.`)

			return r
		},
	}
}

type collectRun struct {
	subcommands.CommandRunBase
	authFlags authcli.Flags
	logLevel  logging.Level

	host           string
	outputProperty string
	interval       time.Duration
	json           string
}

func (r *collectRun) Run(a subcommands.Application, args []string, env subcommands.Env) int {
	ctx := logging.SetLevel(cli.GetContext(a, r, env), r.logLevel)

	if err := r.doRun(ctx, args); err != nil {
		logging.Errorf(ctx, "%s", err)
		return 1
	}

	return 0
}

func (r *collectRun) validateFlagsAndParseBuildIds(args []string) ([]int64, error) {
	if len(r.outputProperty) == 0 {
		return nil, errors.New("-outputprop must be set.")
	}

	if strings.Contains(r.outputProperty, ".") {
		return nil, errors.New(`"." characters in -outputprop not supported`)
	}

	if len(r.json) == 0 {
		return nil, errors.New("-json must be set")
	}

	if len(args) == 0 {
		return nil, errors.New("at least one build must be specified")
	}

	buildIds := make([]int64, len(args))
	for i, arg := range args {
		id, err := strconv.ParseInt(arg, 10, 64)
		if err != nil {
			return nil, err
		}

		buildIds[i] = id
	}

	return buildIds, nil
}

func (r *collectRun) doRun(ctx context.Context, args []string) error {
	buildIds, err := r.validateFlagsAndParseBuildIds(args)
	if err != nil {
		return err
	}

	authOpts, err := r.authFlags.Options()
	if err != nil {
		return err
	}

	httpClient, err := auth.NewAuthenticator(ctx, auth.SilentLogin, authOpts).Client()
	if err != nil {
		return err
	}

	buildsClient := bbpb.NewBuildsPRPCClient(&prpc.Client{
		C:       httpClient,
		Host:    r.host,
		Options: buildbucket.DefaultPRPCOpts(),
	})

	builds, err := buildbucket.PollForOutputProp(
		ctx, buildsClient, buildIds, r.outputProperty, r.interval,
	)
	if err != nil {
		return err
	}

	jsonpbBuilds := make([]string, 0, len(builds))
	for _, build := range builds {
		jsonpbBuild, err := protojson.Marshal(build)
		if err != nil {
			return err
		}
		jsonpbBuilds = append(jsonpbBuilds, string(jsonpbBuild))
	}

	serializedBuilds := strings.Join(jsonpbBuilds, "\n") + "\n"

	if r.json == "-" {
		_, err := fmt.Fprint(os.Stdout, serializedBuilds)
		return err
	}

	return os.WriteFile(r.json, []byte(serializedBuilds), os.ModePerm)
}

func GetApplication(authOpts auth.Options) *cli.Application {
	return &cli.Application{
		Name: "build_poller",

		Context: logCfg.Use,
		Commands: []*subcommands.Command{
			authcli.SubcommandInfo(authOpts, "auth-info", false),
			authcli.SubcommandLogin(authOpts, "auth-login", false),
			authcli.SubcommandLogout(authOpts, "auth-logout", false),
			{},
			cmdCollect(authOpts),
			{},
			subcommands.CmdHelp,
		},
	}
}

func main() {
	opts := chromeinfra.DefaultAuthOptions()
	app := GetApplication(opts)
	os.Exit(subcommands.Run(app, nil))

}
