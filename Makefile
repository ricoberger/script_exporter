BRANCH    ?= $(shell git rev-parse --abbrev-ref HEAD)
BUILDTIME ?= $(shell date '+%Y-%m-%d@%H:%M:%S')
BUILDUSER ?= $(shell id -un)
REVISION  ?= $(shell git rev-parse HEAD)
VERSION   ?= $(shell git describe --tags)

.PHONY: build
build:
	@go build -ldflags "-X github.com/prometheus/common/version.Version=${VERSION} \
		-X github.com/prometheus/common/version.Revision=${REVISION} \
		-X github.com/prometheus/common/version.Branch=${BRANCH} \
		-X github.com/prometheus/common/version.BuildUser=${BUILDUSER} \
		-X github.com/prometheus/common/version.BuildDate=${BUILDTIME}" \
		-o ./bin/script_exporter ./cmd;

.PHONY: test
test:
	# Run tests and generate coverage report. To view the coverage report in a
	# browser run "go tool cover -html=coverage.out".
	go test -covermode=atomic -coverpkg=./... -coverprofile=coverage.out -v ./...
