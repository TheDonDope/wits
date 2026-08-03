VERSION ?= $(shell git describe --tags --abbrev=0)
COMMIT_SHA = $(shell git rev-parse --short HEAD)
COMMIT_DATE = $(shell git --no-pager log -1 --pretty='format:%cd' --date='format:%Y-%m-%dT%H:%M:%S')

# The build stamps pkg/version, which is what every binary reports from.
PKG = github.com/TheDonDope/wits/pkg/version
LDFLAGS = -X $(PKG).Version=$(VERSION) -X $(PKG).CommitSHA=$(COMMIT_SHA) -X $(PKG).CommitDate=$(COMMIT_DATE)

.PHONY: run install install-wits build build-windows clean doc changelog render-tapes \
	test test-ci cover show-cover vet preflight snap release

.DEFAULT_GOAL := build

run: build
	@./bin/wits

# Developer tooling only. Nothing here is needed to build or test — CI does
# not run this target.
install:
	go install golang.org/x/tools/cmd/godoc@latest
	go install github.com/git-chglog/git-chglog/cmd/git-chglog@latest
	go install github.com/charmbracelet/freeze@latest
	go install github.com/charmbracelet/gum@latest
	go install github.com/charmbracelet/vhs@latest

# The working tree, installed where `go install …@latest` would put a release:
# $GOBIN, or $GOPATH/bin when GOBIN is unset. Stamped through the same ldflags
# as `make build`, so --version names the commit rather than confessing
# "unknown (built from source)".
install-wits:
	go install -ldflags "$(LDFLAGS)" ./cmd/wits

build:
	go build \
	  -v \
	  -ldflags "$(LDFLAGS)" \
	  -o ./bin/wits \
	  ./cmd/wits

build-windows:
	GOOS=windows \
	GOARCH=amd64 \
	go build \
	  -v \
	  -ldflags "$(LDFLAGS)" \
	  -o ./bin/wits.exe \
	  ./cmd/wits

clean:
	rm -f ./bin/wits
	rm -f ./bin/wits.exe
	rm -f coverage.html
	rm -f coverage.out
	rm -rf tmp
	rm -rf vendor

doc:
	godoc

changelog:
	git-chglog -o CHANGELOG.md

# Only the renders are cleared. `rm -rf ./assets/*` would also take
# Tracking.2022.cleaned.xlsx, which is committed and is what the importer
# integration tests read — and they skip rather than fail when it is missing,
# so it would have gone quiet rather than red.
render-tapes:
	rm -f ./assets/*.gif
	./render-vhs-tapes.sh

test:
	go test -race -v ./... -coverprofile coverage.out

# The coverage upload is the workflow's job, through the pinned codecov
# action, not a `curl | bash` here.
test-ci:
	go test -race -v ./... -coverprofile coverage.out -covermode=atomic

cover: test
	go tool cover -html coverage.out -o coverage.html

show-cover: cover
	open coverage.html

vet:
	go vet ./...

# Everything the CI and the Codacy gate will say, said here first.
preflight:
	./preflight.sh

# The camera and the tap: screens as text, the fold as JSON.
snap:
	go build -o ./bin/witsnap ./cmd/witsnap

# The guard checks where VERSION came from, not whether it is empty: it always
# has a value here, because it defaults to the previous tag — and releasing
# with the previous tag is exactly the accident to refuse.
release:
	@if [ "$(origin VERSION)" != "command line" ]; then echo "Usage: make release VERSION=vX.X.X"; exit 1; fi
	git-chglog --next-tag $(VERSION) -o CHANGELOG.md
	git add CHANGELOG.md
	git commit -m "docs: update changelog for $(VERSION)"
	git tag -a $(VERSION) -m "Release $(VERSION)"
	git push origin main --tags
