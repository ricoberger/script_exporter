WHAT := script_exporter

PROJECT     ?= script_exporter
REPO        ?= github.com/ricoberger/script_exporter
PWD         ?= $(shell pwd)
VERSION     ?= $(shell git describe --tags)
REVISION    ?= $(shell git rev-parse HEAD)
BRANCH      ?= $(shell git rev-parse --abbrev-ref HEAD)
BUILDUSER   ?= $(shell id -un)
BUILDTIME   ?= $(shell date '+%Y%m%d-%H:%M:%S')

.PHONY: build build-darwin-amd64 build-linux-amd64 build-linux-armv7 build-linux-arm64 build-windows-amd64 clean release release-major release-minor release-patch

build:
	for target in $(WHAT); do \
		go build -ldflags "-X ${REPO}/pkg/version.Version=${VERSION} \
			-X ${REPO}/pkg/version.Revision=${REVISION} \
			-X ${REPO}/pkg/version.Branch=${BRANCH} \
			-X ${REPO}/pkg/version.BuildUser=${BUILDUSER} \
			-X ${REPO}/pkg/version.BuildDate=${BUILDTIME}" \
			-o ./bin/$$target ./cmd/$$target; \
	done

build-darwin-amd64:
	for target in $(WHAT); do \
		CGO_ENABLED=0 GOARCH=amd64 GOOS=darwin go build -a -installsuffix cgo -ldflags "-X ${REPO}/pkg/version.Version=${VERSION} \
			-X ${REPO}/pkg/version.Revision=${REVISION} \
			-X ${REPO}/pkg/version.Branch=${BRANCH} \
			-X ${REPO}/pkg/version.BuildUser=${BUILDUSER} \
			-X ${REPO}/pkg/version.BuildDate=${BUILDTIME}" \
			-o ./bin/$$target-darwin-amd64 ./cmd/$$target; \
	done

build-darwin-arm64:
	for target in $(WHAT); do \
		CGO_ENABLED=0 GOARCH=arm64 GOOS=darwin go build -a -installsuffix cgo -ldflags "-X ${REPO}/pkg/version.Version=${VERSION} \
			-X ${REPO}/pkg/version.Revision=${REVISION} \
			-X ${REPO}/pkg/version.Branch=${BRANCH} \
			-X ${REPO}/pkg/version.BuildUser=${BUILDUSER} \
			-X ${REPO}/pkg/version.BuildDate=${BUILDTIME}" \
			-o ./bin/$$target-darwin-arm64 ./cmd/$$target; \
	done

build-linux-amd64:
	for target in $(WHAT); do \
		CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build -a -installsuffix cgo -ldflags "-X ${REPO}/pkg/version.Version=${VERSION} \
			-X ${REPO}/pkg/version.Revision=${REVISION} \
			-X ${REPO}/pkg/version.Branch=${BRANCH} \
			-X ${REPO}/pkg/version.BuildUser=${BUILDUSER} \
			-X ${REPO}/pkg/version.BuildDate=${BUILDTIME}" \
			-o ./bin/$$target-linux-amd64 ./cmd/$$target; \
	done

build-linux-armv7:
	for target in $(WHAT); do \
		CGO_ENABLED=0 GOARCH=arm GOARM=7 GOOS=linux go build -a -installsuffix cgo -ldflags "-X ${REPO}/pkg/version.Version=${VERSION} \
			-X ${REPO}/pkg/version.Revision=${REVISION} \
			-X ${REPO}/pkg/version.Branch=${BRANCH} \
			-X ${REPO}/pkg/version.BuildUser=${BUILDUSER} \
			-X ${REPO}/pkg/version.BuildDate=${BUILDTIME}" \
			-o ./bin/$$target-linux-armv7 ./cmd/$$target; \
	done

build-linux-arm64:
	for target in $(WHAT); do \
		CGO_ENABLED=0 GOARCH=arm64 GOOS=linux go build -a -installsuffix cgo -ldflags "-X ${REPO}/pkg/version.Version=${VERSION} \
		-X ${REPO}/pkg/version.Revision=${REVISION} \
		-X ${REPO}/pkg/version.Branch=${BRANCH} \
		-X ${REPO}/pkg/version.BuildUser=${BUILDUSER} \
		-X ${REPO}/pkg/version.BuildDate=${BUILDTIME}" \
		-o ./bin/$$target-linux-arm64 ./cmd/$$target; \
		done

build-windows-amd64:
	for target in $(WHAT); do \
		CGO_ENABLED=0 GOARCH=amd64 GOOS=windows go build -a -installsuffix cgo -ldflags "-X ${REPO}/pkg/version.Version=${VERSION} \
			-X ${REPO}/pkg/version.Revision=${REVISION} \
			-X ${REPO}/pkg/version.Branch=${BRANCH} \
			-X ${REPO}/pkg/version.BuildUser=${BUILDUSER} \
			-X ${REPO}/pkg/version.BuildDate=${BUILDTIME}" \
			-o ./bin/$$target-windows-amd64.exe ./cmd/$$target/${WHAT}_windows.go; \
	done

clean:
	rm -rf ./bin

release: clean build-darwin-amd64 build-darwin-arm64 build-linux-amd64 build-linux-armv7 build-linux-arm64 build-windows-amd64

release-major:
	$(eval MAJORVERSION=$(shell git describe --tags --abbrev=0 | sed s/v// | awk -F. '{print "v"$$1+1".0.0"}'))
	git checkout main
	git pull
	git tag -a $(MAJORVERSION) -m 'release $(MAJORVERSION)'
	git push origin --tags

release-minor:
	$(eval MINORVERSION=$(shell git describe --tags --abbrev=0 | sed s/v// | awk -F. '{print "v"$$1"."$$2+1".0"}'))
	git checkout main
	git pull
	git tag -a $(MINORVERSION) -m 'release $(MINORVERSION)'
	git push origin --tags

release-patch:
	$(eval PATCHVERSION=$(shell git describe --tags --abbrev=0 | sed s/v// | awk -F. '{print "v"$$1"."$$2"."$$3+1}'))
	git checkout main
	git pull
	git tag -a $(PATCHVERSION) -m 'release $(PATCHVERSION)'
	git push origin --tags
