module github.com/grongor/go-bootstrap

go 1.17

require (
	github.com/TheZeroSlave/zapsentry v1.7.0
	github.com/getsentry/sentry-go v0.10.0
	github.com/go-errors/errors v1.4.0
	github.com/grongor/panicwatch v0.4.2
	github.com/mitchellh/mapstructure v1.4.1
	go.uber.org/atomic v1.9.0
	go.uber.org/multierr v1.7.0
	go.uber.org/zap v1.19.0
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
)

require (
	github.com/glycerine/rbuf v0.0.0-20190314090850-75b78581bebe // indirect
	github.com/google/go-cmp v0.5.5 // indirect
	golang.org/x/sys v0.0.0-20210816183151-1e6c022a8912 // indirect
	golang.org/x/tools v0.1.0 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)

replace github.com/TheZeroSlave/zapsentry => github.com/grongor/zapsentry v1.6.1-0.20210901143440-8ce99fc79e34
