module github.com/temporalio/features

go 1.20

require (
	github.com/google/uuid v1.6.0
	github.com/otiai10/copy v1.14.0
	github.com/pmezard/go-difflib v1.0.0
	github.com/temporalio/features/features v0.0.0-00010101000000-000000000000
	github.com/temporalio/features/harness/go v0.0.0-00010101000000-000000000000
	github.com/urfave/cli/v2 v2.25.7
	go.temporal.io/sdk v1.26.0
	golang.org/x/mod v0.14.0
)

require (
	github.com/cpuguy83/go-md2man/v2 v2.0.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/facebookgo/clock v0.0.0-20150410010913-600d898af40a // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/mock v1.6.0 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.4.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.19.1 // indirect
	github.com/pborman/uuid v1.2.1 // indirect
	github.com/robfig/cron v1.2.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/stretchr/testify v1.9.0 // indirect
	github.com/twmb/murmur3 v1.1.8 // indirect
	github.com/uber-go/tally/v4 v4.1.7 // indirect
	github.com/xrash/smetrics v0.0.0-20201216005158-039620a65673 // indirect
	go.temporal.io/api v1.29.1 // indirect
	go.temporal.io/sdk/contrib/tally v0.2.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.25.0 // indirect
	golang.org/x/exp v0.0.0-20231127185646-65229373498e // indirect
	golang.org/x/net v0.22.0 // indirect
	golang.org/x/sync v0.6.0 // indirect
	golang.org/x/sys v0.18.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	golang.org/x/time v0.3.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20240304212257-790db918fca8 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240304212257-790db918fca8 // indirect
	google.golang.org/grpc v1.62.1 // indirect
	google.golang.org/protobuf v1.33.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/temporalio/features/features => ./features
	github.com/temporalio/features/harness/go => ./harness/go
)
