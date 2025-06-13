module go.chromium.org/infra

// WARNING: When changing this, please first make sure that golangci-lint is
// updated to a version that supports it.
// See https://github.com/golangci/golangci-lint/issues/5225 for details on how
// golangci-lint version support works.
// For an example update CL, see https://crrev.com/c/6038926
go 1.23.0

toolchain go1.23.10

require (
	cloud.google.com/go v0.120.1
	cloud.google.com/go/appengine v1.9.6
	cloud.google.com/go/bigquery v1.67.0
	cloud.google.com/go/cloudsqlconn v1.13.2
	cloud.google.com/go/cloudtasks v1.13.6
	cloud.google.com/go/compute v1.36.1
	cloud.google.com/go/compute/metadata v0.6.0
	cloud.google.com/go/datastore v1.20.0
	cloud.google.com/go/firestore v1.18.0
	cloud.google.com/go/logging v1.13.0
	cloud.google.com/go/longrunning v0.6.7
	cloud.google.com/go/monitoring v1.24.2
	cloud.google.com/go/profiler v0.4.2
	cloud.google.com/go/pubsub v1.49.0
	cloud.google.com/go/run v1.9.3
	cloud.google.com/go/secretmanager v1.14.7
	cloud.google.com/go/storage v1.51.0
	cloud.google.com/go/trace v1.11.6
	contrib.go.opencensus.io/exporter/stackdriver v0.13.14
	github.com/DATA-DOG/go-sqlmock v1.5.2
	github.com/Microsoft/go-winio v0.6.2
	github.com/PaesslerAG/jsonpath v0.1.1
	github.com/StackExchange/wmi v1.2.1
	github.com/VividCortex/godaemon v1.0.0
	github.com/aclements/go-moremath v0.0.0-20241023150245-c8bbc672ef66
	github.com/alexflint/go-arg v1.5.1
	github.com/andygrunwald/go-gerrit v0.0.0-20210726065827-cc4e14e40b5b
	github.com/armon/circbuf v0.0.0-20190214190532-5111143e8da2
	github.com/bazelbuild/reclient/api v0.0.0-20240617160057-89d6134e48e5
	github.com/bazelbuild/remote-apis v0.0.0-20250106163131-9a0af1d31814
	github.com/bazelbuild/remote-apis-sdks v0.0.0-20250110170550-8bf84d3488e5
	github.com/beevik/etree v1.4.1
	github.com/biogo/hts v1.4.5
	github.com/bmatcuk/doublestar v1.3.4
	github.com/cenkalti/backoff/v4 v4.3.0
	github.com/containerd/cgroups v1.1.0
	github.com/danjacques/gofslock v0.0.0-20240212154529-d899e02bfe22
	github.com/docker/docker v27.5.0+incompatible
	github.com/docker/go-connections v0.5.0
	github.com/dustin/go-humanize v1.0.1
	github.com/electricbubble/gadb v0.1.1-0.20240523030754-97c5a1a929a9
	github.com/fsnotify/fsnotify v1.8.0
	github.com/gliderlabs/ssh v0.3.9-0.20241212082318-d137aad99cd6
	github.com/go-delve/delve v1.24.0
	github.com/go-git/go-git/v5 v5.13.2
	github.com/gofrs/flock v0.12.1
	github.com/gogo/protobuf v1.3.2
	github.com/golang/glog v1.2.5
	github.com/golang/mock v1.6.0
	github.com/golang/protobuf v1.5.4
	github.com/google/cel-go v0.22.1
	github.com/google/generative-ai-go v0.19.0
	github.com/google/go-cmp v0.7.0
	github.com/google/go-containerregistry v0.14.0
	github.com/google/safetext v0.0.0-20240722112252-5a72de7e7962
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510
	github.com/google/subcommands v1.2.0
	github.com/google/uuid v1.6.0
	github.com/googleapis/gax-go/v2 v2.14.1
	github.com/googleapis/google-cloud-go-testing v0.0.0-20210719221736-1c9a4c676720
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hashicorp/go-version v1.7.0
	github.com/jackc/pgconn v1.14.3
	github.com/jackc/pgtype v1.14.4
	github.com/jackc/pgx/v5 v5.7.2
	github.com/jdxcode/netrc v1.0.0
	github.com/julienschmidt/httprouter v1.3.0
	github.com/klauspost/compress v1.18.0
	github.com/klauspost/cpuid/v2 v2.2.9
	github.com/kr/pretty v0.3.1
	github.com/kylelemons/godebug v1.1.0
	github.com/linkedin/goavro/v2 v2.13.1
	github.com/maruel/subcommands v1.1.1
	github.com/mitchellh/go-homedir v1.1.0
	github.com/mitchellh/hashstructure/v2 v2.0.2
	github.com/op/go-logging v0.0.0-20160315200505-970db520ece7
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.1.0
	github.com/opencontainers/runtime-spec v1.2.0
	github.com/otiai10/copy v1.14.1
	github.com/pkg/errors v0.9.1
	github.com/pkg/profile v1.7.0
	github.com/pkg/xattr v0.4.10
	github.com/prometheus/client_golang v1.20.5
	github.com/savaki/jq v0.0.0-20161209013833-0e6baecebbf8
	github.com/shirou/gopsutil/v3 v3.24.5
	github.com/shirou/gopsutil/v4 v4.24.12
	github.com/spf13/cobra v1.8.1
	github.com/stretchr/testify v1.10.0
	github.com/testcontainers/testcontainers-go v0.35.0
	github.com/testcontainers/testcontainers-go/modules/compose v0.35.0
	github.com/ulikunitz/xz v0.5.12
	github.com/waigani/diffparser v0.0.0-20190828052634-7391f219313d
	go.chromium.org/chromiumos/config/go v0.0.0-20240309015314-b8a183866804
	go.chromium.org/chromiumos/ctp v0.0.0-00010101000000-000000000000
	go.chromium.org/chromiumos/infra/proto/go v0.0.0-20250102200227-b13b715cea73
	go.chromium.org/chromiumos/lro v0.0.0-00010101000000-000000000000
	go.chromium.org/luci v0.0.0-20240531181147-0c7c729b2fcf
	go.chromium.org/tast v0.0.0-00010101000000-000000000000
	go.opencensus.io v0.24.0
	go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace v0.58.0
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.60.0
	go.opentelemetry.io/otel v1.35.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.33.0
	go.opentelemetry.io/otel/sdk v1.35.0
	go.opentelemetry.io/otel/trace v1.35.0
	go.skia.org/infra v0.0.0-20250114041601-4a95c0b5f67e
	go.starlark.net v0.0.0-20250127224320-2fb1215d657f
	golang.org/x/crypto v0.37.0
	golang.org/x/exp v0.0.0-20250106191152-7588d65b2ba8
	golang.org/x/mobile v0.0.0-20191031020345-0945064e013a
	golang.org/x/mod v0.23.0
	golang.org/x/net v0.39.0
	golang.org/x/oauth2 v0.29.0
	golang.org/x/perf v0.0.0-20250106172127-400946f43c82
	golang.org/x/sync v0.13.0
	golang.org/x/sys v0.32.0
	golang.org/x/term v0.31.0
	golang.org/x/time v0.11.0
	golang.org/x/tools v0.30.0
	golang.org/x/xerrors v0.0.0-20240903120638-7835f813f4da
	gonum.org/v1/gonum v0.15.1
	google.golang.org/api v0.229.0
	google.golang.org/appengine v1.6.8
	google.golang.org/appengine/v2 v2.0.6
	google.golang.org/genproto v0.0.0-20250425173222-7b384671a197
	google.golang.org/genproto/googleapis/api v0.0.0-20250425173222-7b384671a197
	google.golang.org/genproto/googleapis/bytestream v0.0.0-20250414145226-207652e42e2e
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250425173222-7b384671a197
	google.golang.org/grpc v1.73.0
	google.golang.org/grpc/cmd/protoc-gen-go-grpc v1.5.1
	google.golang.org/protobuf v1.36.6
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.1
	gotest.tools v2.2.0+incompatible
	howett.net/plist v1.0.1
	k8s.io/api v0.32.0
	k8s.io/apimachinery v0.32.0
	k8s.io/client-go v0.32.0
	k8s.io/metrics v0.32.0
	sigs.k8s.io/yaml v1.4.0
)

require (
	cel.dev/expr v0.23.0 // indirect
	cloud.google.com/go/ai v0.8.0 // indirect
	cloud.google.com/go/auth v0.16.0 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.8 // indirect
	cloud.google.com/go/errorreporting v0.3.2 // indirect
	cloud.google.com/go/iam v1.5.2 // indirect
	dario.cat/mergo v1.0.1 // indirect
	github.com/AdaLogics/go-fuzz-headers v0.0.0-20240806141605-e8a1dd7889d6 // indirect
	github.com/AlecAivazis/survey/v2 v2.3.7 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20230124172434-306776ec8161 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp v1.27.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric v0.51.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace v1.26.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping v0.51.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/propagator v0.50.0 // indirect
	github.com/GoogleCloudPlatform/protoc-gen-bq-schema v0.0.0-20190119112626-026f9fcdf705 // indirect
	github.com/Masterminds/semver/v3 v3.2.1 // indirect
	github.com/Microsoft/hcsshim v0.11.5 // indirect
	github.com/PaesslerAG/gval v1.0.0 // indirect
	github.com/ProtonMail/go-crypto v1.1.5 // indirect
	github.com/acarl005/stripansi v0.0.0-20180116102854-5a71ef0e047d // indirect
	github.com/alecthomas/participle/v2 v2.1.0 // indirect
	github.com/alexflint/go-scalar v1.2.0 // indirect
	github.com/anmitsu/go-shlex v0.0.0-20200514113438-38f4b401e2be // indirect
	github.com/antlr4-go/antlr/v4 v4.13.0 // indirect
	github.com/apache/arrow/go/v15 v15.0.2 // indirect
	github.com/aws/aws-sdk-go v1.55.5 // indirect
	github.com/aws/aws-sdk-go-v2 v1.24.1 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.26.6 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.16.16 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.14.11 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.2.10 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.5.10 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.7.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.10.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.10.10 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.18.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.21.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.26.7 // indirect
	github.com/aws/smithy-go v1.19.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/buger/goterm v1.0.4 // indirect
	github.com/cenkalti/backoff v2.2.1+incompatible // indirect
	github.com/census-instrumentation/opencensus-proto v0.4.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cilium/ebpf v0.11.0 // indirect
	github.com/cloudflare/circl v1.6.1 // indirect
	github.com/cncf/xds/go v0.0.0-20250326154945-ae57f3c0d45f // indirect
	github.com/compose-spec/compose-go/v2 v2.1.3 // indirect
	github.com/containerd/console v1.0.4 // indirect
	github.com/containerd/containerd v1.7.18 // indirect
	github.com/containerd/continuity v0.4.3 // indirect
	github.com/containerd/errdefs v0.1.0 // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/containerd/platforms v0.2.1 // indirect
	github.com/containerd/ttrpc v1.2.5 // indirect
	github.com/containerd/typeurl/v2 v2.1.1 // indirect
	github.com/coreos/go-systemd/v22 v22.5.0 // indirect
	github.com/cosiner/argv v0.1.0 // indirect
	github.com/cpuguy83/dockercfg v0.3.2 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.4 // indirect
	github.com/creack/pty v1.1.23 // indirect
	github.com/cyphar/filepath-securejoin v0.3.6 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/derekparker/trie v0.0.0-20230829180723-39f4de51ef7d // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/docker/buildx v0.15.1 // indirect
	github.com/docker/cli v27.0.3+incompatible // indirect
	github.com/docker/compose/v2 v2.28.1 // indirect
	github.com/docker/distribution v2.8.3+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.8.0 // indirect
	github.com/docker/go v1.5.1-1.0.20160303222718-d30aec9fd63c // indirect
	github.com/docker/go-metrics v0.0.1 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/ebitengine/purego v0.8.1 // indirect
	github.com/eiannone/keyboard v0.0.0-20220611211555-0d226195f203 // indirect
	github.com/emicklei/go-restful/v3 v3.11.0 // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/envoyproxy/go-control-plane/envoy v1.32.4 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.2.1 // indirect
	github.com/felixge/fgprof v0.9.4 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fiorix/go-web v1.0.1-0.20150221144011-5b593f1e8966 // indirect
	github.com/fsnotify/fsevents v0.2.0 // indirect
	github.com/fvbommel/sortorder v1.0.2 // indirect
	github.com/fxamacker/cbor/v2 v2.7.0 // indirect
	github.com/go-chi/chi/v5 v5.0.8 // indirect
	github.com/go-delve/liner v1.2.3-0.20231231155935-4726ab1d7f62 // indirect
	github.com/go-git/gcfg v1.5.1-0.20230307220236-3a3c6141e376 // indirect
	github.com/go-git/go-billy/v5 v5.6.2 // indirect
	github.com/go-jose/go-jose/v4 v4.0.5 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/jsonreference v0.20.4 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.0.0 // indirect
	github.com/goccy/go-json v0.10.2 // indirect
	github.com/goccy/go-yaml v1.17.1 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/gogo/googleapis v1.4.1 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/gomodule/redigo v1.9.2 // indirect
	github.com/google/flatbuffers v23.5.26+incompatible // indirect
	github.com/google/gnostic-models v0.6.8 // indirect
	github.com/google/go-dap v0.12.0 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/pprof v0.0.0-20241029153458-d1b30febd7db // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/google/tink/go v1.7.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.6 // indirect
	github.com/gorilla/mux v1.8.1 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.24.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/golang-lru v1.0.2 // indirect
	github.com/in-toto/in-toto-golang v0.5.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jackc/chunkreader/v2 v2.0.1 // indirect
	github.com/jackc/pgio v1.0.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgproto3/v2 v2.3.3 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v4 v4.18.3 // indirect
	github.com/jackc/puddle v1.3.0 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/jcgregorio/logger v0.1.3 // indirect
	github.com/jcgregorio/slog v0.0.0-20190423190439-e6f2d537f900 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/jonboulle/clockwork v0.4.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kballard/go-shellquote v0.0.0-20180428030007-95032a82bc51 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/luci/gtreap v0.0.0-20161228054646-35df89791e8f // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/magiconair/properties v1.8.7 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	github.com/mattn/go-shellwords v1.0.12 // indirect
	github.com/mattn/go-tty v0.0.7 // indirect
	github.com/mgutz/ansi v0.0.0-20200706080929-d51e80ef957d // indirect
	github.com/miekg/pkcs11 v1.1.1 // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/moby/buildkit v0.14.1 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/moby/locker v1.0.1 // indirect
	github.com/moby/patternmatcher v0.6.0 // indirect
	github.com/moby/spdystream v0.5.0 // indirect
	github.com/moby/sys/mountinfo v0.7.1 // indirect
	github.com/moby/sys/sequential v0.5.0 // indirect
	github.com/moby/sys/signal v0.7.0 // indirect
	github.com/moby/sys/symlink v0.2.0 // indirect
	github.com/moby/sys/user v0.1.0 // indirect
	github.com/moby/sys/userns v0.1.0 // indirect
	github.com/moby/term v0.5.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/mxk/go-flowrate v0.0.0-20140419014527-cca7078d478f // indirect
	github.com/otiai10/mint v1.6.3 // indirect
	github.com/pborman/uuid v1.2.1 // indirect
	github.com/pelletier/go-toml v1.9.5 // indirect
	github.com/pierrec/lz4/v4 v4.1.18 // indirect
	github.com/pjbgf/sha1cd v0.3.2 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/power-devops/perfstat v0.0.0-20210106213030-5aafc221ea8c // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.55.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/prometheus/prometheus v0.54.1 // indirect
	github.com/protocolbuffers/txtpbfmt v0.0.0-20241112170944-20d2c9ebc01d // indirect
	github.com/r3labs/sse v0.0.0-20210224172625-26fe804710bc // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/rogpeppe/go-internal v1.13.1 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/secure-systems-lab/go-securesystemslib v0.4.0 // indirect
	github.com/sergi/go-diff v1.3.2-0.20230802210424-5b0b94c5c0d3 // indirect
	github.com/serialx/hashring v0.0.0-20200727003509-22c0c7ab6b1b // indirect
	github.com/shibumi/go-pathspec v1.3.0 // indirect
	github.com/shoenig/go-m1cpu v0.1.6 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/skratchdot/open-golang v0.0.0-20200116055534-eef842397966 // indirect
	github.com/smarty/assertions v1.16.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/spiffe/go-spiffe/v2 v2.5.0 // indirect
	github.com/stoewer/go-strcase v1.3.0 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/texttheater/golang-levenshtein v1.0.1 // indirect
	github.com/theupdateframework/notary v0.7.0 // indirect
	github.com/tilt-dev/fsnotify v1.4.8-0.20220602155310-fff9c274a375 // indirect
	github.com/tklauser/go-sysconf v0.3.12 // indirect
	github.com/tklauser/numcpus v0.6.1 // indirect
	github.com/tonistiigi/fsutil v0.0.0-20240424095704-91a3fc46842c // indirect
	github.com/tonistiigi/units v0.0.0-20180711220420-6950e57a87ea // indirect
	github.com/tonistiigi/vt100 v0.0.0-20240514184818-90bafcd6abab // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20190905194746-02993c407bfb // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/xeipuuv/gojsonschema v1.2.0 // indirect
	github.com/xo/terminfo v0.0.0-20220910002029-abceb7e1c41e // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	github.com/zeebo/bencode v1.0.0 // indirect
	github.com/zeebo/errs v1.4.0 // indirect
	github.com/zeebo/xxh3 v1.0.2 // indirect
	go.einride.tech/aip v0.68.1 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/contrib/detectors/gcp v1.35.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.60.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric v0.42.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v0.42.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v0.42.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.33.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.32.0 // indirect
	go.opentelemetry.io/otel/metric v1.35.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.35.0 // indirect
	go.opentelemetry.io/proto/otlp v1.4.0 // indirect
	go.uber.org/automaxprocs v1.6.0 // indirect
	go.uber.org/mock v0.4.0 // indirect
	golang.org/x/arch v0.11.0 // indirect
	golang.org/x/telemetry v0.0.0-20241106142447-58a1122356f5 // indirect
	golang.org/x/text v0.24.0 // indirect
	gopkg.in/cenkalti/backoff.v1 v1.1.0 // indirect
	gopkg.in/evanphx/json-patch.v4 v4.12.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	k8s.io/kube-openapi v0.0.0-20241105132330-32ad38e42d3f // indirect
	k8s.io/utils v0.0.0-20241104100929-3ea5e8cea738 // indirect
	sigs.k8s.io/json v0.0.0-20241010143419-9aa6b5e7a4b3 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.4.2 // indirect
	tags.cncf.io/container-device-interface v0.7.2 // indirect
)

// Apparently checking out NDKs at head isn't really safe.
replace golang.org/x/mobile => golang.org/x/mobile v0.0.0-20170111200746-6f0c9f6df9bb

// Version 1.2.0 has a bug: https://github.com/sergi/go-diff/issues/115
exclude github.com/sergi/go-diff v1.2.0

// Infra modules are included via gclient DEPS.
replace (
	go.chromium.org/chromiumos/config/go => ../go.chromium.org/chromiumos/config/go/src/go.chromium.org/chromiumos/config/go
	go.chromium.org/chromiumos/infra/proto/go => ../go.chromium.org/chromiumos/infra/proto/go
	go.chromium.org/luci => ../go.chromium.org/luci
)

// Replace longform path to module with proper module name.
// Update by replacing version with `latest` and calling `go mod tidy`.
replace (
	go.chromium.org/chromiumos/ctp => go.chromium.org/chromiumos/platform/dev-util/src/go.chromium.org/chromiumos/ctp v0.0.0-20241114171457-3c3ac99d1157
	go.chromium.org/chromiumos/lro => go.chromium.org/chromiumos/platform/dev-util/src/go.chromium.org/chromiumos/lro v0.0.0-20241114171457-3c3ac99d1157
	go.chromium.org/tast => go.chromium.org/tast/src/go.chromium.org/tast v0.0.0-20241114170151-b736fca5c46d
)
