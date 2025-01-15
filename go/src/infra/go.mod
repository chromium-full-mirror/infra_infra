module infra

go 1.23.0

toolchain go1.23.4

require (
	cloud.google.com/go v0.118.0
	cloud.google.com/go/appengine v1.9.3
	cloud.google.com/go/bigquery v1.65.0
	cloud.google.com/go/cloudsqlconn v1.13.2
	cloud.google.com/go/cloudtasks v1.13.3
	cloud.google.com/go/compute v1.31.1
	cloud.google.com/go/compute/metadata v0.6.0
	cloud.google.com/go/datastore v1.20.0
	cloud.google.com/go/firestore v1.18.0
	cloud.google.com/go/logging v1.13.0
	cloud.google.com/go/longrunning v0.6.4
	cloud.google.com/go/monitoring v1.22.1
	cloud.google.com/go/profiler v0.4.2
	cloud.google.com/go/pubsub v1.45.3
	cloud.google.com/go/secretmanager v1.14.3
	cloud.google.com/go/storage v1.50.0
	cloud.google.com/go/trace v1.11.3
	contrib.go.opencensus.io/exporter/stackdriver v0.13.14
	github.com/DATA-DOG/go-sqlmock v1.5.2
	github.com/Microsoft/go-winio v0.6.2
	github.com/PaesslerAG/jsonpath v0.1.1
	github.com/StackExchange/wmi v1.2.1
	github.com/VividCortex/godaemon v1.0.0
	github.com/aclements/go-moremath v0.0.0-20241023150245-c8bbc672ef66
	github.com/andygrunwald/go-gerrit v0.0.0-20210726065827-cc4e14e40b5b
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
	github.com/go-git/go-git/v5 v5.13.1
	github.com/gofrs/flock v0.12.1
	github.com/gogo/protobuf v1.3.2
	github.com/golang/glog v1.2.4
	github.com/golang/mock v1.6.0
	github.com/golang/protobuf v1.5.4
	github.com/google/cel-go v0.22.1
	github.com/google/go-cmp v0.6.0
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
	github.com/klauspost/compress v1.17.11
	github.com/klauspost/cpuid/v2 v2.2.9
	github.com/kr/pretty v0.3.1
	github.com/kylelemons/godebug v1.1.0
	github.com/linkedin/goavro/v2 v2.13.1
	github.com/maruel/subcommands v1.1.1
	github.com/mitchellh/go-homedir v1.1.0
	github.com/mitchellh/hashstructure/v2 v2.0.2
	github.com/op/go-logging v0.0.0-20160315200505-970db520ece7
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
	github.com/stretchr/testify v1.10.0
	github.com/testcontainers/testcontainers-go v0.35.0
	github.com/ulikunitz/xz v0.5.12
	github.com/waigani/diffparser v0.0.0-20190828052634-7391f219313d
	go.chromium.org/chromiumos/config/go v0.0.0-20240309015314-b8a183866804
	go.chromium.org/chromiumos/ctp v0.0.0-00010101000000-000000000000
	go.chromium.org/chromiumos/infra/proto/go v0.0.0-20240530000842-7d34be97f98c
	go.chromium.org/chromiumos/lro v0.0.0-00010101000000-000000000000
	go.chromium.org/chromiumos/test v0.0.0-00010101000000-000000000000
	go.chromium.org/luci v0.0.0-20240531181147-0c7c729b2fcf
	go.opencensus.io v0.24.0
	go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace v0.58.0
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.58.0
	go.opentelemetry.io/otel v1.33.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.33.0
	go.opentelemetry.io/otel/sdk v1.33.0
	go.opentelemetry.io/otel/trace v1.33.0
	go.skia.org/infra v0.0.0-20250114041601-4a95c0b5f67e
	go.starlark.net v0.0.0-20241226192728-8dfa5b98479f
	golang.org/x/build v0.0.0-20250107184505-240dfd072cb3
	golang.org/x/crypto v0.32.0
	golang.org/x/exp v0.0.0-20250106191152-7588d65b2ba8
	golang.org/x/mobile v0.0.0-20191031020345-0945064e013a
	golang.org/x/mod v0.22.0
	golang.org/x/net v0.34.0
	golang.org/x/oauth2 v0.25.0
	golang.org/x/perf v0.0.0-20250106172127-400946f43c82
	golang.org/x/sync v0.10.0
	golang.org/x/sys v0.29.0
	golang.org/x/term v0.28.0
	golang.org/x/time v0.9.0
	golang.org/x/tools v0.29.0
	golang.org/x/xerrors v0.0.0-20240903120638-7835f813f4da
	gonum.org/v1/gonum v0.15.1
	google.golang.org/api v0.216.0
	google.golang.org/appengine v1.6.8
	google.golang.org/appengine/v2 v2.0.6
	google.golang.org/genproto v0.0.0-20250106144421-5f5ef82da422
	google.golang.org/genproto/googleapis/api v0.0.0-20250106144421-5f5ef82da422
	google.golang.org/genproto/googleapis/bytestream v0.0.0-20250106144421-5f5ef82da422
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250106144421-5f5ef82da422
	google.golang.org/grpc v1.69.4
	google.golang.org/grpc/cmd/protoc-gen-go-grpc v1.5.1
	google.golang.org/protobuf v1.36.2
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
	cel.dev/expr v0.18.0 // indirect
	cloud.google.com/go/auth v0.13.0 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.6 // indirect
	cloud.google.com/go/errorreporting v0.3.2 // indirect
	cloud.google.com/go/iam v1.3.1 // indirect
	dario.cat/mergo v1.0.1 // indirect
	github.com/AdaLogics/go-fuzz-headers v0.0.0-20240806141605-e8a1dd7889d6 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20230124172434-306776ec8161 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp v1.25.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric v0.48.1 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace v1.24.2 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping v0.48.2 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/propagator v0.48.2 // indirect
	github.com/GoogleCloudPlatform/protoc-gen-bq-schema v0.0.0-20190119112626-026f9fcdf705 // indirect
	github.com/PaesslerAG/gval v1.0.0 // indirect
	github.com/ProtonMail/go-crypto v1.1.3 // indirect
	github.com/anmitsu/go-shlex v0.0.0-20200514113438-38f4b401e2be // indirect
	github.com/antlr4-go/antlr/v4 v4.13.0 // indirect
	github.com/apache/arrow/go/v15 v15.0.2 // indirect
	github.com/aws/aws-sdk-go v1.55.5 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cenkalti/backoff v2.2.1+incompatible // indirect
	github.com/census-instrumentation/opencensus-proto v0.4.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cilium/ebpf v0.11.0 // indirect
	github.com/cloudflare/circl v1.4.0 // indirect
	github.com/cncf/xds/go v0.0.0-20240905190251-b4127c9b8d78 // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/containerd/platforms v0.2.1 // indirect
	github.com/coreos/go-systemd/v22 v22.5.0 // indirect
	github.com/cosiner/argv v0.1.0 // indirect
	github.com/cpuguy83/dockercfg v0.3.2 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.2 // indirect
	github.com/cyphar/filepath-securejoin v0.3.6 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/derekparker/trie v0.0.0-20230829180723-39f4de51ef7d // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/docker/cli v23.0.1+incompatible // indirect
	github.com/docker/distribution v2.8.2+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.7.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/ebitengine/purego v0.8.1 // indirect
	github.com/emicklei/go-restful/v3 v3.11.0 // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/envoyproxy/go-control-plane v0.13.1 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.1.0 // indirect
	github.com/felixge/fgprof v0.9.4 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fiorix/go-web v1.0.1-0.20150221144011-5b593f1e8966 // indirect
	github.com/fxamacker/cbor/v2 v2.7.0 // indirect
	github.com/go-chi/chi/v5 v5.0.8 // indirect
	github.com/go-delve/liner v1.2.3-0.20231231155935-4726ab1d7f62 // indirect
	github.com/go-git/gcfg v1.5.1-0.20230307220236-3a3c6141e376 // indirect
	github.com/go-git/go-billy/v5 v5.6.1 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/jsonreference v0.20.4 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/goccy/go-json v0.10.2 // indirect
	github.com/godbus/dbus/v5 v5.0.6 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/gomodule/redigo v1.9.2 // indirect
	github.com/google/flatbuffers v23.5.26+incompatible // indirect
	github.com/google/gnostic-models v0.6.8 // indirect
	github.com/google/go-dap v0.12.0 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/pprof v0.0.0-20241029153458-d1b30febd7db // indirect
	github.com/google/s2a-go v0.1.8 // indirect
	github.com/google/tink/go v1.7.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.4 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.24.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/golang-lru v1.0.2 // indirect
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
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/luci/gtreap v0.0.0-20161228054646-35df89791e8f // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/magiconair/properties v1.8.7 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	github.com/mattn/go-tty v0.0.7 // indirect
	github.com/mgutz/ansi v0.0.0-20200706080929-d51e80ef957d // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/moby/patternmatcher v0.6.0 // indirect
	github.com/moby/sys/sequential v0.5.0 // indirect
	github.com/moby/sys/user v0.1.0 // indirect
	github.com/moby/sys/userns v0.1.0 // indirect
	github.com/moby/term v0.5.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/otiai10/mint v1.6.3 // indirect
	github.com/pborman/uuid v1.2.1 // indirect
	github.com/pierrec/lz4/v4 v4.1.18 // indirect
	github.com/pjbgf/sha1cd v0.3.0 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/power-devops/perfstat v0.0.0-20210106213030-5aafc221ea8c // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.55.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/prometheus/prometheus v0.54.1 // indirect
	github.com/protocolbuffers/txtpbfmt v0.0.0-20240823084532-8e6b51fa9bef // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/rogpeppe/go-internal v1.13.1 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/sergi/go-diff v1.3.2-0.20230802210424-5b0b94c5c0d3 // indirect
	github.com/shoenig/go-m1cpu v0.1.6 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/smarty/assertions v1.16.0 // indirect
	github.com/spf13/cobra v1.7.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/stoewer/go-strcase v1.3.0 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/texttheater/golang-levenshtein v1.0.1 // indirect
	github.com/tklauser/go-sysconf v0.3.12 // indirect
	github.com/tklauser/numcpus v0.6.1 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/xo/terminfo v0.0.0-20220910002029-abceb7e1c41e // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	github.com/zeebo/bencode v1.0.0 // indirect
	github.com/zeebo/xxh3 v1.0.2 // indirect
	go.chromium.org/tast v0.0.0-00010101000000-000000000000 // indirect
	go.einride.tech/aip v0.68.0 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/contrib/detectors/gcp v1.31.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.55.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.33.0 // indirect
	go.opentelemetry.io/otel/metric v1.33.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.31.0 // indirect
	go.opentelemetry.io/proto/otlp v1.4.0 // indirect
	golang.org/x/arch v0.11.0 // indirect
	golang.org/x/telemetry v0.0.0-20241106142447-58a1122356f5 // indirect
	golang.org/x/text v0.21.0 // indirect
	gopkg.in/evanphx/json-patch.v4 v4.12.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	k8s.io/kube-openapi v0.0.0-20241105132330-32ad38e42d3f // indirect
	k8s.io/utils v0.0.0-20241104100929-3ea5e8cea738 // indirect
	sigs.k8s.io/json v0.0.0-20241010143419-9aa6b5e7a4b3 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.4.2 // indirect
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
	go.chromium.org/chromiumos/test => go.chromium.org/chromiumos/platform/dev-util/src/go.chromium.org/chromiumos/test v0.0.0-20241210171927-6a404ab8a3f5
	go.chromium.org/tast => go.chromium.org/tast/src/go.chromium.org/tast v0.0.0-20241114170151-b736fca5c46d
)
