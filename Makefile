PLATFORMS= \
	freebsd-386 \
	freebsd-amd64 \
	freebsd-arm \
	linux-386 \
	linux-amd64 \
	linux-arm \
	openbsd-386 \
	openbsd-amd64 \
	windows-386 \
	windows-amd64 \
	darwin-386 \
	darwin-amd64 \
	$(NULL)

DIST=$(shell pwd)/dist
export GOPATH=$(shell pwd)

test: src/github.com/coryb/cliby
	go test -v

debug: src/github.com/coryb/cliby
	cd src/github.com/coryb/cliby && go get -v github.com/mailgun/godebug && $(GOPATH)/bin/godebug test -instrument github.com/coryb/cliby,encoding/json

src/%:
	mkdir -p $(@D)
	test -L $@ || ln -sf ../../.. $@
	go get -v -t $*

cross-setup:
	for p in $(PLATFORMS); do \
        echo "Building for $$p"; \
		cd $(GOROOT)/src && sudo GOOS=$${p/-*/} GOARCH=$${p/*-/} bash ./make.bash --no-clean; \
   done

all:
	rm -rf $(DIST); \
	mkdir -p $(DIST); \
	cd src/github.com/Netflix-Skunkworks/go-jira/jira; \
	go get -d; \
	for p in $(PLATFORMS); do \
        echo "Building for $$p"; \
		GOOS=$${p/-*/} GOARCH=$${p/*-/} go build -v -ldflags -s -o $(DIST)/jira-$$p; \
   done

fmt:
	gofmt -s -w *.go util/*.go

install:
	export GOBIN=~/bin && ${MAKE} build

CURVER ?= $(shell git describe --abbrev=0 --tags)
NEWVER ?= $(shell echo $(CURVER) | awk -F. '{print $$1"."$$2"."$$3+1}')
TODAY  := $(shell date +%Y-%m-%d)

changes:
	@git log --pretty=format:"* %s [%cn] [%h]" --no-merges ^$(CURVER) HEAD jira | grep -v gofmt | grep -v "bump version"

update-changelog: 
	@echo "# Changelog" > CHANGELOG.md.new; \
	echo >> CHANGELOG.md.new; \
	echo "## $(NEWVER) - $(TODAY)" >> CHANGELOG.md.new; \
	echo >> CHANGELOG.md.new; \
	$(MAKE) changes | \
	perl -pe 's{\[([a-f0-9]+)\]}{[[$$1](https://github.com/coryb/cliby/commit/$$1)]}g' | \
	perl -pe 's{\#(\d+)}{[#$$1](https://github.com/coryb/cliby/issues/$$1)}g' >> CHANGELOG.md.new; \
	tail +2 CHANGELOG.md >> CHANGELOG.md.new; \
	mv CHANGELOG.md.new CHANGELOG.md; \
	git commit -m "Updated Changelog" CHANGELOG.md; \
	git tag $(NEWVER)

clean:
	rm -rf pkg dist bin src
