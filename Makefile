VERSION = $(shell git describe --tags --abbrev=0)
COMMIT_SHA = $(shell git rev-parse --short HEAD)
COMMIT_DATE = $(shell git --no-pager log -1 --pretty='format:%cd' --date='format:%Y-%m-%dT%H:%M:%S')

run: build
	@./bin/wits

install:
	go install golang.org/x/tools/cmd/godoc@latest
	go install github.com/git-chglog/git-chglog/cmd/git-chglog@latest
	go install github.com/charmbracelet/freeze@latest
	go install github.com/charmbracelet/gum@latest
	go install github.com/charmbracelet/vhs@latest

build:
	go build \
	  -v \
	  -ldflags "-X main.Version=$(VERSION) -X main.CommitSHA=$(COMMIT_SHA) -X main.CommitDate=$(COMMIT_DATE)" \
	  -o ./bin/wits \
	  ./cmd/wits

build-windows:
	GOOS=windows \
	GOARCH=amd64 \
	go build \
	  -v \
	  -ldflags "-X main.Version=$(VERSION) -X main.CommitSHA=$(COMMIT_SHA) -X main.CommitDate=$(COMMIT_DATE)" \
	  -o ./bin/wits.exe \
	  ./cmd/wits

clean:
	rm -f ./bin/wits
	rm -f ./bin/wits.exe
	rm -rf ./.wits/log
	rm -f coverage.html
	rm -f coverage.out
	rm -rf tmp
	rm -rf vendor

doc:
	godoc

changelog:
	git-chglog -o CHANGELOG.md

render-tapes:
	rm -rf ./vhs-output/*
	./render-vhs-tapes.sh 

test:
	go test -race -v ./... -coverprofile coverage.out

test-ci:
	go test -race -v ./... -coverprofile coverage.out -covermode=atomic
	bash -c "bash <(curl -s https://codecov.io/bash)"

cover: test
	go tool cover -html coverage.out -o coverage.html

show-cover: cover
	open coverage.html

vet:
	go vet ./...

release:
	@if [ -z "$(VERSION)" ]; then echo "Usage: make release VERSION=vX.X.X"; exit 1; fi
	git-chglog --next-tag $(VERSION) -o CHANGELOG.md
	git add CHANGELOG.md
	git commit -m "docs: update changelog for $(VERSION)"
	git tag -a $(VERSION) -m "Release $(VERSION)"
	git push origin main --tags
