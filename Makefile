VERSION ?= $(shell git describe --tags --abbrev=0)
COMMIT_SHA = $(shell git rev-parse --short HEAD)
COMMIT_DATE = $(shell git --no-pager log -1 --pretty='format:%cd' --date='format:%Y-%m-%dT%H:%M:%S')

# The build stamps pkg/version, which is what every binary reports from.
PKG = github.com/TheDonDope/wits/pkg/version
LDFLAGS = -X $(PKG).Version=$(VERSION) -X $(PKG).CommitSHA=$(COMMIT_SHA) -X $(PKG).CommitDate=$(COMMIT_DATE)

# The client is stamped too, with the commits since the tag and the working
# tree's state, since a page can be built from anywhere between releases.
UI_DEFINE = --define "WITS_VERSION='$(shell git describe --tags --always --dirty)'"

.PHONY: run install install-wits build build-windows clean doc changelog render-tapes \
	test test-ci cover show-cover vet preflight snap release server ui ui-test serve \
	wasm wasm-check pages appimage demo-seed

.DEFAULT_GOAL := build

run: build
	@./bin/wits

# Developer tooling only. Nothing here is needed to build or test — CI does
# not run this target. ./doctor.sh checks these and the rest the loop needs
# (ttyd, ffmpeg, shellcheck, lizard, the Chromium vhs drives), and installs
# what is missing.
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

# The browser interface: wits-server, the JSON API over the repository, and
# wits-ui, the Angular client it can serve. The client needs Node and npm;
# nothing else in the repository does.
server:
	go build -ldflags "$(LDFLAGS)" -o ./bin/wits-server ./cmd/wits-server

ui:
	cd wits-ui && { [ -d node_modules ] || npm ci; } && npx ng build $(UI_DEFINE)

ui-test:
	cd wits-ui && { [ -d node_modules ] || npm ci; } && npx ng test --watch=false

# wits for the phone: the repository, recorder and API compiled to
# WebAssembly, with the Go runtime's loader beside it. Both are build output,
# copied into the client's public folder and not committed.
wasm:
	GOOS=js GOARCH=wasm go build -trimpath -ldflags "-s -w $(LDFLAGS)" -o wits-ui/public/wits.wasm ./cmd/wits-wasm
	cp "$$(go env GOROOT)/lib/wasm/wasm_exec.js" wits-ui/public/wasm_exec.js

# The engine on MemFS under Node, synced with the CLI through a folder: what
# the phone pushes, wits pull reads, and the other way round.
wasm-check: build wasm
	./wasm-check.sh

# The phone's build, as GitHub Pages serves it under /wits/. 404.html is the
# app again, so a link into a page opens it rather than GitHub's 404.
pages: wasm
	cd wits-ui && { [ -d node_modules ] || npm ci; } && npx ng build --configuration pages $(UI_DEFINE)
	cp wits-ui/dist/wits-ui/browser/index.html wits-ui/dist/wits-ui/browser/404.html

# The repository in the working directory, in a browser at http://127.0.0.1:7341
serve: server ui
	./bin/wits-server --ui wits-ui/dist/wits-ui/browser

# The demo (assets/demo-seed.wits), made again: the workbook, the invoices'
# prices, every device, a year of rated sessions, every jar's terpenes looked
# up and the stashes cleared, bundled. The invoices are not committed; without
# assets/invoices the demo has no prices.
demo-seed:
	./demo-bundle.sh

# wits as an AppImage: the server, the command line and the interface in one
# file, dist/wits-<version>-x86_64.AppImage, checked by running it. The
# release workflow builds the same for every release and attaches it.
appimage:
	VERSION=$(shell git describe --tags --always --dirty) ./appimage.sh
	VERSION=$(shell git describe --tags --always --dirty) ./appimage.sh check

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
